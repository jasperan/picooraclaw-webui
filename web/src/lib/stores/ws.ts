import { writable } from 'svelte/store';

export type AgentEvent = {
	type: 'message_start' | 'message_end' | 'tool_call_start' | 'tool_call_end' | 'error' | 'agent_tick';
	session_id?: string;
	message_id?: string;
	client_id?: string;
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
	| { type: 'send'; session_id: string; text: string; client_id?: string };

export const wsConnected = writable(false);

let ws: WebSocket | null = null;
let eventHandlers: Array<(e: AgentEvent) => void> = [];
let reconnectDelay = 250;
let reconnectTimer: ReturnType<typeof setTimeout> | null = null;

// Queue of frames to flush when the socket opens. Svelte stores fire their
// subscribers synchronously on assignment, so subscribe() can be called before
// the WebSocket is OPEN — we hold those frames here instead of dropping them.
let pending: OutgoingFrame[] = [];

// The last subscribe frame. On reconnect, we replay it so the new socket
// immediately re-subscribes to the active session.
let lastSubscribe: { type: 'subscribe'; session_id: string; from?: string } | null = null;

export function connect() {
	if (ws && (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING)) {
		return;
	}
	if (reconnectTimer) {
		clearTimeout(reconnectTimer);
		reconnectTimer = null;
	}
	// Drop any frames left over from a failed previous attempt so we don't
	// replay a stale subscribe (and double-register in the hub) on reconnect.
	pending = [];
	const url = location.origin.replace(/^http/, 'ws') + '/ws';
	const socket = new WebSocket(url);
	ws = socket;
	socket.onopen = () => {
		if (ws !== socket) return;
		wsConnected.set(true);
		reconnectDelay = 250;
		if (lastSubscribe) {
			socket.send(JSON.stringify(lastSubscribe));
		}
		for (const f of pending) {
			socket.send(JSON.stringify(f));
		}
		pending = [];
	};
	socket.onmessage = (ev) => {
		try {
			const frame = JSON.parse(ev.data) as unknown;
			if (isEventFrame(frame)) {
				eventHandlers.forEach((h) => h(frame.payload));
			}
		} catch {
			// Malformed frame — ignore.
		}
	};
	socket.onclose = () => {
		if (ws !== socket) return;
		ws = null;
		wsConnected.set(false);
		reconnectTimer = setTimeout(connect, Math.min((reconnectDelay *= 2), 8000));
	};
	socket.onerror = () => socket.close();
}

export function subscribe(sessionId: string, from?: string) {
	const frame: OutgoingFrame = { type: 'subscribe', session_id: sessionId, from };
	lastSubscribe = { type: 'subscribe', session_id: sessionId, from };
	send(frame);
}

export function sendMessage(sessionId: string, text: string, clientId?: string) {
	send({ type: 'send', session_id: sessionId, text, client_id: clientId });
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

function isEventFrame(value: unknown): value is { type: 'event'; payload: AgentEvent } {
	if (!value || typeof value !== 'object') return false;
	const frame = value as { type?: unknown; payload?: unknown };
	if (frame.type !== 'event' || !frame.payload || typeof frame.payload !== 'object') {
		return false;
	}
	const event = frame.payload as { type?: unknown };
	return (
		event.type === 'message_start' ||
		event.type === 'message_end' ||
		event.type === 'tool_call_start' ||
		event.type === 'tool_call_end' ||
		event.type === 'error' ||
		event.type === 'agent_tick'
	);
}
