package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Gate struct {
	password string
	secret   []byte

	mu       sync.Mutex
	attempts map[string][]time.Time
	done     chan struct{}
	once     sync.Once
}

func NewGate(password, secret string) *Gate {
	g := &Gate{
		password: password,
		secret:   []byte(secret),
		attempts: make(map[string][]time.Time),
		done:     make(chan struct{}),
	}
	go g.sweepLoop()
	return g
}

// Stop halts the background cooldown sweeper. Idempotent.
func (g *Gate) Stop() {
	g.once.Do(func() { close(g.done) })
}

func (g *Gate) sweepLoop() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-g.done:
			return
		case <-ticker.C:
			g.sweep()
		}
	}
}

func (g *Gate) sweep() {
	cutoff := time.Now().Add(-30 * time.Second)
	g.mu.Lock()
	defer g.mu.Unlock()
	for ip, ts := range g.attempts {
		kept := ts[:0]
		for _, t := range ts {
			if t.After(cutoff) {
				kept = append(kept, t)
			}
		}
		if len(kept) == 0 {
			delete(g.attempts, ip)
		} else {
			g.attempts[ip] = kept
		}
	}
}

// Authorized returns true iff the request is permitted.
// Open access when no password is set.
func (g *Gate) Authorized(r *http.Request) bool {
	if g.password == "" {
		return true
	}
	c, err := r.Cookie("pwac_session")
	if err != nil {
		return false
	}
	return g.verify(c.Value)
}

// HandleLogin processes POST /api/login; returns true on success (cookie set).
func (g *Gate) HandleLogin(w http.ResponseWriter, r *http.Request) bool {
	if g.password == "" {
		w.WriteHeader(http.StatusNoContent)
		return true
	}
	ip := clientIP(r)
	if g.cooldown(ip) {
		http.Error(w, "too many attempts, try again later", http.StatusTooManyRequests)
		return false
	}
	var body struct {
		Password string `json:"password"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if body.Password != g.password {
		g.recordAttempt(ip)
		http.Error(w, "invalid password", http.StatusUnauthorized)
		return false
	}
	g.clearAttempts(ip)

	expires := time.Now().Add(30 * 24 * time.Hour)
	val := g.sign(strconv.FormatInt(expires.Unix(), 10))
	http.SetCookie(w, &http.Cookie{
		Name: "pwac_session", Value: val, Path: "/",
		Expires: expires, HttpOnly: true, SameSite: http.SameSiteLaxMode,
	})
	w.WriteHeader(http.StatusNoContent)
	return true
}

func (g *Gate) sign(payload string) string {
	mac := hmac.New(sha256.New, g.secret)
	mac.Write([]byte(payload))
	return payload + "." + hex.EncodeToString(mac.Sum(nil))
}

func (g *Gate) verify(v string) bool {
	parts := strings.SplitN(v, ".", 2)
	if len(parts) != 2 {
		return false
	}
	mac := hmac.New(sha256.New, g.secret)
	mac.Write([]byte(parts[0]))
	want := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(want), []byte(parts[1])) {
		return false
	}
	exp, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return false
	}
	return time.Now().Unix() < exp
}

func (g *Gate) cooldown(ip string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	cutoff := time.Now().Add(-30 * time.Second)
	kept := make([]time.Time, 0)
	for _, t := range g.attempts[ip] {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	g.attempts[ip] = kept
	return len(kept) >= 3
}

func (g *Gate) recordAttempt(ip string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.attempts[ip] = append(g.attempts[ip], time.Now())
}

func (g *Gate) clearAttempts(ip string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.attempts, ip)
}

func clientIP(r *http.Request) string {
	if xf := r.Header.Get("X-Forwarded-For"); xf != "" {
		return strings.TrimSpace(strings.Split(xf, ",")[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
