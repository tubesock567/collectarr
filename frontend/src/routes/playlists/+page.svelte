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
	let createPlaylistDialogEl = $state(null);
	let createPlaylistTriggerEl = $state(null);

	function scrambleText(text) {
		if (!text) return '';
		const chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789';
		let scrambled = '';
		for (let i = 0; i < text.length; i++) {
			scrambled += chars[Math.floor(Math.random() * chars.length)];
		}
		return scrambled;
	}

	function displayPlaylistName(playlist) {
		return $preferences.incognito ? scrambleText(playlist?.name) : playlist?.name;
	}

	function displayPlaylistDescription(playlist) {
		if ($preferences.incognito) return 'Description hidden';
		return playlist?.description || 'No description';
	}

	function getFocusableElements(container) {
		if (!container) return [];
		return Array.from(
			container.querySelectorAll(
				'button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])'
			)
		);
	}

	function focusDialog(container) {
		const [firstFocusable] = getFocusableElements(container);
		if (firstFocusable instanceof HTMLElement) {
			firstFocusable.focus();
			return;
		}
		container?.focus();
	}

	function trapDialogFocus(event, container) {
		if (event.key !== 'Tab' || !container) return;

		const focusableElements = getFocusableElements(container);
		if (focusableElements.length === 0) {
			event.preventDefault();
			container.focus();
			return;
		}

		const firstElement = focusableElements[0];
		const lastElement = focusableElements[focusableElements.length - 1];

		if (event.shiftKey && document.activeElement === firstElement) {
			event.preventDefault();
			lastElement.focus();
		} else if (!event.shiftKey && document.activeElement === lastElement) {
			event.preventDefault();
			firstElement.focus();
		}
	}

	function openCreateModal(event) {
		createPlaylistTriggerEl = event.currentTarget;
		showCreateModal = true;
	}

	function closeCreateModal({ restoreFocus = true } = {}) {
		showCreateModal = false;
		if (restoreFocus && createPlaylistTriggerEl instanceof HTMLElement) {
			queueMicrotask(() => createPlaylistTriggerEl?.focus());
		}
	}

	function handleCreateDialogKeydown(event) {
		if (event.key === 'Escape') {
			event.preventDefault();
			closeCreateModal();
			return;
		}

		trapDialogFocus(event, createPlaylistDialogEl);
	}

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
			closeCreateModal({ restoreFocus: false });
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

	$effect(() => {
		if (!showCreateModal) {
			document.body.style.overflow = '';
			return;
		}

		document.body.style.overflow = 'hidden';
		queueMicrotask(() => focusDialog(createPlaylistDialogEl));

		return () => {
			document.body.style.overflow = '';
		};
	});
</script>

<svelte:head>
	<title>Collectarr - Playlists</title>
</svelte:head>

<div class="mx-auto flex h-full w-full max-w-[1600px] flex-col px-4 py-6 sm:px-6">
	<div class="mono-panel mb-6 rounded-[18px]">
		<div
			class="flex flex-col gap-4 px-4 py-4 sm:flex-row sm:items-center sm:justify-between sm:px-6 shadow-[inset_0_-1px_0_rgba(255,255,255,0.03)]"
		>
			<div>
				<p class="text-[10px] font-bold uppercase tracking-[0.32em] text-neutral-500">Archive</p>
				<h1 class="mt-2 text-sm font-bold uppercase tracking-[0.22em] text-white sm:text-base">
					Playlists
				</h1>
				<p class="mt-2 text-[10px] font-bold uppercase tracking-[0.24em] text-neutral-600">
					{#if loading}
						Syncing playlist index
					{:else}
						{playlists.length} registered {playlists.length === 1 ? 'playlist' : 'playlists'}
					{/if}
				</p>
			</div>
			<button
				class="mono-control h-9 rounded-[8px] px-4 text-[10px] font-bold uppercase tracking-[0.24em] text-neutral-300 transition-colors hover:text-white"
				onclick={openCreateModal}
			>
				Create Playlist
			</button>
		</div>
		<div
			class="grid gap-3 px-4 py-3 text-[10px] font-bold uppercase tracking-[0.24em] text-neutral-600 sm:grid-cols-3 sm:px-6"
		>
			<div class="mono-panel-soft rounded-[10px] px-3 py-2">Catalog: Playlist Registry</div>
			<div class="mono-panel-soft rounded-[10px] px-3 py-2">Mode: Curated Collections</div>
			<div class="mono-panel-soft rounded-[10px] px-3 py-2">
				State: {$preferences.incognito ? 'Incognito' : 'Visible'}
			</div>
		</div>
	</div>

	{#if loading}
		<div class="flex justify-center min-h-[50vh] items-center">
			<span class="loading loading-spinner loading-lg text-white"></span>
		</div>
	{:else if error && playlists.length === 0}
		<div
			class="mono-panel rounded-[16px] p-8 text-center text-sm font-bold uppercase tracking-[0.24em] text-neutral-500"
		>
			Error: {error}
		</div>
	{:else if playlists.length === 0}
		<div class="mono-panel rounded-[16px] flex flex-col items-center p-12 text-center">
			<p class="mb-2 text-[10px] font-bold uppercase tracking-[0.32em] text-neutral-500">
				No Playlists
			</p>
			<p class="text-sm text-neutral-600">Create a playlist to organize your videos.</p>
		</div>
	{:else}
		<div class="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-4">
			{#each playlists as playlist (playlist.id)}
				<div
					class="mono-panel-soft mono-panel-hover flex w-full flex-col gap-4 rounded-[14px] p-2.5 transition-colors"
				>
					<a href="/playlists/{playlist.id}" class="block rounded-[10px] bg-[#1b1c1f] p-2">
						<div class="relative aspect-video overflow-hidden rounded-[8px] bg-neutral-950">
							<img
								src={playlistCoverSrc(playlist)}
								alt="{displayPlaylistName(playlist)} cover"
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
					<div class="flex items-start justify-between gap-4 px-1">
						<a
							href="/playlists/{playlist.id}"
							class="truncate text-[13px] font-bold uppercase tracking-[0.12em] text-neutral-200 transition-colors hover:text-white"
						>
							{displayPlaylistName(playlist)}
						</a>
						<button
							class="rounded-[6px] p-1 text-neutral-500 transition-colors hover:bg-[#242529] hover:text-red-400"
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
					<p class="min-h-[2.5rem] px-1 text-sm text-neutral-500 line-clamp-2">
						{displayPlaylistDescription(playlist)}
					</p>
					<div
						class="mt-1 flex items-center justify-between px-1 pt-2 text-[10px] font-bold uppercase tracking-[0.24em] text-neutral-600"
					>
						<span>Items</span>
						<span>{playlist.item_count} {playlist.item_count === 1 ? 'Unit' : 'Units'}</span>
					</div>
				</div>
			{/each}
		</div>
	{/if}
</div>

{#if showCreateModal}
	<div class="fixed inset-0 z-50 flex items-center justify-center p-4">
		<button
			type="button"
			class="absolute inset-0 bg-black/80 backdrop-blur-sm"
			aria-label="Close create playlist dialog"
			onclick={() => closeCreateModal()}
		></button>
		<div
			bind:this={createPlaylistDialogEl}
			role="dialog"
			aria-modal="true"
			aria-labelledby="create-playlist-title"
			tabindex="-1"
			onkeydown={handleCreateDialogKeydown}
			class="mono-panel relative z-10 w-full max-w-md rounded-[18px] p-6 shadow-2xl"
		>
			<h2 id="create-playlist-title" class="text-lg font-semibold text-white mb-4">
				Create Playlist
			</h2>
			<form onsubmit={createPlaylist} class="flex flex-col gap-4">
				<label class="flex flex-col gap-1">
					<span class="text-xs uppercase tracking-widest text-neutral-500">Name</span>
					<input
						type="text"
						bind:value={newName}
						class="mono-control h-10 rounded-[8px] px-3 text-white outline-none"
						required
					/>
				</label>
				<label class="flex flex-col gap-1">
					<span class="text-xs uppercase tracking-widest text-neutral-500"
						>Description (optional)</span
					>
					<textarea
						bind:value={newDescription}
						class="mono-control h-24 rounded-[8px] p-3 text-white outline-none resize-none"
					></textarea>
				</label>
				<div class="flex justify-end gap-3 mt-4">
					<button
						type="button"
						class="px-4 py-2 text-xs font-bold uppercase tracking-widest text-neutral-400 hover:text-white"
						onclick={() => closeCreateModal()}
					>
						Cancel
					</button>
					<button
						type="submit"
						class="mono-control-active rounded-[8px] px-4 py-2 text-xs font-bold uppercase tracking-widest hover:bg-neutral-200 disabled:opacity-50"
						disabled={creating || !newName.trim()}
					>
						{creating ? 'Creating...' : 'Create'}
					</button>
				</div>
			</form>
		</div>
	</div>
{/if}
