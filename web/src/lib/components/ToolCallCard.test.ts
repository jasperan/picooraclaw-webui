import { render, fireEvent } from '@testing-library/svelte';
import { describe, it, expect } from 'vitest';
import ToolCallCard from './ToolCallCard.svelte';
import type { ToolCall } from '$lib/stores/messages';

describe('ToolCallCard', () => {
	it('renders collapsed by default with tool name in the button', () => {
		const call: ToolCall = {
			id: '1',
			tool: 'remember',
			args: { text: 'hi' },
			ok: true,
			result: 'ok',
			running: false
		};
		const { getByRole, queryByText } = render(ToolCallCard, { props: { call } });
		const btn = getByRole('button');
		expect(btn.textContent).toContain('remember');
		// Collapsed: args JSON pre block must not be present yet.
		expect(queryByText(/"text": "hi"/)).toBeNull();
	});

	it('expands on click and shows args JSON', async () => {
		const call: ToolCall = {
			id: '1',
			tool: 'remember',
			args: { text: 'hi' },
			ok: true,
			result: 'ok',
			running: false
		};
		const { getByRole, findByText } = render(ToolCallCard, { props: { call } });
		await fireEvent.click(getByRole('button'));
		expect(await findByText(/"text": "hi"/)).toBeTruthy();
	});

	it('applies error status when ok=false', () => {
		const call: ToolCall = {
			id: '1',
			tool: 'remember',
			ok: false,
			result: 'boom',
			running: false
		};
		const { container } = render(ToolCallCard, { props: { call } });
		const card = container.querySelector('.tool-card');
		expect(card).toBeTruthy();
		expect(card?.getAttribute('data-status')).toBe('error');
	});
});
