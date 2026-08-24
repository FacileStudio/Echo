<script lang="ts">
	import { onMount } from 'svelte'
	import { page } from '$app/state'
	import { Room, RoomEvent, Track } from 'livekit-client'

	const apiBase = import.meta.env.VITE_API_BASE ?? 'http://localhost:8080'
	const slug = $derived(page.params.slug)

	let displayName = $state('')
	let token = $state<string | null>(null)
	let livekitUrl = $state('')
	let status = $state<'idle' | 'connecting' | 'connected'>('idle')
	let published = $state(false)
	let error = $state('')
	let participants = $state<string[]>([])

	let videoEl: HTMLVideoElement | undefined = $state()
	let room: Room | null = null

	async function join() {
		error = ''
		status = 'connecting'
		try {
			const res = await fetch(`${apiBase}/api/rooms/${slug}/token`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ identity: crypto.randomUUID(), displayName, guest: false })
			})
			if (!res.ok) throw new Error(`token request failed: ${res.status}`)
			const data = await res.json()
			token = data.token
			livekitUrl = data.url

			const url: string = data.url
			room = new Room({ adaptiveStream: true, dynacast: true })
			room.on(RoomEvent.ParticipantConnected, updateParticipants)
			room.on(RoomEvent.ParticipantDisconnected, updateParticipants)
			await room.connect(url, data.token)
			try {
				await room.localParticipant.setCameraEnabled(true)
				const pub = room.localParticipant.getTrackPublication(Track.Source.Camera)
				const track = pub?.videoTrack
				if (track && videoEl) {
					track.attach(videoEl)
				}
				published = true
			} catch {
				const ctx = new AudioContext()
				const osc = ctx.createOscillator()
				const dst = ctx.createMediaStreamDestination()
				osc.connect(dst)
				osc.start()
				await room.localParticipant.publishTrack(dst.stream.getAudioTracks()[0])
				published = true
			}
			updateParticipants()
			status = 'connected'
		} catch (e) {
			error = e instanceof Error ? e.message : String(e)
			status = 'idle'
		}
	}

	function updateParticipants() {
		if (!room) return
		participants = [room.localParticipant, ...Array.from(room.remoteParticipants.values())].map(
			(p) => p.name || p.identity
		)
	}

	onMount(() => {
		return () => {
			room?.disconnect()
			room?.localParticipant.getTrackPublication(Track.Source.Camera)?.track?.stop()
		}
	})
</script>

<svelte:head><title>Echo · {slug}</title></svelte:head>

<main class="mx-auto flex min-h-screen max-w-xl flex-col justify-center gap-4 p-6">
	<h1 class="text-2xl font-bold">Join “{slug}”</h1>

	{#if status !== 'connected'}
		<form onsubmit={(e) => { e.preventDefault(); join() }} class="flex gap-2">
			<input
				class="flex-1 rounded border px-3 py-2"
				bind:value={displayName}
				placeholder="Display name"
				required
			/>
			<button
				class="rounded bg-black px-4 py-2 text-white disabled:opacity-50"
				disabled={status === 'connecting'}
			>
				{status === 'connecting' ? 'Connecting…' : 'Join'}
			</button>
		</form>
	{/if}

	{#if error}
		<p class="rounded bg-red-100 p-3 text-red-700">{error}</p>
	{/if}

	{#if status === 'connected'}
		<p class="text-sm text-gray-500">Connected to {livekitUrl}{published ? ' · publishing' : ''}</p>
		<video bind:this={videoEl} autoplay muted playsinline class="aspect-video w-full rounded bg-black"></video>
		<ul class="text-sm">
			{#each participants as p (p)}
				<li>{p}</li>
			{/each}
		</ul>
	{/if}
</main>
