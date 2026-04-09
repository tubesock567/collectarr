<script>
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { authFetch } from '$lib/auth';
	import VideoCard from '$lib/components/VideoCard.svelte';

	let playlistId = $derived($page.params.id);
	let playlist = $state(null);
	let loading = $state(true);
	let error = $state(null);

	let showEditModal = $state(false);
	let editName = $state('');
	let editDescription = $state('');
	let saving = $state(false);

	async function readError(response, fallback) {
		try {
			const data = await response.json();
			return data?.error || fallback;
		} catch {
			return fallback;
		}
	}

	async function loadPlaylist() {
		try {
			loading = true;
			error = null;
			const res = await authFetch(`/api/playlists/${playlistId}`);
			if (!res.ok) throw new Error(await readError(res, 'Failed to fetch playlist'));
			playlist = await res.json();
			editName = playlist.name;
			editDescription = playlist.description || '';
		} catch (err) {
			error = err.message;
		} finally {
			loading = false;
		}
	}

	async function savePlaylist(e) {
		e.preventDefault();
		if (!editName.trim()) return;
		saving = true;
		try {
			const res = await authFetch(`/api/playlists/${playlistId}`, {
				method: 'PUT',
				body: JSON.stringify({ name: editName, description: editDescription })
			});
			if (!res.ok) throw new Error(await readError(res, 'Failed to update playlist'));
			playlist = await res.json();
			showEditModal = false;
		} catch (err) {
			error = err.message;
		} finally {
			saving = false;
		}
	}

	async function removeItem(videoId) {
		if (!confirm('Remove this video from the playlist?')) return;
		try {
			const res = await authFetch(`/api/playlists/${playlistId}/items/${videoId}`, {
				method: 'DELETE'
			});
			if (!res.ok) throw new Error(await readError(res, 'Failed to remove item'));
			playlist = await res.json();
		} catch (err) {
			error = err.message;
		}
	}

	onMount(() => {
		loadPlaylist();
	});
</script>

<svelte:head>
	<title>Collectarr - {playlist?.name || 'Playlist'}</title>
</svelte:head>

<div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
	<div class="mb-6 flex items-start gap-4">
		<a
			href="/playlists"
			class="h-9 w-9 flex items-center justify-center border border-neutral-600 hover:border-neutral-400 transition-colors text-neutral-300 hover:text-white bg-neutral-900"
			aria-label="Back to playlists"
		>
			<svg class="w-5 h-5" viewBox="0 0 24 24" fill="currentColor"
				><path d="M20 11H7.83l5.59-5.59L12 4l-8 8 8 8 1.41-1.41L7.83 13H20v-2z" /></svg
			>
		</a>
		<div class="flex-1 flex justify-between items-start">
			<div class="flex-1">
				<h1 class="text-xl font-bold tracking-widest uppercase text-white">
					{playlist?.name || 'Loading...'}
				</h1>
				{#if playlist?.description}
					<p class="text-sm text-neutral-400 mt-2 max-w-3xl">{playlist.description}</p>
				{/if}
				<div class="text-[10px] text-neutral-500 uppercase tracking-widest mt-2">
					{playlist?.items?.length || 0}
					{playlist?.items?.length === 1 ? 'item' : 'items'}
				</div>
			</div>
			{#if playlist}
				<button
					class="h-9 px-4 text-xs uppercase tracking-wider border border-neutral-600 bg-neutral-900 text-white hover:border-neutral-400 transition-colors"
					onclick={() => (showEditModal = true)}
				>
					Edit Info
				</button>
			{/if}
		</div>
	</div>

	{#if loading}
		<div class="flex justify-center min-h-[50vh] items-center">
			<span class="loading loading-spinner loading-lg text-white"></span>
		</div>
	{:else if error}
		<div
			class="border border-neutral-800 p-8 text-center text-neutral-500 uppercase tracking-widest text-sm"
		>
			Error: {error}
		</div>
	{:else if !playlist?.items || playlist.items.length === 0}
		<div class="border border-neutral-800 p-12 text-center flex flex-col items-center">
			<p class="text-neutral-400 uppercase tracking-widest mb-2">Playlist is empty</p>
			<p class="text-sm text-neutral-600 mb-6">Select videos in the library to add them here.</p>
			<a
				href="/"
				class="h-9 flex items-center px-4 text-xs font-bold uppercase tracking-widest border border-white bg-white text-black hover:bg-white/80 transition-colors"
				>Go to Library</a
			>
		</div>
	{:else}
		<div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-6">
			{#each playlist.items as video (video.id)}
				<div class="relative group">
					<VideoCard {video} playlistId={playlist.id} />
					<button
						class="absolute top-2 right-2 h-7 w-7 bg-black/80 border border-neutral-600 text-neutral-400 hover:text-white hover:border-red-500 hover:bg-red-900 transition-colors z-20 flex items-center justify-center"
						onclick={() => removeItem(video.id)}
						aria-label="Remove from playlist"
						title="Remove from playlist"
					>
						<svg class="w-4 h-4" viewBox="0 0 24 24" fill="currentColor">
							<path
								d="M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z"
							/>
						</svg>
					</button>
				</div>
			{/each}
		</div>
	{/if}
</div>

{#if showEditModal}
	<div class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/80 backdrop-blur-sm">
		<div class="w-full max-w-md border border-neutral-800 bg-black p-6 shadow-2xl">
			<h2 class="text-lg font-semibold text-white mb-4">Edit Playlist Info</h2>
			<form onsubmit={savePlaylist} class="flex flex-col gap-4">
				<label class="flex flex-col gap-1">
					<span class="text-xs uppercase tracking-widest text-neutral-500">Name</span>
					<input
						type="text"
						bind:value={editName}
						class="h-10 bg-neutral-900 border border-neutral-700 px-3 text-white outline-none focus:border-neutral-500"
						required
					/>
				</label>
				<label class="flex flex-col gap-1">
					<span class="text-xs uppercase tracking-widest text-neutral-500">Description</span>
					<textarea
						bind:value={editDescription}
						class="h-24 bg-neutral-900 border border-neutral-700 p-3 text-white outline-none focus:border-neutral-500 resize-none"
					></textarea>
				</label>
				<div class="flex justify-end gap-3 mt-4">
					<button
						type="button"
						class="px-4 py-2 text-xs font-bold uppercase tracking-widest text-neutral-400 hover:text-white"
						onclick={() => {
							showEditModal = false;
							editName = playlist.name;
							editDescription = playlist.description || '';
						}}
					>
						Cancel
					</button>
					<button
						type="submit"
						class="bg-white text-black px-4 py-2 text-xs font-bold uppercase tracking-widest hover:bg-neutral-200 disabled:opacity-50"
						disabled={saving || !editName.trim()}
					>
						{saving ? 'Saving...' : 'Save'}
					</button>
				</div>
			</form>
		</div>
	</div>
{/if}
