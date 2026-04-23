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

export function connect() {
	const url = location.origin.replace(/^http/, 'ws') + '/ws';
	ws = new WebSocket(url);
	ws.onopen = () => {
		wsConnected.set(true);
		reconnectDelay = 250;
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
	send({ type: 'subscribe', session_id: sessionId, from });
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
	if (!ws || ws.readyState !== WebSocket.OPEN) return;
	ws.send(JSON.stringify(frame));
}
