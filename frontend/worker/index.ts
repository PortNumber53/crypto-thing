export default {
	async fetch(request, env): Promise<Response> {
		const url = new URL(request.url);

		if (url.pathname.startsWith('/api/')) {
			const target = new URL(url.pathname + url.search, env.BACKEND_URL);
			return fetch(target.toString(), {
				method: request.method,
				headers: request.headers,
				body: request.method !== 'GET' && request.method !== 'HEAD' ? request.body : undefined,
			});
		}

		return new Response(null, { status: 404 });
	},
} satisfies ExportedHandler<Env>;
