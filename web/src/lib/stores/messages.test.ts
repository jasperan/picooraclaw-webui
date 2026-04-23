import { get } from 'svelte/store';
import { describe, it, expect, beforeEach } from 'vitest';
import { messagesBySession, appendUserMessage, applyEvent } from './messages';

describe('messages store', () => {
	beforeEach(() => {
		messagesBySession.set({});
	});

	it('folds user message, message_start, tool_call_start/end, message_end', () => {
		appendUserMessage('s1', 'hello');
		applyEvent('s1', { type: 'message_start', session_id: 's1', message_id: 'm1' });
		applyEvent('s1', {
			type: 'tool_call_start',
			session_id: 's1',
			message_id: 'm1',
			id: 'tc1',
			tool: 'remember',
			args: { text: 'hi' }
		});
		applyEvent('s1', {
			type: 'tool_call_end',
			session_id: 's1',
			message_id: 'm1',
			id: 'tc1',
			ok: true,
			result: 'ok'
		});
		applyEvent('s1', {
			type: 'message_end',
			session_id: 's1',
			message_id: 'm1',
			text: 'done'
		});

		const list = get(messagesBySession).s1;
		expect(list).toHaveLength(2);
		expect(list[0].role).toBe('user');
		expect(list[0].text).toBe('hello');
		expect(list[1].role).toBe('assistant');
		expect(list[1].text).toBe('done');
		expect(list[1].streaming).toBe(false);
		expect(list[1].toolCalls).toHaveLength(1);
		expect(list[1].toolCalls[0].running).toBe(false);
		expect(list[1].toolCalls[0].ok).toBe(true);
		expect(list[1].toolCalls[0].result).toBe('ok');
	});

	it('records error events on the pending assistant message', () => {
		appendUserMessage('s2', 'hi');
		applyEvent('s2', { type: 'message_start', session_id: 's2', message_id: 'm2' });
		applyEvent('s2', {
			type: 'error',
			session_id: 's2',
			message_id: 'm2',
			error: 'kaboom'
		});
		const list = get(messagesBySession).s2;
		expect(list).toHaveLength(2);
		expect(list[1].error).toBe('kaboom');
		expect(list[1].streaming).toBe(false);
	});
});
