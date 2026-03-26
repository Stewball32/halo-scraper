import { sveltekit } from '@sveltejs/kit/vite';
import tailwindcss from '@tailwindcss/vite';
import { defineConfig } from 'vite';

export default defineConfig({
	plugins: [tailwindcss(), sveltekit()],
	server: {
		proxy: {
			'/api': 'http://localhost:9000',
			'/ws': {
				target: 'http://localhost:9000',
				ws: true
			},
			'/rooms': 'http://localhost:9000'
		}
	}
});
