import { writable, get } from 'svelte/store';
import type { AgentEvent } from './ws';

export type ToolCall = {
	id: string;
	tool: string;
	args?: Record<string, unknown>;
	result?: string;
	ok?: boolean;
	running: boolean;
};

export type Message = {
	id: string;
	role: 'user' | 'assistant';
	text: string;
	streaming: boolean;
	error?: string;
	toolCalls: ToolCall[];
	ts: number;
};

export const messagesBySession = writable<Record<string, Message[]>>({});

function update(sessionId: string, fn: (msgs: Message[]) => Message[]) {
	messagesBySession.update((all) => {
		const next = { ...all };
		next[sessionId] = fn(all[sessionId] ?? []);
		return next;
	});
}

function findAssistant(msgs: Message[], id?: string): Message | undefined {
	if (id) return msgs.find((m) => m.role === 'assistant' && m.id === id);
	// Fallback: latest streaming assistant.
	for (let i = msgs.length - 1; i >= 0; i--) {
		if (msgs[i].role === 'assistant' && msgs[i].streaming) return msgs[i];
	}
	return undefined;
}

export function appendUserMessage(sessionId: string, text: string) {
	update(sessionId, (msgs) => [
		...msgs,
		{
			id: `u-${Date.now()}`,
			role: 'user',
			text,
			streaming: false,
			toolCalls: [],
			ts: Date.now()
		}
	]);
}

export function applyEvent(sessionId: string, e: AgentEvent) {
	switch (e.type) {
		case 'message_start': {
			const id = e.message_id ?? `a-${Date.now()}`;
			update(sessionId, (msgs) => [
				...msgs,
				{
					id,
					role: 'assistant',
					text: '',
					streaming: true,
					toolCalls: [],
					ts: Date.now()
				}
			]);
			break;
		}
		case 'message_end': {
			update(sessionId, (msgs) => {
				const m = findAssistant(msgs, e.message_id);
				if (!m) return msgs;
				if (e.text !== undefined) m.text = e.text;
				m.streaming = false;
				return [...msgs];
			});
			break;
		}
		case 'tool_call_start': {
			update(sessionId, (msgs) => {
				const m = findAssistant(msgs, e.message_id);
				if (!m) return msgs;
				m.toolCalls.push({
					id: e.id ?? `t-${Date.now()}`,
					tool: e.tool ?? '',
					args: e.args,
					running: true
				});
				return [...msgs];
			});
			break;
		}
		case 'tool_call_end': {
			update(sessionId, (msgs) => {
				const m = findAssistant(msgs, e.message_id);
				if (!m) return msgs;
				const tc = m.toolCalls.find((t) => t.id === e.id);
				if (tc) {
					tc.result = e.result;
					tc.ok = e.ok;
					tc.running = false;
				}
				return [...msgs];
			});
			break;
		}
		case 'error': {
			update(sessionId, (msgs) => {
				const m = findAssistant(msgs, e.message_id);
				if (m) {
					m.error = e.error;
					m.streaming = false;
					return [...msgs];
				}
				// No pending assistant — create a standalone error message.
				return [
					...msgs,
					{
						id: `err-${Date.now()}`,
						role: 'assistant',
						text: '',
						streaming: false,
						error: e.error,
						toolCalls: [],
						ts: Date.now()
					}
				];
			});
			break;
		}
		case 'agent_tick':
			// Heartbeat — no-op on the message list.
			break;
	}
}

export function _peek(sessionId: string) {
	return get(messagesBySession)[sessionId] ?? [];
}
