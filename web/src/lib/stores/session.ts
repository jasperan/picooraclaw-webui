import { writable } from 'svelte/store';

export type Session = { id: string; title: string; last_at: number };

export const currentSession = writable<string>('default');
export const sessions = writable<Session[]>([]);

export async function loadSessions() {
	const res = await fetch('/api/sessions');
	if (res.ok) sessions.set(await res.json());
}
