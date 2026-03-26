import adapter from '@sveltejs/adapter-static';

/** @type {import('@sveltejs/kit').Config} */
const config = {
	kit: {
		adapter: adapter({
			pages: '../web',
			assets: '../web',
			fallback: undefined,
			precompress: false,
			strict: true
		}),
		paths: {
			base: '/ui'
		}
	}
};

export default config;
