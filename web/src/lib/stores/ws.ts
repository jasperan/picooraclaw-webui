import { writable } from 'svelte/store';

export type AgentEvent = {
	type: 'message_start' | 'message_end' | 'tool_call_start' | 'tool_call_end' | 'error' | 'agent_tick';
	session_id?: string;
	message_id?: string;
	id?: string;
	tool?: string;
	args?: Record<string, unknown>;
	result?: string;
	ok?: boolean;
	text?: string;
	error?: string;
	note?: string;
	ts?: string;
};

type OutgoingFrame =
	| { type: 'subscribe'; session_id: string; from?: string }
	| { type: 'send'; session_id: string; text: string };

export const wsConnected = writable(false);

let ws: WebSocket | null = null;
let eventHandlers: Array<(e: AgentEvent) => void> = [];
let reconnectDelay = 250;

// Queue of frames to flush when the socket opens. Svelte stores fire their
// subscribers synchronously on assignment, so subscribe() can be called before
// the WebSocket is OPEN — we hold those frames here instead of dropping them.
let pending: OutgoingFrame[] = [];

// The last subscribe frame. On reconnect, we replay it so the new socket
// immediately re-subscribes to the active session.
let lastSubscribe: { type: 'subscribe'; session_id: string; from?: string } | null = null;

export function connect() {
	// Drop any frames left over from a failed previous attempt so we don't
	// replay a stale subscribe (and double-register in the hub) on reconnect.
	pending = [];
	const url = location.origin.replace(/^http/, 'ws') + '/ws';
	ws = new WebSocket(url);
	ws.onopen = () => {
		wsConnected.set(true);
		reconnectDelay = 250;
		if (lastSubscribe) {
			ws!.send(JSON.stringify(lastSubscribe));
		}
		for (const f of pending) {
			ws!.send(JSON.stringify(f));
		}
		pending = [];
	};
	ws.onmessage = (ev) => {
		try {
			const frame = JSON.parse(ev.data);
			if (frame.type === 'event' && frame.payload) {
				// Go bridge emits Frame.Payload as json.RawMessage, which surfaces as a
				// directly-parsed JSON object in the browser.
				const event = frame.payload as AgentEvent;
				eventHandlers.forEach((h) => h(event));
			}
		} catch {
			// Malformed frame — ignore.
		}
	};
	ws.onclose = () => {
		wsConnected.set(false);
		setTimeout(connect, Math.min((reconnectDelay *= 2), 8000));
	};
	ws.onerror = () => ws?.close();
}

export function subscribe(sessionId: string, from?: string) {
	const frame: OutgoingFrame = { type: 'subscribe', session_id: sessionId, from };
	lastSubscribe = { type: 'subscribe', session_id: sessionId, from };
	send(frame);
}

export function sendMessage(sessionId: string, text: string) {
	send({ type: 'send', session_id: sessionId, text });
}

export function onEvent(h: (e: AgentEvent) => void) {
	eventHandlers.push(h);
	return () => {
		eventHandlers = eventHandlers.filter((x) => x !== h);
	};
}

function send(frame: OutgoingFrame) {
	if (ws && ws.readyState === WebSocket.OPEN) {
		ws.send(JSON.stringify(frame));
		return;
	}
	pending.push(frame);
}
