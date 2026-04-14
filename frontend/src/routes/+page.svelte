<script>
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { authFetch } from '$lib/auth';
	import { toast } from '$lib/toast';
	import MetadataTokenInput from '$lib/components/MetadataTokenInput.svelte';
	import VideoCard from '$lib/components/VideoCard.svelte';

	let videos = $state([]);
	let loading = $state(true);
	let error = $state(null);
	let searchQuery = $state('');
	let sortBy = $state('dateAdded');
	let sortOrder = $state('desc');
	let columnCount = $state(4);
	let currentPage = $state(1);
	let itemsPerPage = $state(24);
	let showSortDropdown = $state(false);
	let showColumnDropdown = $state(false);
	let mobileSortDropdownEl = $state(null);
	let desktopSortDropdownEl = $state(null);
	let columnDropdownEl = $state(null);
	let metadataOptions = $state({ tags: [], actors: [] });
	let selectionMode = $state(false);
	let selectedVideoIds = $state([]);
	let showMetadataPanel = $state(false);
	let singleTagsDraft = $state([]);
	let singleActorsDraft = $state([]);
	let bulkAddTags = $state([]);
	let bulkRemoveTags = $state([]);
	let bulkAddActors = $state([]);
	let bulkRemoveActors = $state([]);
	let savingMetadata = $state(false);
	let metadataMessage = $state('');
	let metadataPanelOverlayEl = $state(null);

	let playlists = $state([]);
	let showPlaylistPanel = $state(false);
	let newPlaylistName = $state('');
	let savingPlaylist = $state(false);
	let playlistMessage = $state('');
	let playlistPanelOverlayEl = $state(null);

	let continueWatching = $state([]);
	let loadingContinueWatching = $state(true);
	let failedContinueWatchingThumbnails = $state({});

	const normalizedSearchQuery = $derived(searchQuery.trim().toLowerCase());
	const filteredVideos = $derived.by(() => {
		const query = normalizedSearchQuery;
		if (!query) {
			return videos;
		}

		return videos.filter((video) => {
			const title = (video.title || '').toLowerCase();
			const addedDate = (video.date_added || '').toLowerCase();
			const tags = video.tags || [];
			const actors = video.actors || [];

			const titleMatch = title.includes(query);
			const dateMatch = addedDate.includes(query);
			const tagsMatch = tags.some((tag) => tag.toLowerCase().includes(query));
			const actorsMatch = actors.some((actor) => actor.toLowerCase().includes(query));

			return titleMatch || dateMatch || tagsMatch || actorsMatch;
		});
	});

	const totalPages = $derived(Math.max(1, Math.ceil(filteredVideos.length / itemsPerPage)));
	const sortedVideos = $derived.by(() => {
		const sorted = [...filteredVideos];
		sorted.sort((a, b) => {
			let valA;
			let valB;
			if (sortBy === 'duration') {
				valA = a.duration || 0;
				valB = b.duration || 0;
			} else if (sortBy === 'alphabetical') {
				valA = (a.title || '').toLowerCase();
				valB = (b.title || '').toLowerCase();
			} else {
				valA = new Date(a.date_added || 0).getTime();
				valB = new Date(b.date_added || 0).getTime();
			}
			if (valA < valB) return sortOrder === 'asc' ? -1 : 1;
			if (valA > valB) return sortOrder === 'asc' ? 1 : -1;
			return 0;
		});
		return sorted;
	});
	const paginatedVideos = $derived.by(() => {
		const start = (currentPage - 1) * itemsPerPage;
		return sortedVideos.slice(start, start + itemsPerPage);
	});
	const selectedVideos = $derived(videos.filter((video) => selectedVideoIds.includes(video.id)));
	const selectedCount = $derived(selectedVideoIds.length);
	const singleSelectedVideo = $derived(selectedCount === 1 ? selectedVideos[0] || null : null);
	const allPageSelected = $derived(
		paginatedVideos.length > 0 &&
			paginatedVideos.every((video) => selectedVideoIds.includes(video.id))
	);
	const allFilteredSelected = $derived(
		filteredVideos.length > 0 &&
			filteredVideos.every((video) => selectedVideoIds.includes(video.id))
	);
	const bulkHasChanges = $derived(
		bulkAddTags.length > 0 ||
			bulkRemoveTags.length > 0 ||
			bulkAddActors.length > 0 ||
			bulkRemoveActors.length > 0
	);

	function setSort(field) {
		if (sortBy === field) {
			sortOrder = sortOrder === 'asc' ? 'desc' : 'asc';
		} else {
			sortBy = field;
			sortOrder = field === 'alphabetical' ? 'asc' : 'desc';
		}
		currentPage = 1;
		showSortDropdown = false;
	}

	function toggleSortOrder() {
		sortOrder = sortOrder === 'asc' ? 'desc' : 'asc';
		currentPage = 1;
	}

	function setColumnCount(count) {
		columnCount = count;
		showColumnDropdown = false;
		currentPage = 1;
	}

	function goToPage(page) {
		if (page >= 1 && page <= totalPages) {
			currentPage = page;
			const scrollContainer = document.querySelector('[data-shell-scroll="true"]');
			if (scrollContainer instanceof HTMLElement) {
				scrollContainer.scrollTo({ top: 0, behavior: 'smooth' });
			} else {
				window.scrollTo({ top: 0, behavior: 'smooth' });
			}
		}
	}

	function getColumnClass(count) {
		const classes = {
			2: 'grid-cols-1 sm:grid-cols-2',
			3: 'grid-cols-1 sm:grid-cols-2 lg:grid-cols-3',
			4: 'grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4'
		};
		return classes[count] || classes[4];
	}

	function getSortLabel() {
		const labels = {
			dateAdded: 'Date added',
			duration: 'Duration',
			alphabetical: 'Alphabetical'
		};
		return labels[sortBy] || 'Date added';
	}

	function handleClickOutside(event) {
		const insideSortDropdown =
			mobileSortDropdownEl?.contains(event.target) || desktopSortDropdownEl?.contains(event.target);
		if (!insideSortDropdown) {
			showSortDropdown = false;
		}
		if (columnDropdownEl && !columnDropdownEl.contains(event.target)) {
			showColumnDropdown = false;
		}
	}

	function handleSearchInput(event) {
		searchQuery = event.currentTarget.value;
		currentPage = 1;
	}

	function readJSONSafe(response) {
		return response.json().catch(() => null);
	}

	async function fetchWithTimeout(url, options = {}, timeoutMs = 10000) {
		const controller = new AbortController();
		const timeoutId = setTimeout(() => controller.abort(), timeoutMs);

		try {
			return await authFetch(url, {
				...options,
				signal: controller.signal
			});
		} catch (err) {
			if (err?.name === 'AbortError') {
				throw new Error('Request timed out');
			}
			throw err;
		} finally {
			clearTimeout(timeoutId);
		}
	}

	async function readError(res, fallback) {
		const data = await readJSONSafe(res);
		return data?.error || fallback;
	}

	function ensureArray(value) {
		return Array.isArray(value) ? value : [];
	}

	function ensureMetadataOptions(value) {
		if (!value || typeof value !== 'object') {
			return { tags: [], actors: [] };
		}

		return {
			tags: ensureArray(value.tags),
			actors: ensureArray(value.actors)
		};
	}

	let isSpinning = $state(false);

	async function loadVideos() {
		isSpinning = true;
		const startTime = Date.now();
		try {
			const res = await fetchWithTimeout('/api/videos');
			if (!res.ok) {
				throw new Error(await readError(res, 'Failed to fetch videos'));
			}
			videos = ensureArray(await res.json());
		} finally {
			const elapsed = Date.now() - startTime;
			const minSpinTime = 500;
			if (elapsed < minSpinTime) {
				setTimeout(() => {
					isSpinning = false;
				}, minSpinTime - elapsed);
			} else {
				isSpinning = false;
			}
		}
	}

	async function loadMetadataOptions() {
		const res = await fetchWithTimeout('/api/videos/metadata/options');
		if (!res.ok) {
			throw new Error(await readError(res, 'Failed to fetch metadata options'));
		}
		metadataOptions = ensureMetadataOptions(await res.json());
	}

	async function loadPlaylists() {
		const res = await fetchWithTimeout('/api/playlists');
		if (!res.ok) {
			throw new Error(await readError(res, 'Failed to fetch playlists'));
		}
		playlists = ensureArray(await res.json());
	}

	async function loadContinueWatching() {
		try {
			const res = await fetchWithTimeout('/api/videos/continue-watching?limit=8');
			if (res.ok) {
				continueWatching = ensureArray(await res.json());
			}
		} catch (err) {
			// Silently ignore continue watching errors - it's not critical
			console.error('Failed to load continue watching:', err);
		} finally {
			loadingContinueWatching = false;
		}
	}

	function resetBulkDrafts() {
		bulkAddTags = [];
		bulkRemoveTags = [];
		bulkAddActors = [];
		bulkRemoveActors = [];
	}

	function toggleVideoSelection(id) {
		if (!selectionMode) {
			return;
		}
		if (selectedVideoIds.includes(id)) {
			selectedVideoIds = selectedVideoIds.filter((videoId) => videoId !== id);
		} else {
			selectedVideoIds = [...selectedVideoIds, id];
		}
		metadataMessage = '';
	}

	function addSelection(ids) {
		selectedVideoIds = Array.from(new Set([...selectedVideoIds, ...ids]));
		if (selectedVideoIds.length > 0) {
			showMetadataPanel = true;
		}
		metadataMessage = '';
	}

	function toggleCurrentPageSelection() {
		const pageIds = paginatedVideos.map((video) => video.id);
		if (pageIds.length === 0) {
			return;
		}
		if (allPageSelected) {
			selectedVideoIds = selectedVideoIds.filter((id) => !pageIds.includes(id));
		} else {
			addSelection(pageIds);
		}
		metadataMessage = '';
	}

	function toggleFilteredSelection() {
		const filteredIds = filteredVideos.map((video) => video.id);
		if (filteredIds.length === 0) {
			return;
		}
		if (allFilteredSelected) {
			selectedVideoIds = selectedVideoIds.filter((id) => !filteredIds.includes(id));
		} else {
			addSelection(filteredIds);
		}
		metadataMessage = '';
	}

	function clearSelection() {
		selectedVideoIds = [];
		showMetadataPanel = false;
		metadataMessage = '';
		resetBulkDrafts();
	}

	function mergeUpdatedGroups(updatedGroups) {
		const groupsByTitle = new Map(updatedGroups.map((group) => [group.title, group]));
		videos = videos.map((video) => groupsByTitle.get(video.title) || video);
	}

	function handleSingleTagsChange(nextValues) {
		singleTagsDraft = nextValues;
		metadataMessage = '';
	}

	function handleSingleActorsChange(nextValues) {
		singleActorsDraft = nextValues;
		metadataMessage = '';
	}

	function handleBulkAddTagsChange(nextValues) {
		bulkAddTags = nextValues;
		metadataMessage = '';
	}

	function handleBulkRemoveTagsChange(nextValues) {
		bulkRemoveTags = nextValues;
		metadataMessage = '';
	}

	function handleBulkAddActorsChange(nextValues) {
		bulkAddActors = nextValues;
		metadataMessage = '';
	}

	function handleBulkRemoveActorsChange(nextValues) {
		bulkRemoveActors = nextValues;
		metadataMessage = '';
	}

	async function saveSingleMetadata() {
		if (!singleSelectedVideo || savingMetadata) {
			return;
		}

		savingMetadata = true;
		metadataMessage = '';

		try {
			const res = await authFetch(`/api/videos/${singleSelectedVideo.id}/metadata`, {
				method: 'PUT',
				body: JSON.stringify({
					tags: singleTagsDraft,
					actors: singleActorsDraft
				})
			});
			if (!res.ok) {
				throw new Error(await readError(res, 'Failed to save video metadata'));
			}

			const updatedVideo = await res.json();
			mergeUpdatedGroups([updatedVideo]);
			toast.success('Video details updated');
			try {
				await loadMetadataOptions();
			} catch {
				toast.warning('Video updated but suggestions could not be refreshed');
			}
		} catch (err) {
			metadataMessage = err.message;
			toast.error(err.message);
		} finally {
			savingMetadata = false;
		}
	}

	async function addToPlaylist(playlistId) {
		if (savingPlaylist || selectedCount === 0) return;
		savingPlaylist = true;
		playlistMessage = '';
		try {
			const existingPlaylist = playlists.find((playlist) => playlist.id === playlistId);
			const previousCount = existingPlaylist?.item_count || 0;
			const res = await authFetch(`/api/playlists/${playlistId}/items`, {
				method: 'POST',
				body: JSON.stringify({ video_ids: selectedVideoIds })
			});
			if (!res.ok) throw new Error(await readError(res, 'Failed to add to playlist'));
			const playlist = await res.json();
			const addedCount = Math.max((playlist?.items?.length || 0) - previousCount, 0);
			if (addedCount > 0) {
				toast.success(`Added ${addedCount} ${addedCount === 1 ? 'item' : 'items'} to playlist`);
			} else {
				toast.info('All selected items were already in that playlist');
			}
			await loadPlaylists();
			setTimeout(() => {
				showPlaylistPanel = false;
				clearSelection();
			}, 1500);
		} catch (err) {
			playlistMessage = err.message;
			toast.error(err.message);
		} finally {
			savingPlaylist = false;
		}
	}

	async function createPlaylist() {
		if (savingPlaylist || selectedCount === 0 || !newPlaylistName.trim()) return;
		savingPlaylist = true;
		playlistMessage = '';
		try {
			const res = await authFetch('/api/playlists', {
				method: 'POST',
				body: JSON.stringify({
					name: newPlaylistName,
					description: '',
					video_ids: selectedVideoIds
				})
			});
			if (!res.ok) throw new Error(await readError(res, 'Failed to create playlist'));
			const playlist = await res.json();
			toast.success(`Playlist "${playlist.name}" created with ${playlist?.items?.length || 0} ${(playlist?.items?.length || 0) === 1 ? 'item' : 'items'}`);
			await loadPlaylists();
			newPlaylistName = '';
			setTimeout(() => {
				showPlaylistPanel = false;
				clearSelection();
			}, 1500);
		} catch (err) {
			playlistMessage = err.message;
			toast.error(err.message);
		} finally {
			savingPlaylist = false;
		}
	}

	async function saveBulkMetadata() {
		if (!selectedCount || savingMetadata || !bulkHasChanges) {
			return;
		}

		savingMetadata = true;
		metadataMessage = '';

		try {
			const res = await authFetch('/api/videos/metadata/bulk', {
				method: 'POST',
				body: JSON.stringify({
					ids: selectedVideoIds,
					add_tags: bulkAddTags,
					remove_tags: bulkRemoveTags,
					add_actors: bulkAddActors,
					remove_actors: bulkRemoveActors
				})
			});
			if (!res.ok) {
				throw new Error(await readError(res, 'Failed to update selected videos'));
			}

			const data = await res.json();
			mergeUpdatedGroups(data.updated_groups || []);
			resetBulkDrafts();
			toast.success(`Updated ${data.updated_count || selectedCount} selected video groups`);
			try {
				await loadMetadataOptions();
			} catch {
				toast.warning('Updated videos but suggestions could not be refreshed');
			}
		} catch (err) {
			metadataMessage = err.message;
			toast.error(err.message);
		} finally {
			savingMetadata = false;
		}
	}

	$effect(() => {
		const pageCount = totalPages;
		if (currentPage > pageCount) {
			currentPage = pageCount;
		}
	});

	$effect(() => {
		if (selectedCount === 0) {
			showMetadataPanel = false;
			metadataMessage = '';
			resetBulkDrafts();
			singleTagsDraft = [];
			singleActorsDraft = [];
			return;
		}

		if (selectedCount > 1) {
			singleTagsDraft = [];
			singleActorsDraft = [];
		}
	});

	$effect(() => {
		const video = singleSelectedVideo;
		if (!video) {
			return;
		}
		singleTagsDraft = [...(video.tags || [])];
		singleActorsDraft = [...(video.actors || [])];
		metadataMessage = '';
	});

	$effect(() => {
		if (showMetadataPanel || showPlaylistPanel) {
			document.body.style.overflow = 'hidden';
			queueMicrotask(() => {
				if (showMetadataPanel) metadataPanelOverlayEl?.focus();
				if (showPlaylistPanel) playlistPanelOverlayEl?.focus();
			});
		} else {
			document.body.style.overflow = '';
		}
		return () => {
			document.body.style.overflow = '';
		};
	});

	function toggleSelectionMode() {
		selectionMode = !selectionMode;
		if (!selectionMode) {
			clearSelection();
		}
	}

	onMount(() => {
		document.addEventListener('click', handleClickOutside);

		(async () => {
			loadMetadataOptions().catch((err) => {
				console.error('Failed to load metadata options:', err);
			});
			loadPlaylists().catch((err) => {
				console.error('Failed to load playlists:', err);
			});
			loadContinueWatching();

			try {
				await loadVideos();
			} catch (err) {
				error = err.message;
			} finally {
				loading = false;
			}
		})();

		return () => {
			document.removeEventListener('click', handleClickOutside);
		};
	});
</script>

<svelte:head>
	<title>Collectarr - Library</title>
</svelte:head>

<div class="px-4 sm:px-8 py-8 lg:py-12 max-w-[1600px] mx-auto w-full">
	<div class="mb-10 flex flex-col gap-6">
		<div class="w-full relative">
			<label
				class="flex h-14 w-full items-center overflow-hidden rounded-xl border border-neutral-800 bg-neutral-950/80 text-white transition-all focus-within:border-neutral-500 focus-within:bg-neutral-900 focus-within:ring-4 focus-within:ring-neutral-800/50 shadow-sm"
			>
				<div class="pl-5 pr-4 text-neutral-500 flex items-center justify-center shrink-0">
					<svg class="w-6 h-6" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
						<circle cx="11" cy="11" r="8"></circle>
						<line x1="21" y1="21" x2="16.65" y2="16.65"></line>
					</svg>
				</div>
				<input
					type="search"
					value={searchQuery}
					oninput={handleSearchInput}
					placeholder="Search title, date, tags, actors..."
					class="w-full h-full bg-transparent px-2 text-lg text-white placeholder:text-neutral-600 outline-none"
					aria-label="Search videos"
				/>
			</label>
		</div>

		<div class="flex flex-wrap items-center justify-between gap-4">
			<div class="flex items-center gap-3">
				<button
					class="h-10 w-10 rounded-lg flex items-center justify-center border border-neutral-800 hover:border-neutral-600 transition-colors text-neutral-400 hover:text-white bg-neutral-900/50"
					aria-label="Refresh library"
					onclick={() => loadVideos()}
					disabled={loading}
				>
					<svg
						class="w-5 h-5"
						class:animate-spin-full={isSpinning}
						viewBox="0 0 24 24"
						fill="currentColor"
					>
						<path
							d="M17.65 6.35C16.2 4.9 14.21 4 12 4c-4.42 0-7.99 3.58-7.99 8s3.57 8 7.99 8c3.73 0 6.84-2.55 7.73-6h-2.08c-.82 2.33-3.04 4-5.65 4-3.31 0-6-2.69-6-6s2.69-6 6-6c1.66 0 3.14.69 4.22 1.78L13 11h7V4l-2.35 2.35z"
						/>
					</svg>
				</button>
				
				<div class="relative" bind:this={desktopSortDropdownEl}>
					<button
						class="flex items-center justify-center gap-2 h-10 px-4 rounded-lg text-sm font-medium tracking-wide border border-neutral-800 hover:border-neutral-600 transition-colors text-neutral-300 hover:text-white bg-neutral-900/50"
						onclick={() => (showSortDropdown = !showSortDropdown)}
						aria-label="Sort options"
					>
						<span>Sort: {getSortLabel()}</span>
						<span
							class="p-0.5 rounded hover:bg-neutral-700 transition-colors"
							onclick={(event) => {
								event.stopPropagation();
								toggleSortOrder();
							}}
							role="button"
							tabindex="0"
							onkeydown={(event) => {
								if (event.key === 'Enter' || event.key === ' ') {
									event.stopPropagation();
									toggleSortOrder();
								}
							}}
							aria-label="Toggle sort order"
							title={sortOrder === 'asc' ? 'Ascending' : 'Descending'}
						>
							<svg class="w-4 h-4" viewBox="0 0 24 24" fill="currentColor">
								{#if sortOrder === 'asc'}
									<path d="M4 12l1.41 1.41L11 7.83V20h2V7.83l5.58 5.59L20 12l-8-8-8 8z" />
								{:else}
									<path d="M20 12l-1.41-1.41L13 16.17V4h-2v12.17l-5.58-5.59L4 12l8 8 8-8z" />
								{/if}
							</svg>
						</span>
					</button>
					{#if showSortDropdown}
						<div
							class="absolute top-full left-0 mt-2 w-48 rounded-xl bg-neutral-900 border border-neutral-700 shadow-2xl z-30 overflow-hidden"
						>
							<button
								class="w-full px-4 py-3 text-sm text-left hover:bg-neutral-800 transition-colors {sortBy ===
								'dateAdded'
									? 'text-white font-medium bg-neutral-800/50'
									: 'text-neutral-300'}"
								onclick={() => setSort('dateAdded')}>Date added</button
							>
							<button
								class="w-full px-4 py-3 text-sm text-left hover:bg-neutral-800 transition-colors {sortBy ===
								'duration'
									? 'text-white font-medium bg-neutral-800/50'
									: 'text-neutral-300'}"
								onclick={() => setSort('duration')}>Duration</button
							>
							<button
								class="w-full px-4 py-3 text-sm text-left hover:bg-neutral-800 transition-colors {sortBy ===
								'alphabetical'
									? 'text-white font-medium bg-neutral-800/50'
									: 'text-neutral-300'}"
								onclick={() => setSort('alphabetical')}>Alphabetical</button
							>
						</div>
					{/if}
				</div>
			</div>

			<div class="flex items-center gap-3">
				<div class="hidden sm:block relative" bind:this={columnDropdownEl}>
					<button
						class="flex items-center gap-2 h-10 px-4 rounded-lg text-sm font-medium tracking-wide border border-neutral-800 hover:border-neutral-600 transition-colors text-neutral-300 hover:text-white bg-neutral-900/50"
						onclick={() => (showColumnDropdown = !showColumnDropdown)}
						aria-label="Column count options"
					>
						<span>Grid: {columnCount}</span>
						<svg class="w-4 h-4 opacity-70" viewBox="0 0 24 24" fill="currentColor">
							<path d="M7 10l5 5 5-5z" />
						</svg>
					</button>
					{#if showColumnDropdown}
						<div
							class="absolute top-full right-0 mt-2 w-40 rounded-xl bg-neutral-900 border border-neutral-700 shadow-2xl z-30 overflow-hidden"
						>
							{#each [2, 3, 4] as count}
								<button
									class="w-full px-4 py-3 text-sm text-left hover:bg-neutral-800 transition-colors {columnCount ===
									count
										? 'text-white font-medium bg-neutral-800/50'
										: 'text-neutral-300'}"
									onclick={() => setColumnCount(count)}>{count} columns</button
								>
							{/each}
						</div>
					{/if}
				</div>

				<button
					class="h-10 px-5 rounded-lg text-sm font-medium tracking-wide border transition-colors {selectionMode
						? 'border-white bg-white text-black hover:bg-neutral-200'
						: 'border-neutral-800 bg-neutral-900/50 text-neutral-300 hover:text-white hover:border-neutral-600'}"
					onclick={toggleSelectionMode}
					aria-label={selectionMode ? 'Exit selection mode' : 'Enter selection mode'}
				>
					{selectionMode ? 'Done' : 'Select'}
				</button>
			</div>
		</div>
	</div>

	{#if selectionMode && !loading && videos.length > 0}
		<div class="mb-6 border border-neutral-800 bg-black/70 px-4 py-4">
			<div class="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
				<div>
					<p class="text-[10px] uppercase tracking-[0.3em] text-neutral-500">Selection</p>
					<p class="mt-2 text-sm text-white">
						{#if selectedCount > 0}
							{selectedCount} selected {selectedCount === 1 ? 'video group' : 'video groups'}
						{:else}
							Select videos directly from the grid to edit tags and actors here.
						{/if}
					</p>
				</div>

				<div class="flex flex-wrap items-center gap-2">
					{#if selectedCount > 0}
						<button
							class="h-9 px-3 text-xs uppercase tracking-wider border border-neutral-600 bg-neutral-900 text-white hover:border-neutral-400 transition-colors"
							onclick={() => (showPlaylistPanel = true)}
						>
							Add to Playlist
						</button>
						<button
							class="h-9 px-3 text-xs uppercase tracking-wider border border-neutral-600 bg-neutral-900 text-white hover:border-neutral-400 transition-colors"
							onclick={() => (showMetadataPanel = true)}
						>
							Edit selection
						</button>
						<button
							class="h-9 px-3 text-xs uppercase tracking-wider border border-neutral-600 text-white hover:border-neutral-400 hover:bg-neutral-800 transition-colors"
							onclick={clearSelection}
						>
							Clear
						</button>
					{/if}
				</div>
			</div>
		</div>
	{/if}

	{#if !loading && continueWatching.length > 0}
		<div class="mb-8">
			<div class="flex items-center justify-between mb-4">
				<h2 class="text-lg font-bold uppercase tracking-widest text-white">Continue Watching</h2>
				<a href="/player/{continueWatching[0].video_id}" class="text-xs uppercase tracking-wider text-neutral-400 hover:text-white transition-colors">
					Resume Latest →
				</a>
			</div>
			<div class="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 xl:grid-cols-8 gap-4">
				{#each continueWatching as item (item.video_id)}
					<a href="/player/{item.video_id}" class="group block">
						<div class="relative aspect-video bg-neutral-900 border border-neutral-800 overflow-hidden">
							{#if item.thumbnail_url && !failedContinueWatchingThumbnails[item.video_id]}
								<img src={item.thumbnail_url} alt={item.title} class="w-full h-full object-cover opacity-80 group-hover:opacity-100 transition-opacity" onerror={() => {
									failedContinueWatchingThumbnails = {
										...failedContinueWatchingThumbnails,
										[item.video_id]: true
									};
								}} />
							{:else}
								<div class="w-full h-full flex items-center justify-center">
									<svg class="w-8 h-8 text-neutral-600" viewBox="0 0 24 24" fill="currentColor">
										<path d="M8 5v14l11-7z"/>
									</svg>
								</div>
							{/if}
							<div class="absolute bottom-0 left-0 right-0 h-1 bg-neutral-800">
								<div class="h-full bg-white" style="width: {item.progress_pct}%"></div>
							</div>
							<div class="absolute inset-0 flex items-center justify-center opacity-0 group-hover:opacity-100 transition-opacity">
								<div class="w-12 h-12 rounded-full bg-white/90 flex items-center justify-center">
									<svg class="w-6 h-6 text-black ml-1" viewBox="0 0 24 24" fill="currentColor">
										<path d="M8 5v14l11-7z"/>
									</svg>
								</div>
							</div>
						</div>
						<div class="mt-2">
							<p class="text-sm text-white truncate">{item.title}</p>
							<p class="text-xs text-neutral-500">{Math.round(item.progress_pct)}% watched</p>
						</div>
					</a>
				{/each}
			</div>
		</div>
	{/if}

	{#if loading}
		<div class="flex flex-col items-center justify-center min-h-[50vh] space-y-4">
			<span class="loading loading-spinner loading-lg text-white"></span>
			<p class="text-neutral-500 uppercase tracking-widest text-sm">Loading...</p>
		</div>
	{:else if error && videos.length === 0}
		<div class="flex flex-col items-center justify-center min-h-[50vh] space-y-4">
			<div class="border border-neutral-800 bg-neutral-950/50 p-12 flex flex-col items-center text-center max-w-md">
				<div class="w-16 h-16 mb-4 text-neutral-700">
					<svg viewBox="0 0 24 24" fill="currentColor">
						<path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm1 15h-2v-2h2v2zm0-4h-2V7h2v6z"/>
					</svg>
				</div>
				<p class="text-neutral-400 uppercase tracking-widest mb-4">Error Loading Library</p>
				<p class="text-sm text-neutral-600 mb-6">{error}</p>
				<button onclick={() => window.location.reload()} class="border border-neutral-700 px-4 py-2 text-xs uppercase tracking-widest text-neutral-400 hover:text-white hover:border-neutral-500 transition-colors">
					Retry
				</button>
			</div>
		</div>
	{:else if videos.length === 0}
		<div class="flex flex-col items-center justify-center min-h-[50vh] space-y-4">
			<div class="border border-neutral-800 bg-neutral-950/50 p-12 flex flex-col items-center text-center max-w-md">
				<div class="w-16 h-16 mb-4 text-neutral-700">
					<svg viewBox="0 0 24 24" fill="currentColor">
						<path d="M9.75 15.5a.75.75 0 0 1-.75-.75v-4.5a.75.75 0 0 1 1.13-.65l4.5 2.25a.75.75 0 0 1 0 1.34l-4.5 2.25a.75.75 0 0 1-.38.1Z"/>
						<path d="M2 6.5a4 4 0 0 1 4-4h12a4 4 0 0 1 4 4v11a4 4 0 0 1-4 4H6a4 4 0 0 1-4-4v-11Zm4-2.5a2.5 2.5 0 0 0-2.5 2.5v11a2.5 2.5 0 0 0 2.5 2.5h12a2.5 2.5 0 0 0 2.5-2.5v-11a2.5 2.5 0 0 0-2.5-2.5H6Z"/>
					</svg>
				</div>
				<p class="text-neutral-400 uppercase tracking-widest mb-4">Welcome to Collectarr</p>
				<p class="text-sm text-neutral-600 mb-2">
					Your video library is empty.
				</p>
				<p class="text-sm text-neutral-600 mb-6">
					Add your media path in settings and run a scan to get started.
				</p>
				<a
					href="/settings"
					class="border border-neutral-700 px-4 py-2 text-xs uppercase tracking-widest text-neutral-400 hover:text-white hover:border-neutral-500 transition-colors"
					>Go to Settings</a
				>
			</div>
		</div>
	{:else if filteredVideos.length === 0}
		<div class="flex flex-col items-center justify-center min-h-[50vh] space-y-4">
			<div class="border border-neutral-800 bg-neutral-950/50 p-12 flex flex-col items-center text-center max-w-md">
				<div class="w-16 h-16 mb-4 text-neutral-700">
					<svg viewBox="0 0 24 24" fill="currentColor">
						<path d="M15.5 14h-.79l-.28-.27a6.5 6.5 0 0 0 1.48-5.34c-.47-2.78-2.79-5-5.59-5.34a6.505 6.505 0 0 0-7.27 7.27c.34 2.8 2.56 5.12 5.34 5.59a6.5 6.5 0 0 0 5.34-1.48l.27.28v.79l4.25 4.25c.41.41 1.08.41 1.49 0 .41-.41.41-1.08 0-1.49L15.5 14zm-6 0C7.01 14 5 11.99 5 9.5S7.01 5 9.5 5 14 7.01 14 9.5 11.99 14 9.5 14z"/>
					</svg>
				</div>
				<p class="text-neutral-400 uppercase tracking-widest mb-4">No Matching Videos</p>
				<p class="text-sm text-neutral-600 mb-6">
					No videos found matching "{searchQuery}".
				</p>
				<button onclick={() => { searchQuery = ''; currentPage = 1; }} class="border border-neutral-700 px-4 py-2 text-xs uppercase tracking-widest text-neutral-400 hover:text-white hover:border-neutral-500 transition-colors">
					Clear Search
				</button>
			</div>
		</div>
	{:else}
		<div
			class="text-xs text-neutral-500 uppercase tracking-wider mb-4 flex flex-wrap items-center gap-2"
		>
			<span
				>Showing {(currentPage - 1) * itemsPerPage + 1} - {Math.min(
					currentPage * itemsPerPage,
					filteredVideos.length
				)} of {filteredVideos.length} videos</span
			>
		</div>

		<div class="grid {getColumnClass(columnCount)} gap-6">
			{#each paginatedVideos as video (video.id)}
				<VideoCard
					{video}
					selectable={selectionMode}
					selected={selectedVideoIds.includes(video.id)}
					onToggleSelect={toggleVideoSelection}
				/>
			{/each}
		</div>

		{#if totalPages > 1}
			<div class="flex justify-center items-center gap-2 mt-8">
				<button
					class="flex h-12 w-12 items-center justify-center border border-neutral-700 text-neutral-400 transition-colors hover:border-neutral-500 hover:text-white disabled:opacity-50 disabled:cursor-not-allowed"
					onclick={() => goToPage(currentPage - 1)}
					disabled={currentPage === 1}
					aria-label="Previous page"
				>
					<svg class="w-5 h-5" viewBox="0 0 24 24" fill="currentColor"
						><path d="M15.41 7.41L14 6l-6 6 6 6 1.41-1.41L10.83 12z" /></svg
					>
				</button>

				{#each Array.from({ length: Math.min(5, totalPages) }, (_, i) => {
					let start = Math.max(1, Math.min(currentPage - 2, totalPages - 4));
					return start + i;
				}) as pageNum}
					<button
						class="flex h-12 w-12 items-center justify-center border text-xs font-mono transition-colors {currentPage ===
						pageNum
							? 'bg-neutral-700 text-white border-neutral-600'
							: 'text-neutral-400 border-neutral-700 hover:border-neutral-500 hover:text-white'}"
						onclick={() => goToPage(pageNum)}
						aria-label="Page {pageNum}">{pageNum}</button
					>
				{/each}

				{#if totalPages > 5 && currentPage < totalPages - 2}
					<span class="text-neutral-500 px-1">...</span>
				{/if}

				{#if totalPages > 5 && currentPage < totalPages - 2}
					<button
						class="flex h-12 min-w-12 items-center justify-center border border-neutral-700 px-3 text-xs font-mono text-neutral-400 transition-colors hover:border-neutral-500 hover:text-white"
						onclick={() => goToPage(totalPages)}
						aria-label="Last page">{totalPages}</button
					>
				{/if}

				<button
					class="flex h-12 w-12 items-center justify-center border border-neutral-700 text-neutral-400 transition-colors hover:border-neutral-500 hover:text-white disabled:opacity-50 disabled:cursor-not-allowed"
					onclick={() => goToPage(currentPage + 1)}
					disabled={currentPage === totalPages}
					aria-label="Next page"
				>
					<svg class="w-5 h-5" viewBox="0 0 24 24" fill="currentColor"
						><path d="M10 6L8.59 7.41 13.17 12l-4.58 4.59L10 18l6-6z" /></svg
					>
				</button>
			</div>
		{/if}
	{/if}
</div>

{#if selectionMode && selectedCount > 0 && showMetadataPanel}
	<div
		class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/80 backdrop-blur-sm"
		bind:this={metadataPanelOverlayEl}
		role="button"
		tabindex="0"
		aria-label="Close metadata editor"
		onclick={(e) => {
			if (e.target === e.currentTarget) showMetadataPanel = false;
		}}
		onkeydown={(e) => {
			if (e.target !== e.currentTarget) return;
			if (e.key === 'Enter' || e.key === ' ' || e.key === 'Escape') {
				e.preventDefault();
				showMetadataPanel = false;
			}
		}}
	>
		<div
			class="w-full max-w-2xl max-h-[90vh] overflow-hidden border border-neutral-800 bg-black shadow-2xl"
		>
			<div class="flex items-start justify-between gap-4 border-b border-neutral-800 px-6 py-5">
				<div>
					<p class="text-[10px] uppercase tracking-[0.3em] text-neutral-500">Metadata Editor</p>
					<h2 class="mt-2 text-lg font-semibold text-white">
						{#if selectedCount === 1}
							{singleSelectedVideo?.title || 'Selected video'}
						{:else}
							{selectedCount} selected video groups
						{/if}
					</h2>
				</div>
				<button
					class="mt-0.5 text-neutral-400 hover:text-white transition-colors"
					aria-label="Close metadata editor"
					onclick={() => (showMetadataPanel = false)}
				>
					<svg class="w-6 h-6" viewBox="0 0 24 24" fill="currentColor"
						><path
							d="M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z"
						/></svg
					>
				</button>
			</div>

			<div class="overflow-y-auto px-6 py-5 text-sm text-white/80 max-h-[calc(90vh-140px)]">
				{#if selectedCount === 1}
					<div class="space-y-6">
						<div class="grid gap-4 border border-neutral-800 bg-neutral-950/60 p-4 text-sm">
							<div>
								<p class="text-[10px] uppercase tracking-[0.3em] text-neutral-500">Selection</p>
								<p class="mt-2 text-white">
									Exact metadata editing for this video group. Changes apply to every quality
									variant with the same title.
								</p>
							</div>
						</div>

						<MetadataTokenInput
							label="Tags"
							values={singleTagsDraft}
							suggestions={metadataOptions.tags}
							placeholder="Type to add or select tags"
							helpText="Type to filter existing tags, then press Enter, Tab, or comma to add one."
							disabled={savingMetadata}
							onChange={handleSingleTagsChange}
						/>

						<MetadataTokenInput
							label="Actors / Actresses"
							values={singleActorsDraft}
							suggestions={metadataOptions.actors}
							placeholder="Type to add or select actors"
							helpText="Pick existing names from the list or add new ones as chips."
							disabled={savingMetadata}
							onChange={handleSingleActorsChange}
						/>
					</div>
				{:else}
					<div class="space-y-6">
						<div class="border border-neutral-800 bg-neutral-950/60 p-4">
							<p class="text-[10px] uppercase tracking-[0.3em] text-neutral-500">Bulk editing</p>
							<p class="mt-2 text-white">
								Bulk updates are non-destructive: add tags and actors to every selected group, or
								remove specific ones across the whole selection.
							</p>
						</div>

						<div class="grid gap-6 lg:grid-cols-2">
							<MetadataTokenInput
								label="Add Tags"
								values={bulkAddTags}
								suggestions={metadataOptions.tags}
								excludeValues={bulkRemoveTags}
								placeholder="Type tags to add"
								helpText="Every selected group will gain these tags."
								disabled={savingMetadata}
								onChange={handleBulkAddTagsChange}
							/>

							<MetadataTokenInput
								label="Remove Tags"
								values={bulkRemoveTags}
								suggestions={metadataOptions.tags}
								excludeValues={bulkAddTags}
								placeholder="Type tags to remove"
								helpText="Matching tags will be removed from every selected group."
								disabled={savingMetadata}
								onChange={handleBulkRemoveTagsChange}
							/>

							<MetadataTokenInput
								label="Add Actors / Actresses"
								values={bulkAddActors}
								suggestions={metadataOptions.actors}
								excludeValues={bulkRemoveActors}
								placeholder="Type actors to add"
								helpText="Every selected group will gain these names."
								disabled={savingMetadata}
								onChange={handleBulkAddActorsChange}
							/>

							<MetadataTokenInput
								label="Remove Actors / Actresses"
								values={bulkRemoveActors}
								suggestions={metadataOptions.actors}
								excludeValues={bulkAddActors}
								placeholder="Type actors to remove"
								helpText="Matching names will be removed from every selected group."
								disabled={savingMetadata}
								onChange={handleBulkRemoveActorsChange}
							/>
						</div>
					</div>
				{/if}
			</div>

			<div class="border-t border-neutral-800 px-6 py-5">
				<div class="flex items-center gap-3">
					<button
						class="border border-white bg-white px-4 py-2 text-xs font-bold uppercase tracking-[0.25em] text-black transition-colors hover:bg-white/85 disabled:border-neutral-700 disabled:bg-neutral-800 disabled:text-neutral-500"
						onclick={selectedCount === 1 ? saveSingleMetadata : saveBulkMetadata}
						disabled={savingMetadata || (selectedCount > 1 && !bulkHasChanges)}
					>
						{#if savingMetadata}
							Saving...
						{:else if selectedCount === 1}
							Save Details
						{:else}
							Apply Bulk Update
						{/if}
					</button>
					<button
						class="border border-neutral-700 px-4 py-2 text-xs font-bold uppercase tracking-[0.25em] text-neutral-300 transition-colors hover:border-neutral-500 hover:text-white"
						onclick={() => (showMetadataPanel = false)}
					>
						Cancel
					</button>
				</div>
				{#if metadataMessage}
					<p class="mt-3 text-xs text-neutral-400">{metadataMessage}</p>
				{/if}
			</div>
		</div>
	</div>
{/if}

{#if selectionMode && selectedCount > 0 && showPlaylistPanel}
	<div
		class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/80 backdrop-blur-sm"
		bind:this={playlistPanelOverlayEl}
		role="button"
		tabindex="0"
		aria-label="Close playlist panel"
		onclick={(e) => {
			if (e.target === e.currentTarget) showPlaylistPanel = false;
		}}
		onkeydown={(e) => {
			if (e.target !== e.currentTarget) return;
			if (e.key === 'Enter' || e.key === ' ' || e.key === 'Escape') {
				e.preventDefault();
				showPlaylistPanel = false;
			}
		}}
	>
		<div
			class="w-full max-w-lg overflow-hidden border border-neutral-800 bg-black shadow-2xl flex flex-col max-h-[90vh]"
		>
			<div
				class="flex items-start justify-between gap-4 border-b border-neutral-800 px-6 py-5 shrink-0"
			>
				<div>
					<p class="text-[10px] uppercase tracking-[0.3em] text-neutral-500">Playlists</p>
					<h2 class="mt-2 text-lg font-semibold text-white">
						Add {selectedCount}
						{selectedCount === 1 ? 'video' : 'videos'} to playlist
					</h2>
				</div>
				<button
					class="mt-0.5 text-neutral-400 hover:text-white transition-colors"
					aria-label="Close playlist panel"
					onclick={() => (showPlaylistPanel = false)}
				>
					<svg class="w-6 h-6" viewBox="0 0 24 24" fill="currentColor"
						><path
							d="M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z"
						/></svg
					>
				</button>
			</div>

			<div class="px-6 py-5 space-y-6 overflow-y-auto min-h-0 flex-1">
				<div>
					<h3
						class="text-sm uppercase tracking-widest text-neutral-400 mb-3 border-b border-neutral-800 pb-2"
					>
						Create New
					</h3>
					<form
						class="flex gap-2"
						onsubmit={(e) => {
							e.preventDefault();
							createPlaylist();
						}}
					>
						<input
							type="text"
							bind:value={newPlaylistName}
							placeholder="Playlist name"
							class="h-10 flex-1 bg-neutral-900 border border-neutral-700 px-3 text-white outline-none focus:border-neutral-500 text-sm"
							disabled={savingPlaylist}
						/>
						<button
							type="submit"
							class="h-10 px-4 text-xs font-bold uppercase tracking-widest bg-white text-black hover:bg-white/80 transition-colors disabled:opacity-50"
							disabled={savingPlaylist || !newPlaylistName.trim()}
						>
							Create
						</button>
					</form>
				</div>

				{#if playlists.length > 0}
					<div>
						<h3
							class="text-sm uppercase tracking-widest text-neutral-400 mb-3 border-b border-neutral-800 pb-2"
						>
							Add to Existing
						</h3>
						<div class="flex flex-col gap-2 max-h-60 overflow-y-auto pr-2">
							{#each playlists as playlist (playlist.id)}
								<button
									class="flex items-center justify-between p-3 border border-neutral-800 hover:border-neutral-500 bg-neutral-950/60 hover:bg-neutral-900 transition-colors text-left"
									onclick={() => addToPlaylist(playlist.id)}
									disabled={savingPlaylist}
								>
									<span class="text-sm text-white font-medium truncate pr-4">{playlist.name}</span>
									<span class="text-xs text-neutral-500 shrink-0">{playlist.item_count} items</span>
								</button>
							{/each}
						</div>
					</div>
				{/if}

				{#if playlistMessage}
					<p class="text-sm text-neutral-400 p-3 bg-neutral-900 border border-neutral-800">
						{playlistMessage}
					</p>
				{/if}
			</div>

			<div class="border-t border-neutral-800 px-6 py-5 shrink-0">
				<button
					class="w-full h-10 border border-neutral-700 text-xs font-bold uppercase tracking-[0.25em] text-neutral-300 transition-colors hover:border-neutral-500 hover:text-white"
					onclick={() => (showPlaylistPanel = false)}
				>
					Close
				</button>
			</div>
		</div>
	</div>
{/if}
