#!/usr/bin/env bash
# verify-stack-hardening.sh — assert the security properties of scripts/stack.sh.
#
# The config patch is a python heredoc embedded in stack.sh, so this harness extracts that exact
# block (no copy to drift) and runs it against a temporary HOME twice: once to check a fresh
# config, once to check that a second run keeps the token it generated.
set -euo pipefail

HERE="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
STACK="${HERE}/stack.sh"
fails=0
check() { # check <description> <condition-output>
  if [[ "$2" == "ok" ]]; then printf '  PASS  %s\n' "$1"; else printf '  FAIL  %s\n' "$1"; fails=$((fails+1)); fi
}

tmp="$(mktemp -d)"
trap 'rm -rf "${tmp}"' EXIT
export HOME="${tmp}/home"
mkdir -p "${HOME}/.picooraclaw"
printf '{"channels":{"web":{"enabled":false,"host":"0.0.0.0","port":8090,"token":""}}}\n' > "${HOME}/.picooraclaw/config.json"

# Extract the python heredoc body that patches the config.
awk '/<<.PY.$/{flag=1;next} flag && /^PY$/{exit} flag' "${STACK}" > "${tmp}/patch.py"
[[ -s "${tmp}/patch.py" ]] || { echo "could not extract the config patch"; exit 1; }

run_patch() { python3 "${tmp}/patch.py" "${HOME}/.picooraclaw/config.json" "pw" "1521" "model" "8090" "$1"; }

run_patch "127.0.0.1"
token1="$(python3 -c 'import json,sys;print(json.load(open(sys.argv[1]))["channels"]["web"]["token"])' "${HOME}/.picooraclaw/config.json")"
host1="$(python3 -c 'import json,sys;print(json.load(open(sys.argv[1]))["channels"]["web"]["host"])' "${HOME}/.picooraclaw/config.json")"

[[ -n "${token1}" ]] && check "a web channel token is generated" ok || check "a web channel token is generated" fail
[[ "${#token1}" -ge 20 ]] && check "the token is long enough to be a secret (${#token1} chars)" ok || check "the token is long enough (${#token1})" fail
[[ "${host1}" == "127.0.0.1" ]] && check "the channel binds loopback by default" ok || check "loopback default (got ${host1})" fail

run_patch "127.0.0.1"
token2="$(python3 -c 'import json,sys;print(json.load(open(sys.argv[1]))["channels"]["web"]["token"])' "${HOME}/.picooraclaw/config.json")"
[[ "${token1}" == "${token2}" ]] && check "a second run keeps the generated token" ok || check "token is stable across runs" fail

# A config written by the previous version of this script carries 0.0.0.0; the next run must
# migrate it to the loopback default instead of leaving the stale value in place.
python3 - "${HOME}/.picooraclaw/config.json" <<'PY'
import json, sys
path = sys.argv[1]
cfg = json.load(open(path))
cfg["channels"]["web"]["host"] = "0.0.0.0"
json.dump(cfg, open(path, "w"))
PY
run_patch "127.0.0.1"
host3="$(python3 -c 'import json,sys;print(json.load(open(sys.argv[1]))["channels"]["web"]["host"])' "${HOME}/.picooraclaw/config.json")"
[[ "${host3}" == "127.0.0.1" ]] && check "a stale 0.0.0.0 from an older run is migrated to loopback" ok || check "stale host migrated (got ${host3})" fail

# ...and the operator's explicit choice still wins.
run_patch "0.0.0.0"
host4="$(python3 -c 'import json,sys;print(json.load(open(sys.argv[1]))["channels"]["web"]["host"])' "${HOME}/.picooraclaw/config.json")"
[[ "${host4}" == "0.0.0.0" ]] && check "WEB_CH_HOST=0.0.0.0 is honoured when asked for" ok || check "explicit host honoured (got ${host4})" fail
run_patch "127.0.0.1"

# The empty-token + all-interface combination the gateway refuses to start must not be written.
grep -q 'web.setdefault("token", "")' "${STACK}" && check "stack.sh no longer pins an empty token" fail || check "stack.sh no longer pins an empty token" ok
grep -q 'WEBUI_PASSWORD:-demo' "${STACK}" && check "stack.sh has no 'demo' password default" fail || check "stack.sh has no 'demo' password default" ok

echo
if [[ "${fails}" -gt 0 ]]; then echo "${fails} check(s) failed"; exit 1; fi
echo "stack.sh hardening checks passed"
