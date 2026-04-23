package ws

import (
	"sync"
	"testing"
	"time"
)

type fakeConn struct {
	id     string
	mu     sync.Mutex
	frames []Frame
}

func (c *fakeConn) Send(f Frame) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.frames = append(c.frames, f)
	return nil
}

func (c *fakeConn) Close() {}

func (c *fakeConn) frameCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.frames)
}

func TestHub_SubscribeAndBroadcast(t *testing.T) {
	h := NewHub()
	defer h.Close()

	cA := &fakeConn{id: "a"}
	cB := &fakeConn{id: "b"}
	cOther := &fakeConn{id: "c"}
	h.Register(cA, "s1")
	h.Register(cB, "s1")
	h.Register(cOther, "s2")

	h.Broadcast("s1", Frame{Type: "event", Payload: []byte(`{"type":"x"}`)})

	waitFrame(t, cA)
	waitFrame(t, cB)
	if cOther.frameCount() != 0 {
		t.Fatalf("cOther should not receive")
	}
}

func TestHub_Unregister(t *testing.T) {
	h := NewHub()
	defer h.Close()
	c := &fakeConn{id: "a"}
	h.Register(c, "s1")
	h.Unregister(c)
	h.Broadcast("s1", Frame{Type: "event", Payload: []byte(`{"t":"x"}`)})
	if c.frameCount() != 0 {
		t.Fatalf("unregistered conn should not receive, got %d", c.frameCount())
	}
}

func TestHub_ConcurrentRegisterAndBroadcast(t *testing.T) {
	h := NewHub()
	defer h.Close()

	var wg sync.WaitGroup
	const N = 50
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c := &fakeConn{id: "c"}
			h.Register(c, "s1")
			h.Broadcast("s1", Frame{Type: "event", Payload: []byte(`{"t":"x"}`)})
			h.Unregister(c)
		}()
	}
	wg.Wait()
}

func waitFrame(t *testing.T, c *fakeConn) {
	t.Helper()
	deadline := time.Now().Add(200 * time.Millisecond)
	for time.Now().Before(deadline) {
		if c.frameCount() > 0 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("conn %s never received a frame", c.id)
}
