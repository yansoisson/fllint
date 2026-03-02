import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';

export default defineConfig({
	plugins: [sveltekit()],
	server: {
		port: 5173,
		proxy: {
			'/api': {
				target: 'http://localhost:8420',
				changeOrigin: true,
				configure: (proxy) => {
					proxy.on('proxyRes', (proxyRes) => {
						if (proxyRes.headers['content-type']?.includes('text/event-stream')) {
							proxyRes.headers['cache-control'] = 'no-cache';
							proxyRes.headers['x-accel-buffering'] = 'no';
						}
					});
				}
			}
		}
	}
});
