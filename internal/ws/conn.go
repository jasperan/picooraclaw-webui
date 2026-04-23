package ws

import (
	"context"
	"encoding/json"
	"time"

	"nhooyr.io/websocket"
)

type wsConn struct {
	c   *websocket.Conn
	ctx context.Context
}

func NewWSConn(ctx context.Context, c *websocket.Conn) Conn {
	return &wsConn{c: c, ctx: ctx}
}

func (w *wsConn) Send(f Frame) error {
	ctx, cancel := context.WithTimeout(w.ctx, 5*time.Second)
	defer cancel()
	buf, err := json.Marshal(f)
	if err != nil {
		return err
	}
	return w.c.Write(ctx, websocket.MessageText, buf)
}

func (w *wsConn) Close() {
	_ = w.c.Close(websocket.StatusNormalClosure, "")
}

type IncomingFrame struct {
	Type      string `json:"type"`
	SessionID string `json:"session_id"`
	Text      string `json:"text"`
	From      string `json:"from"`
}

func ReadFrame(ctx context.Context, c *websocket.Conn) (IncomingFrame, error) {
	_, buf, err := c.Read(ctx)
	if err != nil {
		return IncomingFrame{}, err
	}
	var f IncomingFrame
	if err := json.Unmarshal(buf, &f); err != nil {
		return IncomingFrame{}, err
	}
	return f, nil
}
