const ANTHROPIC_API_KEY = process.env.ANTHROPIC_API_KEY;

if (!ANTHROPIC_API_KEY) {
    console.error('ANTHROPIC_API_KEY is required');
    process.exit(1);
}

Bun.serve({
    port: 3100,
    async fetch(req) {
        if (req.method === 'OPTIONS') {
            return new Response(null, {
                headers: {
                    'Access-Control-Allow-Origin': '*',
                    'Access-Control-Allow-Methods': 'POST',
                    'Access-Control-Allow-Headers': 'Content-Type'
                }
            });
        }

        if (req.method !== 'POST') {
            return new Response('Method not allowed', { status: 405 });
        }

        const { system, message } = await req.json();

        const response = await fetch('https://api.anthropic.com/v1/messages', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
                'x-api-key': ANTHROPIC_API_KEY,
                'anthropic-version': '2023-06-01'
            },
            body: JSON.stringify({
                model: 'claude-sonnet-4-5-20241022',
                max_tokens: 4096,
                system,
                messages: [{ role: 'user', content: message }]
            })
        });

        if (!response.ok) {
            const err = await response.text();

            console.error(`Anthropic API error ${response.status}: ${err}`);

            return new Response(JSON.stringify({ error: err }), {
                status: response.status,
                headers: { 'Content-Type': 'application/json' }
            });
        }

        const data = await response.json() as any;
        const content = data.content?.[0]?.text ?? '';

        return Response.json({ content });
    }
});

console.log('AI proxy listening on :3100');
