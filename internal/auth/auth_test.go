package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestGate_NoPassword_OpenAccess(t *testing.T) {
	g := NewGate("", "secret")
	defer g.Stop()
	req, _ := http.NewRequest("GET", "/", nil)
	if !g.Authorized(req) {
		t.Fatal("no-password mode should always authorize")
	}
}

func TestGate_LoginAndCookieRoundTrip(t *testing.T) {
	g := NewGate("pw", "secret")
	defer g.Stop()

	rr := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/login", strings.NewReader(`{"password":"pw"}`))
	if !g.HandleLogin(rr, req) {
		t.Fatal("login should succeed")
	}
	cookies := rr.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != "pwac_session" {
		t.Fatalf("unexpected cookies: %+v", cookies)
	}

	req2, _ := http.NewRequest("GET", "/", nil)
	for _, c := range cookies {
		req2.AddCookie(c)
	}
	if !g.Authorized(req2) {
		t.Fatal("cookie should authorize subsequent requests")
	}
}

func TestGate_WrongPasswordCooldown(t *testing.T) {
	g := NewGate("pw", "secret")
	defer g.Stop()
	for i := 0; i < 3; i++ {
		rr := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/login", strings.NewReader(`{"password":"nope"}`))
		req.RemoteAddr = "1.2.3.4:1"
		g.HandleLogin(rr, req)
	}
	rr := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/login", strings.NewReader(`{"password":"pw"}`))
	req.RemoteAddr = "1.2.3.4:1"
	if g.HandleLogin(rr, req) {
		t.Fatal("expected cooldown to block even correct password")
	}
}

func TestGate_ForgedCookieRejected(t *testing.T) {
	g := NewGate("pw", "secret")
	defer g.Stop()
	req, _ := http.NewRequest("GET", "/", nil)
	// Forge a cookie with a plausible expiry but wrong HMAC.
	req.AddCookie(&http.Cookie{Name: "pwac_session", Value: "9999999999.deadbeef"})
	if g.Authorized(req) {
		t.Fatal("forged cookie must not authorize")
	}
}

func TestGate_DifferentSecretsRejected(t *testing.T) {
	g1 := NewGate("pw", "secret1")
	defer g1.Stop()
	g2 := NewGate("pw", "secret2")
	defer g2.Stop()
	rr := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/login", strings.NewReader(`{"password":"pw"}`))
	g1.HandleLogin(rr, req)
	cookies := rr.Result().Cookies()

	req2, _ := http.NewRequest("GET", "/", nil)
	for _, c := range cookies {
		req2.AddCookie(c)
	}
	if g2.Authorized(req2) {
		t.Fatal("cookie signed by g1 must not validate with g2's secret")
	}
}

func TestClientIP_RespectsXForwardedFor(t *testing.T) {
	req, _ := http.NewRequest("GET", "/", nil)
	req.RemoteAddr = "10.0.0.1:55555"
	req.Header.Set("X-Forwarded-For", "5.6.7.8, 10.0.0.1")
	if got := clientIP(req); got != "5.6.7.8" {
		t.Fatalf("client IP = %q, want 5.6.7.8", got)
	}
}

func TestGate_SweepPrunesStaleEntries(t *testing.T) {
	g := NewGate("pw", "secret")
	defer g.Stop()

	g.mu.Lock()
	g.attempts["1.2.3.4"] = []time.Time{time.Now().Add(-2 * time.Minute)}
	g.mu.Unlock()

	g.sweep()

	g.mu.Lock()
	defer g.mu.Unlock()
	if _, still := g.attempts["1.2.3.4"]; still {
		t.Fatal("expected stale entry to be swept")
	}
}
