import adapter from '@sveltejs/adapter-static';

/** @type {import('@sveltejs/kit').Config} */
const config = {
	compilerOptions: {
		// Force runes mode for the project, except for libraries. Can be removed in svelte 6.
		runes: ({ filename }) =>
			filename.split(/[/\\]/).includes('node_modules') ? undefined : true
	},
	kit: {
		// Pure SPA: everything talks to the API over HTTP, nothing needs SSR.
		adapter: adapter({ fallback: 'index.html', strict: false })
	}
};

export default config;
