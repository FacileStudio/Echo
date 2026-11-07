import { base, expectOk, headers } from './capture';

export interface Upload {
	id: string;
	next_chunk: number;
	size: number;
}

export async function startUpload(slug: string, contentType: string): Promise<Upload> {
	const res = await fetch(`${base(slug)}/recording`, {
		method: 'POST',
		credentials: 'include',
		headers: headers('', true),
		body: JSON.stringify({ content_type: contentType })
	});
	await expectOk(res);
	return (await res.json()) as Upload;
}

export type ChunkResult = { ok: true; upload: Upload } | { ok: false; gap?: Upload; status: number };

export async function putChunk(slug: string, id: string, index: number, data: Blob): Promise<ChunkResult> {
	const res = await fetch(`${base(slug)}/recording/${id}/chunks/${index}`, {
		method: 'PUT',
		credentials: 'include',
		headers: headers(''),
		body: data
	});
	if (res.ok) return { ok: true, upload: (await res.json()) as Upload };
	if (res.status === 409) {
		const body = (await res.json().catch(() => null)) as Upload | null;
		return { ok: false, status: 409, gap: body && 'next_chunk' in body ? body : undefined };
	}
	return { ok: false, status: res.status };
}

export async function finalizeUpload(slug: string, id: string): Promise<void> {
	await expectOk(
		await fetch(`${base(slug)}/recording/${id}/finalize`, {
			method: 'POST',
			credentials: 'include',
			headers: headers('')
		})
	);
}

/** Finalizes on pagehide where a normal request would be cancelled. The server also finalizes when the call ends. */
export function finalizeOnExit(slug: string, id: string): void {
	void fetch(`${base(slug)}/recording/${id}/finalize`, {
		method: 'POST',
		credentials: 'include',
		headers: headers(''),
		keepalive: true
	}).catch(() => undefined);
}
