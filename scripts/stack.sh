#!/usr/bin/env bash
# stack.sh — boot the full picooraclaw stack (Oracle + OCI proxy + gateway + webui).
#
# Usage:
#   scripts/stack.sh up         # start everything (default), idempotent
#   scripts/stack.sh down       # stop webui + gateway + proxy (Oracle stays unless ORACLE_KEEP=0)
#   scripts/stack.sh status     # health-probe each service
#   scripts/stack.sh logs       # tail all service logs
#
# Env overrides (all optional):
#   PICOORACLAW_DIR     path to picooraclaw checkout       (default: ../picooraclaw next to this repo)
#   ORACLE_PWD          system + picooraclaw password      (default: PicoOraclaw123, matches setup-oracle.sh)
#   ORACLE_CONTAINER    container name to manage           (default: oracle-free, matches setup-oracle.sh)
#   ORACLE_KEEP         keep Oracle running on `down`      (default: 1)
#   PROXY_PORT          OCI GenAI proxy port               (default: 9999)
#   WEB_CH_PORT         picooraclaw web channel port       (default: 8090)
#   GATEWAY_PORT        picooraclaw gateway health port    (default: 18790)
#   WEBUI_PORT          host port for the browser UI       (default: 3000)
#   WEBUI_PASSWORD      single-field login password        (default: demo)
#   SKIP_ORACLE=1       skip Oracle entirely (file-based fallback)
#   SKIP_PROXY=1        skip the OCI GenAI proxy
#
# Notes:
#   * Container name + port (1521 → host 1521) are owned by picooraclaw/scripts/setup-oracle.sh.
#     If you already have another Oracle container (e.g. pythia-oracle on host:1523), this script
#     creates a separate one named ${ORACLE_CONTAINER} on host:1521 — they don't interfere.
#   * If `docker` requires sudo on this host, the script auto-detects and uses `sudo -n docker`.

set -euo pipefail

HERE="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
WEBUI_DIR="$(cd -- "${HERE}/.." && pwd)"
PICOORACLAW_DIR="${PICOORACLAW_DIR:-$(cd -- "${WEBUI_DIR}/../picooraclaw" 2>/dev/null && pwd || true)}"
RUN_DIR="${WEBUI_DIR}/.run"
LOG_DIR="${RUN_DIR}/logs"
mkdir -p "${RUN_DIR}" "${LOG_DIR}"

ORACLE_CONTAINER="${ORACLE_CONTAINER:-oracle-free}"
ORACLE_PWD="${ORACLE_PWD:-PicoOraclaw123}"
PROXY_PORT="${PROXY_PORT:-9999}"
WEB_CH_PORT="${WEB_CH_PORT:-8090}"
GATEWAY_PORT="${GATEWAY_PORT:-18790}"
WEBUI_PORT="${WEBUI_PORT:-3000}"
WEBUI_PASSWORD="${WEBUI_PASSWORD:-demo}"

c_blue=$'\033[36m'; c_yellow=$'\033[33m'; c_red=$'\033[31m'; c_green=$'\033[32m'; c_reset=$'\033[0m'
log()  { printf '%s[stack]%s %s\n' "${c_blue}"   "${c_reset}" "$*"; }
ok()   { printf '%s[stack]%s %s\n' "${c_green}"  "${c_reset}" "$*"; }
warn() { printf '%s[stack]%s %s\n' "${c_yellow}" "${c_reset}" "$*" >&2; }
err()  { printf '%s[stack]%s %s\n' "${c_red}"    "${c_reset}" "$*" >&2; }

# Auto-pick docker invocation: direct if the daemon socket is reachable, else `sudo -n docker`.
DOCKER=""
detect_docker() {
  if [[ -n "${DOCKER}" ]]; then return 0; fi
  command -v docker >/dev/null 2>&1 || { err "docker not found in PATH"; exit 1; }
  if docker info >/dev/null 2>&1; then
    DOCKER="docker"
  elif sudo -n docker info >/dev/null 2>&1; then
    DOCKER="sudo -n docker"
    log "docker: using passwordless sudo"
  else
    err "docker daemon unreachable (and sudo -n docker also failed). Add this user to the 'docker' group or fix sudoers."
    exit 1
  fi
}
dk() { ${DOCKER} "$@"; }

# Reliable port-listening probe using bash /dev/tcp (no ss/awk parsing pitfalls).
port_listening() {
  local p="$1"
  (exec 3<>"/dev/tcp/127.0.0.1/${p}") >/dev/null 2>&1 && { exec 3<&-; exec 3>&-; return 0; } || return 1
}
http_code() { curl -s -m 3 -o /dev/null -w '%{http_code}' "$1" 2>/dev/null || echo 000; }

pidfile() { echo "${RUN_DIR}/$1.pid"; }
read_pid() { cat "$(pidfile "$1")" 2>/dev/null || true; }
alive() { local p="${1:-}"; [[ -n "${p}" ]] && kill -0 "${p}" 2>/dev/null; }

require_picooraclaw_dir() {
  if [[ -z "${PICOORACLAW_DIR:-}" || ! -d "${PICOORACLAW_DIR}" ]]; then
    err "picooraclaw checkout not found. Set PICOORACLAW_DIR or place picooraclaw alongside this repo."
    exit 1
  fi
}

# ──────────────────────────────────────────────────────────── Oracle ──
# We delegate container creation + schema bootstrap entirely to
# picooraclaw's setup-oracle.sh on first run. Subsequent runs only
# need to ensure the container is started.

ensure_oracle_container() {
  detect_docker
  if dk ps --format '{{.Names}}' | grep -qx "${ORACLE_CONTAINER}"; then
    ok "oracle: container ${ORACLE_CONTAINER} already running"
    return 0
  fi
  if dk ps -a --format '{{.Names}}' | grep -qx "${ORACLE_CONTAINER}"; then
    log "oracle: starting existing container ${ORACLE_CONTAINER}"
    dk start "${ORACLE_CONTAINER}" >/dev/null
    log "oracle: waiting for FREEPDB1 (up to 4 min)"
    local i
    for i in $(seq 1 48); do
      if dk exec "${ORACLE_CONTAINER}" sh -c \
           "echo 'select 1 from dual;' | sqlplus -s system/${ORACLE_PWD}@//localhost:1521/FREEPDB1" 2>/dev/null \
           | grep -qE '^[[:space:]]*1[[:space:]]*$'; then
        ok "oracle: ready"
        return 0
      fi
      sleep 5
    done
    err "oracle: container started but FREEPDB1 not responding. Check: ${DOCKER} logs ${ORACLE_CONTAINER}"
    exit 1
  fi
  return 2  # container does not exist; let setup-oracle.sh create it
}

bootstrap_oracle() {
  if [[ "${SKIP_ORACLE:-0}" == "1" ]]; then
    warn "oracle: SKIP_ORACLE=1 — using file-based fallback"
    return 0
  fi
  require_picooraclaw_dir
  detect_docker

  set +e
  ensure_oracle_container
  local rc=$?
  set -e

  local marker="${RUN_DIR}/oracle-bootstrapped"
  if [[ ${rc} -eq 0 && -f "${marker}" ]]; then
    return 0
  fi

  local script="${PICOORACLAW_DIR}/scripts/setup-oracle.sh"
  if [[ ! -x "${script}" ]]; then
    if [[ ${rc} -eq 2 ]]; then
      err "oracle: ${script} missing — cannot bootstrap a new container."
      exit 1
    fi
    warn "oracle: ${script} not executable, container is up but schema unverified"
    return 0
  fi

  log "oracle: running setup-oracle.sh (creates container if missing, bootstraps user/schema/ONNX, ~2 min)"
  if [[ "${DOCKER}" != "docker" ]]; then
    # setup-oracle.sh calls `docker` directly — make it work under sudo by exporting an alias-like wrapper.
    # Easiest: re-invoke the script with sudo so its `docker` calls inherit root.
    ( cd "${PICOORACLAW_DIR}" && sudo -n -E env "PATH=$PATH" "HOME=$HOME" bash "${script}" "${ORACLE_PWD}" )
  else
    ( cd "${PICOORACLAW_DIR}" && "${script}" "${ORACLE_PWD}" )
  fi
  touch "${marker}"
  ok "oracle: bootstrapped"
}

# ──────────────────────────────────────────────────────── OCI proxy ──
start_proxy() {
  if [[ "${SKIP_PROXY:-0}" == "1" ]]; then
    warn "oci-proxy: SKIP_PROXY=1 — skipping (configure another LLM in ~/.picooraclaw/config.json)"
    return 0
  fi
  if port_listening "${PROXY_PORT}"; then
    ok "oci-proxy: already on :${PROXY_PORT}"
    return 0
  fi
  require_picooraclaw_dir
  local proxy_py="${PICOORACLAW_DIR}/oci-genai/proxy.py"
  [[ -f "${proxy_py}" ]] || { err "oci-proxy: ${proxy_py} not found"; exit 1; }
  [[ -f "${HOME}/.oci/config" ]] || { err "oci-proxy: ~/.oci/config not found (needed for OCI GenAI auth)"; exit 1; }

  export OCI_COMPARTMENT_ID="${OCI_COMPARTMENT_ID:-$(awk '/^tenancy=/{sub(/^tenancy=/,"");print;exit}' "${HOME}/.oci/config")}"
  log "oci-proxy: starting on :${PROXY_PORT}"
  ( cd "${PICOORACLAW_DIR}/oci-genai" && \
    nohup python3 proxy.py >"${LOG_DIR}/oci-proxy.log" 2>&1 </dev/null & echo $! >"$(pidfile oci-proxy)" )
  local i
  for i in $(seq 1 20); do
    if [[ "$(http_code "http://localhost:${PROXY_PORT}/v1/models")" == "200" ]]; then
      ok "oci-proxy: ready"
      return 0
    fi
    sleep 1
  done
  err "oci-proxy: failed to start; see ${LOG_DIR}/oci-proxy.log"
  exit 1
}

# ────────────────────────────────────────────────── picooraclaw gateway ──
start_gateway() {
  if port_listening "${WEB_CH_PORT}" && port_listening "${GATEWAY_PORT}"; then
    ok "gateway: already running (:${WEB_CH_PORT} web, :${GATEWAY_PORT} health)"
    return 0
  fi
  require_picooraclaw_dir
  local bin="${PICOORACLAW_DIR}/build/picooraclaw"
  [[ -x "${bin}" ]] || { err "gateway: ${bin} not built. Run: (cd ${PICOORACLAW_DIR} && make build)"; exit 1; }

  log "gateway: starting picooraclaw gateway --enable-web"
  local env_prefix=()
  if [[ "${SKIP_ORACLE:-0}" != "1" ]]; then
    env_prefix=(env "PICO_ORACLE_ENABLED=true" "PICO_ORACLE_PASSWORD=${ORACLE_PWD}")
  fi
  ( cd "${PICOORACLAW_DIR}" && \
    nohup "${env_prefix[@]+"${env_prefix[@]}"}" ./build/picooraclaw gateway --enable-web \
      >"${LOG_DIR}/gateway.log" 2>&1 </dev/null & echo $! >"$(pidfile gateway)" )

  local i
  for i in $(seq 1 30); do
    if [[ "$(http_code "http://localhost:${GATEWAY_PORT}/health")" == "200" ]] && \
       [[ "$(http_code "http://localhost:${WEB_CH_PORT}/v1/sessions")" == "200" ]]; then
      ok "gateway: ready"
      return 0
    fi
    sleep 1
  done
  err "gateway: failed to come up; see ${LOG_DIR}/gateway.log"
  exit 1
}

# ──────────────────────────────────────────────────────── browser UI ──
start_webui() {
  if port_listening "${WEBUI_PORT}"; then
    if pgrep -af 'picooraclaw-webui ' 2>/dev/null | grep -q -- "--picooraclaw-url http://127.0.0.1:${WEB_CH_PORT}"; then
      ok "webui: already running on :${WEBUI_PORT} → :${WEB_CH_PORT}"
      return 0
    fi
    warn "webui: :${WEBUI_PORT} held by a process not pointing at :${WEB_CH_PORT}, replacing"
    pkill -f 'picooraclaw-webui ' 2>/dev/null || true
    sleep 1
  fi
  local bin="${WEBUI_DIR}/bin/picooraclaw-webui"
  if [[ ! -x "${bin}" ]]; then
    log "webui: building binary (make build)"
    ( cd "${WEBUI_DIR}" && make build )
  fi

  log "webui: starting on :${WEBUI_PORT} → upstream :${WEB_CH_PORT}"
  ( cd "${WEBUI_DIR}" && \
    nohup ./bin/picooraclaw-webui \
      --picooraclaw-url "http://127.0.0.1:${WEB_CH_PORT}" \
      --listen ":${WEBUI_PORT}" \
      --password "${WEBUI_PASSWORD}" \
      >"${LOG_DIR}/webui.log" 2>&1 </dev/null & echo $! >"$(pidfile webui)" )

  local i
  for i in $(seq 1 20); do
    if [[ "$(http_code "http://localhost:${WEBUI_PORT}/")" == "200" ]]; then
      ok "webui: ready"
      return 0
    fi
    sleep 1
  done
  err "webui: failed to start; see ${LOG_DIR}/webui.log"
  exit 1
}

# ──────────────────────────────────────────────────────────── commands ──
cmd_up() {
  bootstrap_oracle
  start_proxy
  start_gateway
  start_webui
  cat <<EOF

${c_green}✓ Stack is up.${c_reset}
  Oracle    container ${ORACLE_CONTAINER}        (system / ${ORACLE_PWD})
  OCI Proxy http://localhost:${PROXY_PORT}/v1
  Gateway   http://localhost:${GATEWAY_PORT}/health
  Web ch    http://localhost:${WEB_CH_PORT}/v1/sessions
  Web UI    http://localhost:${WEBUI_PORT}              (password: ${WEBUI_PASSWORD})

  Logs:    ${LOG_DIR}/
  Status:  ${0##*/} status
  Stop:    ${0##*/} down
EOF
}

cmd_down() {
  for svc in webui gateway oci-proxy; do
    local p; p="$(read_pid "${svc}")"
    if alive "${p}"; then
      log "stopping ${svc} (pid ${p})"
      kill "${p}" 2>/dev/null || true
    fi
    rm -f "$(pidfile "${svc}")"
  done
  pkill -f 'picooraclaw-webui ' 2>/dev/null || true
  pkill -f 'picooraclaw gateway' 2>/dev/null || true
  pkill -f 'oci-genai/proxy.py' 2>/dev/null || true

  if [[ "${ORACLE_KEEP:-1}" == "0" ]]; then
    detect_docker || true
    log "stopping oracle container ${ORACLE_CONTAINER}"
    dk stop "${ORACLE_CONTAINER}" >/dev/null 2>&1 || true
  else
    log "leaving oracle running (set ORACLE_KEEP=0 to stop it)"
  fi
  ok "down"
}

cmd_status() {
  set +e
  local oracle_status="down (or unreachable)"
  if command -v docker >/dev/null 2>&1; then
    detect_docker 2>/dev/null
    if [[ -n "${DOCKER}" ]]; then
      local s
      s="$(dk ps --filter "name=^${ORACLE_CONTAINER}$" --format '{{.Status}}' 2>/dev/null | head -1)"
      [[ -n "${s}" ]] && oracle_status="${s}"
    fi
  fi
  printf "%-12s %s\n" "Oracle"    "${oracle_status}"
  printf "%-12s http %s on :%s\n" "OCI Proxy" "$(http_code "http://localhost:${PROXY_PORT}/v1/models")"     "${PROXY_PORT}"
  printf "%-12s http %s on :%s\n" "Gateway"   "$(http_code "http://localhost:${GATEWAY_PORT}/health")"      "${GATEWAY_PORT}"
  printf "%-12s http %s on :%s\n" "Web ch"    "$(http_code "http://localhost:${WEB_CH_PORT}/v1/sessions")"  "${WEB_CH_PORT}"
  printf "%-12s http %s on :%s\n" "Web UI"    "$(http_code "http://localhost:${WEBUI_PORT}/")"              "${WEBUI_PORT}"
  set -e
}

cmd_logs() {
  shopt -s nullglob
  local files=("${LOG_DIR}"/*.log)
  if (( ${#files[@]} == 0 )); then
    err "no logs in ${LOG_DIR}"
    exit 1
  fi
  tail -n 50 -F "${files[@]}"
}

case "${1:-up}" in
  up)     cmd_up ;;
  down)   cmd_down ;;
  status) cmd_status ;;
  logs)   cmd_logs ;;
  -h|--help|help)
    grep -E '^# ' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
    ;;
  *)
    err "unknown command: $1"
    err "usage: ${0##*/} [up|down|status|logs]"
    exit 2
    ;;
esac
