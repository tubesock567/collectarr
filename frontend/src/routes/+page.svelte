<script>
	import { onMount } from 'svelte';
	import { authFetch } from '$lib/auth';
	import MetadataTokenInput from '$lib/components/MetadataTokenInput.svelte';
	import VideoCard from '$lib/components/VideoCard.svelte';
	import { preferences } from '$lib/preferences';
	import { theme } from '$lib/theme';

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
	let sortDropdownEl = $state(null);
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
	const allPageSelected = $derived(paginatedVideos.length > 0 && paginatedVideos.every((video) => selectedVideoIds.includes(video.id)));
	const allFilteredSelected = $derived(filteredVideos.length > 0 && filteredVideos.every((video) => selectedVideoIds.includes(video.id)));
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
			window.scrollTo({ top: 0, behavior: 'smooth' });
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
		if (sortDropdownEl && !sortDropdownEl.contains(event.target)) {
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

	async function readError(res, fallback) {
		const data = await readJSONSafe(res);
		return data?.error || fallback;
	}

	async function loadVideos() {
		const res = await authFetch('/api/videos');
		if (!res.ok) {
			throw new Error(await readError(res, 'Failed to fetch videos'));
		}
		videos = await res.json();
	}

	async function loadMetadataOptions() {
		const res = await authFetch('/api/videos/metadata/options');
		if (!res.ok) {
			throw new Error(await readError(res, 'Failed to fetch metadata options'));
		}
		metadataOptions = await res.json();
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
			showMetadataPanel = true;
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

	function selectCurrentPage() {
		addSelection(paginatedVideos.map((video) => video.id));
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

	function selectFilteredVideos() {
		addSelection(filteredVideos.map((video) => video.id));
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

	function toggleSelectionMode() {
		selectionMode = !selectionMode;
		if (!selectionMode) {
			clearSelection();
		}
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
			metadataMessage = 'Video details updated.';
			try {
				await loadMetadataOptions();
			} catch {
				metadataMessage = 'Video details updated. Suggestions could not be refreshed.';
			}
		} catch (err) {
			metadataMessage = err.message;
		} finally {
			savingMetadata = false;
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
			metadataMessage = `Updated ${data.updated_count || selectedCount} selected video groups.`;
			try {
				await loadMetadataOptions();
			} catch {
				metadataMessage = `Updated ${data.updated_count || selectedCount} selected video groups. Suggestions could not be refreshed.`;
			}
		} catch (err) {
			metadataMessage = err.message;
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
		if (!selectionMode && selectedVideoIds.length > 0) {
			clearSelection();
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

	onMount(() => {
		document.addEventListener('click', handleClickOutside);

		(async () => {
			try {
				await Promise.all([loadVideos(), loadMetadataOptions()]);
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

<div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8 {selectedCount > 0 && showMetadataPanel ? 'xl:pr-[28rem]' : ''}">
	<div class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3 mb-6">
		<div class="flex items-center gap-3">
			<button
				class="h-9 w-9 flex items-center justify-center rounded-none border border-neutral-600 hover:border-neutral-400 transition-colors text-neutral-300 hover:text-white bg-neutral-900"
				aria-label={$theme === 'light' ? 'Switch to dark mode' : 'Switch to light mode'}
				onclick={() => theme.toggleTheme($theme)}
			>
				{#if $theme === 'light'}
					<svg class="w-5 h-5" viewBox="0 0 24 24" fill="currentColor">
						<path d="M9 2c-1.05 0-2.05.16-3 .46 1.69 1.23 2.8 3.24 2.8 5.54 0 3.87-3.13 7-7 7-1.11 0-2.16-.26-3.09-.72C.56 16.2 3.5 19 7 19c4.97 0 9-4.03 9-9 0-4.97-4.03-9-9-9z"/>
					</svg>
				{:else}
					<svg class="w-5 h-5" viewBox="0 0 24 24" fill="currentColor">
						<path d="M6.76 4.84l-1.8-1.79-1.41 1.41 1.79 1.79 1.42-1.41zM4 10.5H1v2h3v-2zm9-9.95h-2V3.5h2V.55zm7.45 3.91l-1.41-1.41-1.79 1.79 1.41 1.41 1.79-1.79zm-3.21 13.7l1.79 1.8 1.41-1.41-1.8-1.79-1.4 1.4zM20 10.5v2h3v-2h-3zm-8-5c-3.31 0-6 2.69-6 6s2.69 6 6 6 6-2.69 6-6-2.69-6-6-6zm-1 16.95h2V22h-2v5.05zm-7.45-3.91l1.41 1.41 1.79-1.8-1.41-1.41-1.79 1.8z"/>
					</svg>
				{/if}
			</button>
			<button
				class="h-9 w-9 flex items-center justify-center rounded-none border border-neutral-600 hover:border-neutral-400 transition-colors text-neutral-300 hover:text-white bg-neutral-900"
				aria-label={$preferences.incognito ? 'Disable incognito mode' : 'Enable incognito mode'}
				onclick={() => preferences.toggleIncognito()}
			>
				{#if $preferences.incognito}
					<svg class="w-5 h-5" viewBox="0 0 24 24" fill="currentColor">
						<path d="M12 4.5C7 4.5 2.73 7.61 1 12c1.73 4.39 6 7.5 11 7.5s9.27-3.11 11-7.5c-1.73-4.39-6-7.5-11-7.5zM12 17c-2.76 0-5-2.24-5-5s2.24-5 5-5 5 2.24 5 5-2.24 5-5 5zm0-8c-1.66 0-3 1.34-3 3s1.34 3 3 3 3-1.34 3-3-1.34-3-3-3z"/>
					</svg>
				{:else}
					<svg class="w-5 h-5" viewBox="0 0 24 24" fill="currentColor">
						<path d="M12 7c2.76 0 5 2.24 5 5 0 .65-.13 1.26-.36 1.83l2.92 2.92c1.51-1.26 2.7-2.89 3.43-4.75-1.73-4.39-6-7.5-11-7.5-1.4 0-2.74.25-3.98.7l2.16 2.16C10.74 7.13 11.35 7 12 7zM2 4.27l2.28 2.28.46.46C3.08 8.3 1.78 10.02 1 12c1.73 4.39 6 7.5 11 7.5 1.55 0 3.03-.3 4.38-.84l.42.42L19.73 22 21 20.73 3.27 3 2 4.27zM7.53 9.8l1.55 1.55c-.05.21-.08.43-.08.65 0 1.66 1.34 3 3 3 .22 0 .44-.03.65-.08l1.55 1.55c-.67.33-1.41.53-2.2.53-2.76 0-5-2.24-5-5 0-.79.2-1.53.53-2.2zm4.31-.78l3.15 3.15.02-.16c0-1.66-1.34-3-3-3l-.17.01z"/>
					</svg>
				{/if}
			</button>
		</div>
		<div class="flex items-center gap-3 flex-wrap sm:justify-end">
			<button
				class="h-9 px-3 text-xs uppercase tracking-wider border rounded-none transition-colors {selectionMode ? 'border-white bg-white text-black' : 'border-neutral-600 bg-neutral-900 text-white hover:border-neutral-400'}"
				onclick={toggleSelectionMode}
				aria-label={selectionMode ? 'Exit selection mode' : 'Enter selection mode'}
			>
				{selectionMode ? 'Selection on' : 'Select'}
			</button>

			<label class="flex items-center h-9 border border-neutral-800 bg-black text-white rounded-none overflow-hidden focus-within:border-neutral-500 transition-colors">
				<span class="px-3 text-[10px] uppercase tracking-[0.25em] text-neutral-500 border-r border-neutral-800 h-full flex items-center shrink-0">Search</span>
				<input
					type="search"
					value={searchQuery}
					oninput={handleSearchInput}
					placeholder="Title, date, tags, actors"
					class="w-56 sm:w-64 md:w-72 h-full bg-black px-3 text-sm text-white placeholder:text-neutral-600 outline-none focus:border-neutral-500"
					aria-label="Search videos"
				/>
			</label>

			<div class="relative" bind:this={sortDropdownEl}>
				<button
					class="flex items-center gap-2 h-9 px-3 text-xs uppercase tracking-wider border border-neutral-600 rounded-none hover:border-neutral-400 transition-colors text-white bg-neutral-900"
					onclick={() => showSortDropdown = !showSortDropdown}
					aria-label="Sort options"
				>
					<span>Sort: {getSortLabel()}</span>
					<span
						class="p-0.5 hover:bg-neutral-700 rounded-none"
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
								<path d="M4 12l1.41 1.41L11 7.83V20h2V7.83l5.58 5.59L20 12l-8-8-8 8z"/>
							{:else}
								<path d="M20 12l-1.41-1.41L13 16.17V4h-2v12.17l-5.58-5.59L4 12l8 8 8-8z"/>
							{/if}
						</svg>
					</span>
					<svg class="w-4 h-4" viewBox="0 0 24 24" fill="currentColor">
						<path d="M7 10l5 5 5-5z"/>
					</svg>
				</button>
				{#if showSortDropdown}
					<div class="absolute top-full left-0 mt-1 w-40 bg-black border border-neutral-600 rounded-none shadow-xl z-20">
						<button class="w-full px-3 py-2 text-sm text-left hover:bg-neutral-900 transition-colors {sortBy === 'dateAdded' ? 'text-white bg-neutral-900' : 'text-neutral-300'}" onclick={() => setSort('dateAdded')}>Date added</button>
						<button class="w-full px-3 py-2 text-sm text-left hover:bg-neutral-900 transition-colors {sortBy === 'duration' ? 'text-white bg-neutral-900' : 'text-neutral-300'}" onclick={() => setSort('duration')}>Duration</button>
						<button class="w-full px-3 py-2 text-sm text-left hover:bg-neutral-900 transition-colors {sortBy === 'alphabetical' ? 'text-white bg-neutral-900' : 'text-neutral-300'}" onclick={() => setSort('alphabetical')}>Alphabetical</button>
					</div>
				{/if}
			</div>

			<div class="relative" bind:this={columnDropdownEl}>
				<button
					class="flex items-center gap-2 h-9 px-3 text-xs uppercase tracking-wider border border-neutral-600 rounded-none hover:border-neutral-400 transition-colors text-white bg-neutral-900"
					onclick={() => showColumnDropdown = !showColumnDropdown}
					aria-label="Column count options"
				>
					<span>Columns: {columnCount}</span>
					<svg class="w-4 h-4" viewBox="0 0 24 24" fill="currentColor">
						<path d="M7 10l5 5 5-5z"/>
					</svg>
				</button>
				{#if showColumnDropdown}
					<div class="absolute top-full left-0 mt-1 w-32 bg-black border border-neutral-600 rounded-none shadow-xl z-20">
						{#each [2, 3, 4] as count}
							<button class="w-full px-3 py-2 text-sm text-left hover:bg-neutral-900 transition-colors {columnCount === count ? 'text-white bg-neutral-900' : 'text-neutral-300'}" onclick={() => setColumnCount(count)}>{count} columns</button>
						{/each}
					</div>
				{/if}
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
					<button class="h-9 px-3 text-xs uppercase tracking-wider border rounded-none transition-colors {allPageSelected ? 'border-white bg-white text-black' : 'border-neutral-600 bg-neutral-900 text-white hover:border-neutral-400'}" onclick={toggleCurrentPageSelection}>
						{allPageSelected ? 'Deselect page' : 'Select page'}
					</button>
					<button class="h-9 px-3 text-xs uppercase tracking-wider border rounded-none transition-colors {allFilteredSelected ? 'border-white bg-white text-black' : 'border-neutral-600 bg-neutral-900 text-white hover:border-neutral-400'}" onclick={toggleFilteredSelection}>
						{allFilteredSelected ? 'Deselect filtered' : 'Select filtered'}
					</button>
					{#if selectedCount > 0}
						<button class="h-9 px-3 text-xs uppercase tracking-wider border border-neutral-600 bg-neutral-900 text-white rounded-none hover:border-neutral-400 transition-colors" onclick={() => showMetadataPanel = !showMetadataPanel}>
							{showMetadataPanel ? 'Hide editor' : 'Edit selection'}
						</button>
						<button class="h-9 px-3 text-xs uppercase tracking-wider border border-neutral-700 text-neutral-300 rounded-none hover:border-neutral-400 hover:text-white transition-colors" onclick={clearSelection}>
							Clear
						</button>
					{/if}
				</div>
			</div>
		</div>
	{/if}

	{#if loading}
		<div class="flex flex-col items-center justify-center min-h-[50vh] space-y-4">
			<span class="loading loading-spinner loading-lg text-white"></span>
			<p class="text-neutral-500 uppercase tracking-widest text-sm">Scanning Library...</p>
		</div>
	{:else if error && videos.length === 0}
		<div class="flex flex-col items-center justify-center min-h-[50vh] space-y-4">
			<p class="text-neutral-500 uppercase tracking-widest text-sm border border-neutral-800 p-8">Error: {error}</p>
		</div>
	{:else if videos.length === 0}
		<div class="flex flex-col items-center justify-center min-h-[50vh] space-y-4">
			<div class="border border-neutral-800 p-12 flex flex-col items-center text-center max-w-md">
				<p class="text-neutral-400 uppercase tracking-widest mb-4">No Videos Found</p>
				<p class="text-sm text-neutral-600 mb-6">Your library is currently empty or scanning is still in progress.</p>
				<a href="/settings" class="btn btn-outline btn-sm text-white rounded-none uppercase tracking-widest text-xs">Go to Settings</a>
			</div>
		</div>
	{:else if filteredVideos.length === 0}
		<div class="flex flex-col items-center justify-center min-h-[50vh] space-y-4">
			<div class="border border-neutral-800 bg-black p-12 flex flex-col items-center text-center max-w-md">
				<p class="text-neutral-400 uppercase tracking-widest mb-4">No Matching Videos</p>
				<p class="text-sm text-neutral-600">Try a different title or date-added search.</p>
			</div>
		</div>
	{:else}
		<div class="text-xs text-neutral-500 uppercase tracking-wider mb-4 flex flex-wrap items-center justify-between gap-2">
			<span>Showing {(currentPage - 1) * itemsPerPage + 1} - {Math.min(currentPage * itemsPerPage, filteredVideos.length)} of {filteredVideos.length} videos</span>
			{#if selectedCount > 0}
				<span class="text-white">{selectedCount} selected</span>
			{/if}
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
				<button class="p-2 rounded border border-neutral-700 hover:border-neutral-500 transition-colors text-neutral-400 hover:text-white disabled:opacity-50 disabled:cursor-not-allowed" onclick={() => goToPage(currentPage - 1)} disabled={currentPage === 1} aria-label="Previous page">
					<svg class="w-5 h-5" viewBox="0 0 24 24" fill="currentColor"><path d="M15.41 7.41L14 6l-6 6 6 6 1.41-1.41L10.83 12z"/></svg>
				</button>

				{#each Array.from({ length: Math.min(5, totalPages) }, (_, i) => {
					let start = Math.max(1, Math.min(currentPage - 2, totalPages - 4));
					return start + i;
				}) as pageNum}
					<button class="px-3 py-1 text-xs font-mono rounded border transition-colors {currentPage === pageNum ? 'bg-neutral-700 text-white border-neutral-600' : 'text-neutral-400 border-neutral-700 hover:border-neutral-500 hover:text-white'}" onclick={() => goToPage(pageNum)} aria-label="Page {pageNum}">{pageNum}</button>
				{/each}

				{#if totalPages > 5 && currentPage < totalPages - 2}
					<span class="text-neutral-500 px-1">...</span>
				{/if}

				{#if totalPages > 5 && currentPage < totalPages - 2}
					<button class="px-3 py-1 text-xs font-mono rounded border text-neutral-400 border-neutral-700 hover:border-neutral-500 hover:text-white transition-colors" onclick={() => goToPage(totalPages)} aria-label="Last page">{totalPages}</button>
				{/if}

				<button class="p-2 rounded border border-neutral-700 hover:border-neutral-500 transition-colors text-neutral-400 hover:text-white disabled:opacity-50 disabled:cursor-not-allowed" onclick={() => goToPage(currentPage + 1)} disabled={currentPage === totalPages} aria-label="Next page">
					<svg class="w-5 h-5" viewBox="0 0 24 24" fill="currentColor"><path d="M10 6L8.59 7.41 13.17 12l-4.58 4.59L10 18l6-6z"/></svg>
				</button>
			</div>
		{/if}
	{/if}
</div>

{#if selectionMode && selectedCount > 0}
	<div class="fixed inset-y-0 right-0 z-40 w-full max-w-md border-l border-neutral-800 bg-black/95 shadow-2xl backdrop-blur transition-transform duration-300 {showMetadataPanel ? 'translate-x-0' : 'translate-x-full'}">
		<div class="flex h-full flex-col">
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
				<button class="mt-0.5 text-neutral-400 hover:text-white transition-colors" aria-label="Close metadata editor" onclick={() => showMetadataPanel = false}>
					<svg class="w-6 h-6" viewBox="0 0 24 24" fill="currentColor"><path d="M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z"/></svg>
				</button>
			</div>

			<div class="flex-1 overflow-y-auto px-6 py-5 text-sm text-white/80">
				{#if selectedCount === 1}
					<div class="space-y-6">
						<div class="grid gap-4 border border-neutral-800 bg-neutral-950/60 p-4 text-sm">
							<div>
								<p class="text-[10px] uppercase tracking-[0.3em] text-neutral-500">Selection</p>
								<p class="mt-2 text-white">Exact metadata editing for this video group. Changes apply to every quality variant with the same title.</p>
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
							<p class="mt-2 text-white">Bulk updates are non-destructive: add tags and actors to every selected group, or remove specific ones across the whole selection.</p>
						</div>

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
							helpText="Matching tags will be removed from every selected group. Tags already queued for add are hidden here."
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
							helpText="Matching names will be removed from every selected group. Names already queued for add are hidden here."
							disabled={savingMetadata}
							onChange={handleBulkRemoveActorsChange}
						/>
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
					<button class="border border-neutral-700 px-4 py-2 text-xs font-bold uppercase tracking-[0.25em] text-neutral-300 transition-colors hover:border-neutral-500 hover:text-white" onclick={clearSelection}>
						Clear Selection
					</button>
				</div>
				{#if metadataMessage}
					<p class="mt-3 text-xs text-neutral-400">{metadataMessage}</p>
				{/if}
			</div>
		</div>
	</div>
{/if}
