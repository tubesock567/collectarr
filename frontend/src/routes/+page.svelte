<script>
	import { onMount } from 'svelte';
	import { authFetch } from '$lib/auth';
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

    const normalizedSearchQuery = $derived(searchQuery.trim().toLowerCase());

    const filteredVideos = $derived.by(() => {
        const query = normalizedSearchQuery;
        if (!query) {
            return videos;
        }

        return videos.filter((video) => {
            const title = (video.title || '').toLowerCase();
            const addedDate = (video.date_added || '').toLowerCase();
            return title.includes(query) || addedDate.includes(query);
        });
    });

    const totalPages = $derived(Math.max(1, Math.ceil(filteredVideos.length / itemsPerPage)));

    const sortedVideos = $derived.by(() => {
        const sorted = [...filteredVideos];
        sorted.sort((a, b) => {
            let valA, valB;
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
        const end = start + itemsPerPage;
        return sortedVideos.slice(start, end);
    });

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

    $effect(() => {
        const pageCount = totalPages;
        if (currentPage > pageCount) {
            currentPage = pageCount;
        }
    });

	async function readError(res, fallback) {
		try {
			const data = await res.json();
			return data?.error || fallback;
		} catch {
			return fallback;
		}
	}

	onMount(() => {
        document.addEventListener('click', handleClickOutside);

		(async () => {
			try {
				const res = await authFetch('/api/videos');
				if (!res.ok) throw new Error(await readError(res, 'Failed to fetch videos'));
				videos = await res.json();
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

<div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
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
			<label class="flex items-center h-9 border border-neutral-800 bg-black text-white rounded-none overflow-hidden focus-within:border-neutral-500 transition-colors">
				<span class="px-3 text-[10px] uppercase tracking-[0.25em] text-neutral-500 border-r border-neutral-800 h-full flex items-center shrink-0">Search</span>
				<input
					type="search"
					value={searchQuery}
					oninput={handleSearchInput}
					placeholder="Title or date added"
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
					<span class="p-0.5 hover:bg-neutral-700 rounded-none"
						onclick={(e) => { e.stopPropagation(); toggleSortOrder(); }}
						role="button"
						tabindex="0"
						onkeydown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.stopPropagation(); toggleSortOrder(); }}}
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
						<button
							class="w-full px-3 py-2 text-sm text-left hover:bg-neutral-900 transition-colors {sortBy === 'dateAdded' ? 'text-white bg-neutral-900' : 'text-neutral-300'}"
							onclick={() => setSort('dateAdded')}
						>
							Date added
						</button>
						<button
							class="w-full px-3 py-2 text-sm text-left hover:bg-neutral-900 transition-colors {sortBy === 'duration' ? 'text-white bg-neutral-900' : 'text-neutral-300'}"
							onclick={() => setSort('duration')}
						>
							Duration
						</button>
						<button
							class="w-full px-3 py-2 text-sm text-left hover:bg-neutral-900 transition-colors {sortBy === 'alphabetical' ? 'text-white bg-neutral-900' : 'text-neutral-300'}"
							onclick={() => setSort('alphabetical')}
						>
							Alphabetical
						</button>
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
							<button
								class="w-full px-3 py-2 text-sm text-left hover:bg-neutral-900 transition-colors {columnCount === count ? 'text-white bg-neutral-900' : 'text-neutral-300'}"
								onclick={() => setColumnCount(count)}
							>
								{count} columns
							</button>
						{/each}
					</div>
				{/if}
			</div>

		</div>
	</div>

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
		<div class="text-xs text-neutral-500 uppercase tracking-wider mb-4">
			Showing {(currentPage - 1) * itemsPerPage + 1} - {Math.min(currentPage * itemsPerPage, filteredVideos.length)} of {filteredVideos.length} videos
		</div>

		<div class="grid {getColumnClass(columnCount)} gap-6">
			{#each paginatedVideos as video (video.id)}
				<VideoCard {video} />
			{/each}
		</div>

		{#if totalPages > 1}
			<div class="flex justify-center items-center gap-2 mt-8">
				<button
					class="p-2 rounded border border-neutral-700 hover:border-neutral-500 transition-colors text-neutral-400 hover:text-white disabled:opacity-50 disabled:cursor-not-allowed"
					onclick={() => goToPage(currentPage - 1)}
					disabled={currentPage === 1}
					aria-label="Previous page"
				>
					<svg class="w-5 h-5" viewBox="0 0 24 24" fill="currentColor">
						<path d="M15.41 7.41L14 6l-6 6 6 6 1.41-1.41L10.83 12z"/>
					</svg>
				</button>

				{#each Array.from({ length: Math.min(5, totalPages) }, (_, i) => {
					let start = Math.max(1, Math.min(currentPage - 2, totalPages - 4));
					return start + i;
				}) as pageNum}
					<button
						class="px-3 py-1 text-xs font-mono rounded border transition-colors {currentPage === pageNum ? 'bg-neutral-700 text-white border-neutral-600' : 'text-neutral-400 border-neutral-700 hover:border-neutral-500 hover:text-white'}"
						onclick={() => goToPage(pageNum)}
						aria-label="Page {pageNum}"
					>
						{pageNum}
					</button>
				{/each}

				{#if totalPages > 5 && currentPage < totalPages - 2}
					<span class="text-neutral-500 px-1">...</span>
				{/if}

				{#if totalPages > 5 && currentPage < totalPages - 2}
					<button
						class="px-3 py-1 text-xs font-mono rounded border text-neutral-400 border-neutral-700 hover:border-neutral-500 hover:text-white transition-colors"
						onclick={() => goToPage(totalPages)}
						aria-label="Last page"
					>
						{totalPages}
					</button>
				{/if}

				<button
					class="p-2 rounded border border-neutral-700 hover:border-neutral-500 transition-colors text-neutral-400 hover:text-white disabled:opacity-50 disabled:cursor-not-allowed"
					onclick={() => goToPage(currentPage + 1)}
					disabled={currentPage === totalPages}
					aria-label="Next page"
				>
					<svg class="w-5 h-5" viewBox="0 0 24 24" fill="currentColor">
						<path d="M10 6L8.59 7.41 13.17 12l-4.58 4.59L10 18l6-6z"/>
					</svg>
				</button>
			</div>
		{/if}
	{/if}
</div>
