<script>
	import { onMount } from 'svelte';
	import { authFetch } from '$lib/auth';
	import { preferences } from '$lib/preferences';

	let playlists = $state([]);
	let loading = $state(true);
	let error = $state(null);

	let showCreateModal = $state(false);
	let newName = $state('');
	let newDescription = $state('');
	let creating = $state(false);

	function playlistCoverSrc(playlist) {
		const version = playlist?.date_updated || playlist?.date_created || '0';
		return `/api/playlists/${playlist.id}/cover?v=${encodeURIComponent(version)}`;
	}

	async function readError(response, fallback) {
		try {
			const data = await response.json();
			return data?.error || fallback;
		} catch {
			return fallback;
		}
	}

	async function loadPlaylists() {
		try {
			loading = true;
			error = null;
			const res = await authFetch('/api/playlists');
			if (!res.ok) throw new Error(await readError(res, 'Failed to fetch playlists'));
			playlists = await res.json();
		} catch (err) {
			error = err.message;
		} finally {
			loading = false;
		}
	}

	async function createPlaylist(e) {
		e.preventDefault();
		if (!newName.trim()) return;
		creating = true;
		try {
			const res = await authFetch('/api/playlists', {
				method: 'POST',
				body: JSON.stringify({ name: newName, description: newDescription, video_ids: [] })
			});
			if (!res.ok) throw new Error(await readError(res, 'Failed to create playlist'));
			await loadPlaylists();
			showCreateModal = false;
			newName = '';
			newDescription = '';
		} catch (err) {
			error = err.message;
		} finally {
			creating = false;
		}
	}

	async function deletePlaylist(id) {
		if (!confirm('Are you sure you want to delete this playlist?')) return;
		try {
			const res = await authFetch(`/api/playlists/${id}`, { method: 'DELETE' });
			if (!res.ok) throw new Error(await readError(res, 'Failed to delete playlist'));
			await loadPlaylists();
		} catch (err) {
			error = err.message;
		}
	}

	onMount(() => {
		loadPlaylists();
	});
</script>

<svelte:head>
	<title>Collectarr - Playlists</title>
</svelte:head>

<div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
	<div class="flex items-center justify-between mb-6">
		<h1 class="text-xl font-bold tracking-widest uppercase text-white">Playlists</h1>
		<button
			class="h-9 px-4 text-xs uppercase tracking-wider border border-neutral-600 bg-neutral-900 text-white hover:border-neutral-400 transition-colors"
			onclick={() => (showCreateModal = true)}
		>
			Create Playlist
		</button>
	</div>

	{#if loading}
		<div class="flex justify-center min-h-[50vh] items-center">
			<span class="loading loading-spinner loading-lg text-white"></span>
		</div>
	{:else if error && playlists.length === 0}
		<div
			class="border border-neutral-800 p-8 text-center text-neutral-500 uppercase tracking-widest text-sm"
		>
			Error: {error}
		</div>
	{:else if playlists.length === 0}
		<div class="border border-neutral-800 p-12 text-center flex flex-col items-center">
			<p class="text-neutral-400 uppercase tracking-widest mb-2">No Playlists</p>
			<p class="text-sm text-neutral-600">Create a playlist to organize your videos.</p>
		</div>
	{:else}
		<div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-6">
			{#each playlists as playlist (playlist.id)}
				<div
					class="border border-neutral-800 bg-black p-4 flex flex-col gap-4 hover:border-neutral-600 transition-colors"
				>
					<a href="/playlists/{playlist.id}" class="block">
						<div
							class="relative aspect-video overflow-hidden border border-neutral-800 bg-neutral-950"
						>
							<img
								src={playlistCoverSrc(playlist)}
								alt="{playlist.name} cover"
								class="absolute inset-0 h-full w-full object-cover"
								class:blur-md={$preferences.incognito}
								class:brightness-75={$preferences.incognito}
							/>
							{#if $preferences.incognito}
								<div class="absolute inset-0 flex items-center justify-center bg-black/50">
									<span class="text-[10px] font-semibold uppercase tracking-[0.3em] text-white/70"
										>Incognito</span
									>
								</div>
							{/if}
						</div>
					</a>
					<div class="flex justify-between items-start">
						<a
							href="/playlists/{playlist.id}"
							class="text-lg font-semibold text-white hover:underline truncate"
						>
							{playlist.name}
						</a>
						<button
							class="text-neutral-500 hover:text-red-500 transition-colors"
							onclick={() => deletePlaylist(playlist.id)}
							aria-label="Delete playlist"
						>
							<svg class="w-5 h-5" viewBox="0 0 24 24" fill="currentColor">
								<path
									d="M6 19c0 1.1.9 2 2 2h8c1.1 0 2-.9 2-2V7H6v12zM19 4h-3.5l-1-1h-5l-1 1H5v2h14V4z"
								/>
							</svg>
						</button>
					</div>
					<p class="text-sm text-neutral-400 line-clamp-2 min-h-[2.5rem]">
						{playlist.description || 'No description'}
					</p>
					<div class="text-xs text-neutral-500 uppercase tracking-widest mt-2">
						{playlist.item_count}
						{playlist.item_count === 1 ? 'item' : 'items'}
					</div>
				</div>
			{/each}
		</div>
	{/if}
</div>

{#if showCreateModal}
	<div class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/80 backdrop-blur-sm">
		<div class="w-full max-w-md border border-neutral-800 bg-black p-6 shadow-2xl">
			<h2 class="text-lg font-semibold text-white mb-4">Create Playlist</h2>
			<form onsubmit={createPlaylist} class="flex flex-col gap-4">
				<label class="flex flex-col gap-1">
					<span class="text-xs uppercase tracking-widest text-neutral-500">Name</span>
					<input
						type="text"
						bind:value={newName}
						class="h-10 bg-neutral-900 border border-neutral-700 px-3 text-white outline-none focus:border-neutral-500"
						required
					/>
				</label>
				<label class="flex flex-col gap-1">
					<span class="text-xs uppercase tracking-widest text-neutral-500"
						>Description (optional)</span
					>
					<textarea
						bind:value={newDescription}
						class="h-24 bg-neutral-900 border border-neutral-700 p-3 text-white outline-none focus:border-neutral-500 resize-none"
					></textarea>
				</label>
				<div class="flex justify-end gap-3 mt-4">
					<button
						type="button"
						class="px-4 py-2 text-xs font-bold uppercase tracking-widest text-neutral-400 hover:text-white"
						onclick={() => (showCreateModal = false)}
					>
						Cancel
					</button>
					<button
						type="submit"
						class="bg-white text-black px-4 py-2 text-xs font-bold uppercase tracking-widest hover:bg-neutral-200 disabled:opacity-50"
						disabled={creating || !newName.trim()}
					>
						{creating ? 'Creating...' : 'Create'}
					</button>
				</div>
			</form>
		</div>
	</div>
{/if}
