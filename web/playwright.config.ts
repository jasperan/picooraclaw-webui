import { defineConfig } from '@playwright/test';

export default defineConfig({
	testDir: './tests',
	timeout: 30_000,
	use: {
		baseURL: 'http://localhost:3000',
		trace: 'on-first-retry'
	},
	webServer: {
		command:
			'../bin/picooraclaw-webui --listen :3000 --picooraclaw-url http://localhost:8090',
		url: 'http://localhost:3000',
		reuseExistingServer: true,
		timeout: 15_000
	}
});
