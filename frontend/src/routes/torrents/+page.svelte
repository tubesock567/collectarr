<script>
	import { authFetch } from '$lib/auth';
	import { onMount } from 'svelte';

	// Tab state
	let activeTab = $state('search');
	const tabs = [
		{ id: 'search', label: 'Search' },
		{ id: 'indexers', label: 'Manage Indexers' },
		{ id: 'history', label: 'History' }
	];

	// Indexers state
	let indexers = $state([]);
	let indexersLoading = $state(true);
	let indexerMessage = $state('');
	let savingIndexer = $state(false);
	let deletingIndexerId = $state('');
	let label = $state('');
	let torznabURL = $state('');
	let apiKey = $state('');
	let showAddIndexerPopup = $state(false);

	// Search state
	let query = $state('');
	let minSeeders = $state('');
	let minSize = $state('');
	let maxSize = $state('');
	let resultFilter = $state('');
	let searching = $state(false);
	let searchMessage = $state('');
	let searchedQuery = $state('');
	let results = $state([]);
	let warnings = $state([]);
	let searchInputRef = $state(null);

	// Pagination state
	let currentPage = $state(1);
	let itemsPerPage = $state(20);
	let totalResults = $state(0);
	let totalPages = $derived(Math.max(1, Math.ceil(totalResults / itemsPerPage)));

	// Filtered and deduplicated results
	let uniqueResults = $derived.by(() => {
		const seen = new Map();
		const filtered = [];
		for (const result of results) {
			const key = `${result.title?.toLowerCase()}-${result.size}`;
			if (seen.has(key)) {
				const existing = seen.get(key);
				if (result.seeders > existing.seeders) {
					seen.set(key, result);
					const idx = filtered.indexOf(existing);
					if (idx >= 0) filtered[idx] = result;
				}
			} else {
				seen.set(key, result);
				filtered.push(result);
			}
		}
		return filtered;
	});

	let filteredResults = $derived.by(() => {
		if (!resultFilter.trim()) return uniqueResults;
		const filter = resultFilter.toLowerCase();
		return uniqueResults.filter(
			(r) => r.title?.toLowerCase().includes(filter) || r.tracker?.toLowerCase().includes(filter)
		);
	});

	let totalResultsFiltered = $derived(filteredResults.length);
	let paginatedResults = $derived.by(() => {
		const start = (currentPage - 1) * itemsPerPage;
		return sortedResults.slice(start, start + itemsPerPage);
	});

	// Sorting state
	let sortBy = $state('seeders');
	let sortOrder = $state('desc');
	let sortedResults = $derived.by(() => {
		const sorted = [...filteredResults];
		sorted.sort((a, b) => {
			let valA, valB;
			switch (sortBy) {
				case 'title':
					valA = (a.title || '').toLowerCase();
					valB = (b.title || '').toLowerCase();
					break;
				case 'tracker':
					valA = (a.tracker || '').toLowerCase();
					valB = (b.tracker || '').toLowerCase();
					break;
				case 'size':
					valA = a.size || 0;
					valB = b.size || 0;
					break;
				case 'seeders':
					valA = a.seeders || 0;
					valB = b.seeders || 0;
					break;
				case 'leechers':
					valA = a.leechers || 0;
					valB = b.leechers || 0;
					break;
				case 'published':
					valA = a.published ? new Date(a.published).getTime() : 0;
					valB = b.published ? new Date(b.published).getTime() : 0;
					break;
				default:
					valA = a.seeders || 0;
					valB = b.seeders || 0;
			}
			if (valA < valB) return sortOrder === 'asc' ? -1 : 1;
			if (valA > valB) return sortOrder === 'asc' ? 1 : -1;
			return 0;
		});
		return sorted;
	});

	// History state
	let historyItems = $state([]);
	let historyLoading = $state(false);
	let historyPage = $state(1);
	let historyPerPage = $state(20);
	let historyTotalCount = $state(0);
	let historyTotalPages = $derived(Math.max(1, Math.ceil(historyTotalCount / historyPerPage)));
	let historyMessage = $state('');
	let historyTrackerFilter = $state('');
	let historySearchFilter = $state('');
	let uniqueTrackers = $derived([...new Set(historyItems.map((i) => i.tracker).filter(Boolean))]);
	let deletingHistoryId = $state(null);

	// Auto-refresh history when tab is active
	$effect(() => {
		if (activeTab !== 'history') return;
		loadHistory();
	});

	onMount(async () => {
		await loadIndexers();

		// Keyboard shortcuts
		document.addEventListener('keydown', handleKeydown);
		return () => {
			document.removeEventListener('keydown', handleKeydown);
		};
	});

	function handleKeydown(e) {
		// / or Ctrl+K to focus search
		if ((e.key === '/' || (e.ctrlKey && e.key === 'k')) && activeTab === 'search') {
			e.preventDefault();
			searchInputRef?.focus();
		}
		// Arrow keys for pagination
		if (activeTab === 'search' && results.length > 0) {
			if (e.key === 'ArrowLeft' && currentPage > 1) {
				e.preventDefault();
				goToPage(currentPage - 1);
			}
			if (e.key === 'ArrowRight' && currentPage < totalPages) {
				e.preventDefault();
				goToPage(currentPage + 1);
			}
		}
	}

	async function readJSONSafe(response) {
		try {
			return await response.json();
		} catch {
			return null;
		}
	}

	async function readError(response, fallback) {
		const data = await readJSONSafe(response);
		return data?.error || fallback;
	}

	async function loadIndexers() {
		indexersLoading = true;
		try {
			const response = await authFetch('/api/settings/torrent-indexers');
			if (!response.ok) {
				throw new Error(await readError(response, 'Failed to load torrent indexers'));
			}
			indexers = await response.json();
		} catch (error) {
			indexerMessage = `Error: ${error.message}`;
		} finally {
			indexersLoading = false;
		}
	}

	async function addIndexer() {
		if (savingIndexer) return;
		indexerMessage = '';

		const trimmedLabel = label.trim();
		const trimmedURL = torznabURL.trim();
		const trimmedKey = apiKey.trim();
		if (!trimmedURL || !trimmedKey) {
			indexerMessage = 'Error: Torznab link and Jackett API key are required.';
			return;
		}

		savingIndexer = true;
		try {
			const response = await authFetch('/api/settings/torrent-indexers', {
				method: 'POST',
				body: JSON.stringify({
					label: trimmedLabel,
					torznab_url: trimmedURL,
					api_key: trimmedKey
				})
			});

			if (!response.ok) {
				throw new Error(await readError(response, 'Failed to save torrent indexer'));
			}

			indexers = await response.json();
			label = '';
			torznabURL = '';
			apiKey = '';
			showAddIndexerPopup = false;
			indexerMessage = 'Torrent indexer added.';
		} catch (error) {
			indexerMessage = `Error: ${error.message}`;
		} finally {
			savingIndexer = false;
		}
	}

	async function removeIndexer(id) {
		if (deletingIndexerId) return;
		indexerMessage = '';
		deletingIndexerId = id;
		try {
			const response = await authFetch(`/api/settings/torrent-indexers/${id}`, {
				method: 'DELETE'
			});
			if (!response.ok) {
				throw new Error(await readError(response, 'Failed to delete torrent indexer'));
			}
			indexers = await response.json();
			indexerMessage = 'Torrent indexer removed.';
		} catch (error) {
			indexerMessage = `Error: ${error.message}`;
		} finally {
			deletingIndexerId = '';
		}
	}

	async function searchTorrents() {
		if (searching) return;
		searchMessage = '';
		results = [];
		totalResults = 0;
		warnings = [];
		currentPage = 1;
		resultFilter = '';

		const trimmedQuery = query.trim();
		if (!trimmedQuery) {
			searchMessage = 'Error: Enter search query.';
			return;
		}
		if (indexers.length === 0) {
			searchMessage = 'Error: Add at least one torrent indexer first.';
			return;
		}

		searching = true;
		searchedQuery = trimmedQuery;
		try {
			const params = new URLSearchParams({ q: trimmedQuery });
			if (minSeeders) params.set('min_seeders', minSeeders);
			if (minSize) params.set('min_size', minSize);
			if (maxSize) params.set('max_size', maxSize);

			const response = await authFetch(`/api/torrents/search?${params.toString()}`);
			if (!response.ok) {
				throw new Error(await readError(response, 'Torrent search failed'));
			}
			const data = await response.json();
			results = data?.results || [];
			totalResults = results.length;
			warnings = data?.warnings || [];
			searchMessage =
				results.length > 0 ? `Found ${results.length} results.` : 'No torrent results found.';
		} catch (error) {
			searchMessage = `Error: ${error.message}`;
		} finally {
			searching = false;
		}
	}

	function handleSearchSubmit(event) {
		event.preventDefault();
		searchTorrents();
	}

	function handleSort(field) {
		if (sortBy === field) {
			sortOrder = sortOrder === 'asc' ? 'desc' : 'asc';
		} else {
			sortBy = field;
			sortOrder = field === 'title' || field === 'tracker' ? 'asc' : 'desc';
		}
		currentPage = 1;
	}

	function goToPage(page) {
		if (page >= 1 && page <= totalPages) {
			currentPage = page;
			window.scrollTo({ top: 0, behavior: 'smooth' });
		}
	}

	async function recordDownload(result, event) {
		// Prevent default download behavior to track status
		if (event) event.preventDefault();

		const downloadUrl = result.download_url || result.download_url;
		if (!downloadUrl) return;

		// Record as pending first
		try {
			await authFetch('/api/torrents/history', {
				method: 'POST',
				body: JSON.stringify({
					title: result.title,
					url: result.url,
					download_url: downloadUrl,
					tracker: result.tracker,
					size: result.size,
					seeders: result.seeders,
					leechers: result.leechers,
					freeleech: result.freeleech,
					status: 'pending'
				})
			});
		} catch (err) {
			console.error('Failed to record download:', err);
		}

		// Trigger actual download
		const a = document.createElement('a');
		a.href = downloadUrl;
		a.download = '';
		a.target = '_blank';
		document.body.appendChild(a);
		a.click();
		document.body.removeChild(a);

		// Update status to success after a brief delay
		setTimeout(async () => {
			try {
				await authFetch('/api/torrents/history', {
					method: 'POST',
					body: JSON.stringify({
						title: result.title,
						url: result.url,
						download_url: downloadUrl,
						tracker: result.tracker,
						size: result.size,
						seeders: result.seeders,
						leechers: result.leechers,
						freeleech: result.freeleech,
						status: 'success'
					})
				});
			} catch (err) {
				console.error('Failed to update download status:', err);
			}
		}, 1000);
	}

	async function loadHistory() {
		historyLoading = true;
		historyMessage = '';
		try {
			const params = new URLSearchParams({
				page: historyPage.toString(),
				per_page: historyPerPage.toString()
			});
			if (historyTrackerFilter) params.set('tracker', historyTrackerFilter);
			if (historySearchFilter) params.set('search', historySearchFilter);

			const response = await authFetch(`/api/torrents/history?${params.toString()}`);
			if (!response.ok) {
				throw new Error(await readError(response, 'Failed to load history'));
			}
			const data = await response.json();
			historyItems = data?.items || [];
			historyTotalCount = data?.total_count || 0;
		} catch (error) {
			historyMessage = `Error: ${error.message}`;
		} finally {
			historyLoading = false;
		}
	}

	async function deleteHistoryItem(id) {
		if (deletingHistoryId) return;
		deletingHistoryId = id;
		try {
			const response = await authFetch(`/api/torrents/history/${id}`, { method: 'DELETE' });
			if (!response.ok) {
				throw new Error(await readError(response, 'Failed to delete history item'));
			}
			await loadHistory();
		} catch (error) {
			historyMessage = `Error: ${error.message}`;
		} finally {
			deletingHistoryId = null;
		}
	}

	async function clearHistory() {
		if (!confirm('Clear all download history?')) return;
		try {
			const response = await authFetch('/api/torrents/history/clear', { method: 'POST' });
			if (!response.ok) {
				throw new Error(await readError(response, 'Failed to clear history'));
			}
			historyItems = [];
			historyTotalCount = 0;
			historyPage = 1;
			historyMessage = 'History cleared.';
		} catch (error) {
			historyMessage = `Error: ${error.message}`;
		}
	}

	function goToHistoryPage(page) {
		if (page >= 1 && page <= historyTotalPages) {
			historyPage = page;
			loadHistory();
			window.scrollTo({ top: 0, behavior: 'smooth' });
		}
	}

	function formatBytes(bytes) {
		if (!bytes || bytes <= 0) return 'Unknown';
		const units = ['B', 'KB', 'MB', 'GB', 'TB'];
		let value = bytes;
		let index = 0;
		while (value >= 1024 && index < units.length - 1) {
			value /= 1024;
			index += 1;
		}
		return `${value >= 10 || index === 0 ? value.toFixed(0) : value.toFixed(1)} ${units[index]}`;
	}

	function maskAPIKey(value) {
		return value || 'Not set';
	}

	function getIndexerDisplayName(indexer) {
		return indexer.label || indexer.tracker || 'Unnamed';
	}

	function formatDate(dateStr) {
		if (!dateStr) return 'Unknown';
		const date = new Date(dateStr);
		return date.toLocaleString();
	}

	function formatRelativeTime(dateStr) {
		if (!dateStr) return 'Unknown';
		const date = new Date(dateStr);
		const now = new Date();
		const diffMs = now - date;
		const diffDay = Math.floor(diffMs / (1000 * 60 * 60 * 24));
		const diffYear = Math.floor(diffDay / 365);

		if (diffDay < 1) return '< 1d';
		if (diffYear < 1) return `${diffDay}d`;
		return `${diffYear}y`;
	}

	function getSortIcon(field) {
		if (sortBy !== field) {
			return '<svg class="inline h-3 w-3 ml-1 text-neutral-600" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M7 15l5 5 5-5M7 9l5-5 5 5"/></svg>';
		}
		if (sortOrder === 'asc') {
			return '<svg class="inline h-3 w-3 ml-1 text-white" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M5 15l7-7 7 7"/></svg>';
		}
		return '<svg class="inline h-3 w-3 ml-1 text-white" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M19 9l-7 7-7-7"/></svg>';
	}

	function isMagnetUrl(url) {
		return url?.startsWith('magnet:');
	}
</script>

<svelte:head>
	<title>Collectarr - Torrents</title>
</svelte:head>

<div class="mx-auto max-w-7xl px-4 py-12 sm:px-6 lg:px-8">
	<div class="mb-8 border-b border-neutral-800 pb-4">
		<h1 class="text-2xl font-bold uppercase tracking-widest">Torrents</h1>
		<p class="mt-2 max-w-3xl text-sm text-neutral-500">
			Search torrents across Jackett indexers, manage your indexers, and view download history.
		</p>
	</div>

	<!-- Tabs -->
	<div class="mb-8 border-b border-neutral-800">
		<div class="flex min-w-max gap-1">
			{#each tabs as tab}
				<button
					onclick={() => (activeTab = tab.id)}
					role="tab"
					aria-selected={activeTab === tab.id}
					class="shrink-0 whitespace-nowrap px-6 py-3 text-xs uppercase tracking-widest font-semibold transition-colors {activeTab ===
					tab.id
						? 'bg-white text-black'
						: 'text-neutral-400 hover:text-white hover:bg-neutral-900'}"
				>
					{tab.label}
				</button>
			{/each}
		</div>
	</div>

	<!-- Search Tab -->
	{#if activeTab === 'search'}
		<section class="space-y-6">
			<section class="border border-neutral-800 p-6">
				<div class="flex flex-col gap-6 lg:flex-row lg:items-end lg:justify-between">
					<div>
						<h2 class="text-sm font-semibold uppercase tracking-widest text-white">
							Search Torrents
						</h2>
						<p class="mt-1 text-xs text-neutral-500">
							Search all saved indexers in parallel through Jackett Torznab.
						</p>
					</div>

					{#if searchedQuery}
						<p class="text-xs uppercase tracking-[0.25em] text-neutral-500">
							Last query: {searchedQuery}
						</p>
					{/if}
				</div>

				<form class="mt-6 grid gap-4" onsubmit={handleSearchSubmit}>
					<input
						bind:this={searchInputRef}
						bind:value={query}
						type="search"
						placeholder="Search torrents..."
						class="w-full border border-neutral-800 bg-black px-4 py-3 text-sm outline-none focus:border-neutral-500"
					/>

					<div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
						<label class="grid gap-1">
							<span class="text-[10px] uppercase tracking-[0.25em] text-neutral-500"
								>Min Seeders</span
							>
							<input
								bind:value={minSeeders}
								type="number"
								min="0"
								placeholder="e.g., 5"
								class="w-full border border-neutral-800 bg-black px-3 py-2 text-sm outline-none focus:border-neutral-500"
							/>
						</label>
						<label class="grid gap-1">
							<span class="text-[10px] uppercase tracking-[0.25em] text-neutral-500"
								>Min Size (GB)</span
							>
							<input
								bind:value={minSize}
								type="number"
								min="0"
								step="0.1"
								placeholder="e.g., 1"
								class="w-full border border-neutral-800 bg-black px-3 py-2 text-sm outline-none focus:border-neutral-500"
							/>
						</label>
						<label class="grid gap-1">
							<span class="text-[10px] uppercase tracking-[0.25em] text-neutral-500"
								>Max Size (GB)</span
							>
							<input
								bind:value={maxSize}
								type="number"
								min="0"
								step="0.1"
								placeholder="e.g., 10"
								class="w-full border border-neutral-800 bg-black px-3 py-2 text-sm outline-none focus:border-neutral-500"
							/>
						</label>
						<div class="flex items-end">
							<button
								type="submit"
								disabled={searching}
								class="w-full bg-white px-6 py-3 text-xs font-bold uppercase tracking-widest text-black transition-colors hover:bg-neutral-300 disabled:bg-neutral-800 disabled:text-neutral-500"
							>
								{#if searching}
									Searching...
								{:else}
									Search
								{/if}
							</button>
						</div>
					</div>
				</form>

				{#if searchMessage}
					<p
						class="mt-4 text-xs tracking-wide {searchMessage.startsWith('Error')
							? 'text-red-500'
							: 'text-neutral-400'}"
					>
						{searchMessage}
					</p>
				{/if}

				{#if warnings.length > 0}
					<div
						class="mt-4 space-y-2 border border-amber-600 bg-amber-100/50 p-4 text-xs text-amber-900 dark:border-amber-900 dark:bg-amber-950/20 dark:text-amber-300"
					>
						<div class="flex items-center justify-between">
							<p class="uppercase tracking-[0.25em] text-amber-900 dark:text-amber-400">
								Indexer warnings ({warnings.length})
							</p>
							<button
								onclick={searchTorrents}
								class="text-[10px] uppercase tracking-widest text-amber-900 hover:text-black dark:text-amber-400 dark:hover:text-white"
								>Retry</button
							>
						</div>
						{#each warnings as warning}
							<p>{warning}</p>
						{/each}
					</div>
				{/if}
			</section>

			{#if filteredResults.length > 0}
				<section class="border border-neutral-800 p-6">
					<div class="flex flex-col gap-4 mb-4 lg:flex-row lg:items-center lg:justify-between">
						<div>
							<h2 class="text-sm font-semibold uppercase tracking-widest text-white">Results</h2>
							<p class="mt-1 text-xs text-neutral-500">
								Click column headers to sort. Click title to open torrent page. Use arrow keys for
								pagination.
							</p>
						</div>
						<div class="flex flex-col gap-3 sm:flex-row sm:items-center">
							<input
								bind:value={resultFilter}
								type="text"
								placeholder="Filter results..."
								class="border border-neutral-800 bg-black px-3 py-2 text-sm outline-none focus:border-neutral-500"
							/>
							<p class="text-xs uppercase tracking-[0.25em] text-neutral-500">
								{(currentPage - 1) * itemsPerPage + 1} - {Math.min(
									currentPage * itemsPerPage,
									totalResultsFiltered
								)} of {totalResultsFiltered}
								{#if totalResultsFiltered !== totalResults}
									(filtered from {totalResults})
								{/if}
							</p>
						</div>
					</div>

					<div class="overflow-x-auto border border-neutral-800 bg-black">
						<table class="min-w-full divide-y divide-neutral-800 text-left text-sm">
							<thead
								class="bg-neutral-950 text-[11px] uppercase tracking-[0.25em] text-neutral-400"
							>
								<tr>
									<th
										class="px-4 py-3 cursor-pointer hover:text-white select-none whitespace-nowrap"
										onclick={() => handleSort('title')}
									>
										Title {@html getSortIcon('title')}
									</th>
									<th
										class="px-4 py-3 cursor-pointer hover:text-white select-none whitespace-nowrap"
										onclick={() => handleSort('tracker')}
									>
										Tracker {@html getSortIcon('tracker')}
									</th>
									<th
										class="px-4 py-3 cursor-pointer hover:text-white select-none whitespace-nowrap"
										onclick={() => handleSort('size')}
									>
										Size {@html getSortIcon('size')}
									</th>
									<th
										class="px-4 py-3 cursor-pointer hover:text-white select-none whitespace-nowrap"
										onclick={() => handleSort('seeders')}
									>
										Seeders {@html getSortIcon('seeders')}
									</th>
									<th
										class="px-4 py-3 cursor-pointer hover:text-white select-none whitespace-nowrap"
										onclick={() => handleSort('leechers')}
									>
										Leechers {@html getSortIcon('leechers')}
									</th>
									<th
										class="px-4 py-3 cursor-pointer hover:text-white select-none whitespace-nowrap"
										onclick={() => handleSort('published')}
									>
										Age {@html getSortIcon('published')}
									</th>
									<th class="px-4 py-3"></th>
								</tr>
							</thead>
							<tbody class="divide-y divide-neutral-900">
								{#each paginatedResults as result, index (`${result.tracker}-${result.download_url || result.url || result.title}-${index}`)}
									<tr
										class="align-top hover:bg-neutral-950/70 {result.seeders >= 50
											? 'bg-emerald-950/10'
											: result.seeders >= 20
												? 'bg-blue-950/10'
												: ''}"
									>
										<td class="max-w-xl px-4 py-4 text-white">
											<div class="space-y-2">
												<div class="flex items-center gap-2 flex-wrap">
													{#if result.url}
														<a
															href={result.url}
															target="_blank"
															rel="noreferrer"
															class="font-medium leading-6 transition-colors hover:text-neutral-300"
															>{result.title || 'Untitled torrent'}</a
														>
													{:else}
														<p class="font-medium leading-6">
															{result.title || 'Untitled torrent'}
														</p>
													{/if}
													{#if result.freeleech}
														<span
															class="inline-flex border border-emerald-700 bg-emerald-50 px-2 py-0.5 text-[10px] uppercase tracking-[0.25em] text-emerald-900 dark:border-emerald-800 dark:bg-transparent dark:text-emerald-300"
															>Freeleech</span
														>
													{/if}
													{#if isMagnetUrl(result.download_url)}
														<span
															class="inline-flex border border-purple-700 bg-purple-50 px-2 py-0.5 text-[10px] uppercase tracking-[0.25em] text-purple-900 dark:border-purple-800 dark:bg-transparent dark:text-purple-300"
															>Magnet</span
														>
													{:else}
														<span
															class="inline-flex border border-blue-700 bg-blue-50 px-2 py-0.5 text-[10px] uppercase tracking-[0.25em] text-blue-900 dark:border-blue-800 dark:bg-transparent dark:text-blue-300"
															>Torrent</span
														>
													{/if}
												</div>
											</div>
										</td>
										<td class="px-4 py-4 text-neutral-300">{result.tracker || 'Unknown'}</td>
										<td class="px-4 py-4 text-neutral-300">{formatBytes(result.size)}</td>
										<td
											class="px-4 py-4 {result.seeders >= 50
												? 'text-emerald-700 font-semibold dark:text-emerald-400'
												: result.seeders >= 20
													? 'text-blue-700 dark:text-blue-400'
													: 'text-neutral-700 dark:text-neutral-300'}">{result.seeders ?? 0}</td
										>
										<td class="px-4 py-4 text-neutral-300">{result.leechers ?? 0}</td>
										<td
											class="px-4 py-4 text-neutral-300"
											title={result.published ? formatDate(result.published) : ''}
											>{result.published ? formatRelativeTime(result.published) : 'Unknown'}</td
										>
										<td class="px-4 py-4">
											{#if result.download_url}
												<button
													onclick={(e) => recordDownload(result, e)}
													class="inline-flex items-center justify-center text-white transition-colors hover:text-neutral-300"
													aria-label="Download"
												>
													<svg class="h-5 w-5" viewBox="0 0 24 24" fill="currentColor">
														<path d="M19 9h-4V3H9v6H5l7 7 7-7zM5 18v2h14v-2H5z" />
													</svg>
												</button>
											{:else}
												<span class="text-xs uppercase tracking-[0.25em] text-neutral-600">N/A</span
												>
											{/if}
										</td>
									</tr>
								{/each}
							</tbody>
						</table>
					</div>

					<!-- Pagination -->
					{#if totalPages > 1}
						<div class="mt-6 flex items-center justify-center gap-2">
							<button
								onclick={() => goToPage(currentPage - 1)}
								disabled={currentPage === 1}
								class="px-3 py-2 border border-neutral-800 text-xs uppercase tracking-widest text-neutral-400 transition-colors hover:text-white hover:border-neutral-600 disabled:opacity-50 disabled:cursor-not-allowed"
							>
								← Prev
							</button>

							{#each Array.from({ length: Math.min(5, totalPages) }, (_, i) => {
								let start = Math.max(1, Math.min(currentPage - 2, totalPages - 4));
								return start + i;
							}) as pageNum}
								<button
									onclick={() => goToPage(pageNum)}
									class="px-3 py-2 border text-xs uppercase tracking-widest transition-colors {currentPage ===
									pageNum
										? 'bg-white text-black border-white'
										: 'border-neutral-800 text-neutral-400 hover:text-white hover:border-neutral-600'}"
								>
									{pageNum}
								</button>
							{/each}

							{#if totalPages > 5 && currentPage < totalPages - 2}
								<span class="text-neutral-600">...</span>
								<button
									onclick={() => goToPage(totalPages)}
									class="px-3 py-2 border border-neutral-800 text-xs uppercase tracking-widest text-neutral-400 transition-colors hover:text-white hover:border-neutral-600"
								>
									{totalPages}
								</button>
							{/if}

							<button
								onclick={() => goToPage(currentPage + 1)}
								disabled={currentPage === totalPages}
								class="px-3 py-2 border border-neutral-800 text-xs uppercase tracking-widest text-neutral-400 transition-colors hover:text-white hover:border-neutral-600 disabled:opacity-50 disabled:cursor-not-allowed"
							>
								Next →
							</button>
						</div>
					{/if}
				</section>
			{:else if !searching && searchedQuery}
				<div
					class="flex min-h-64 flex-col items-center justify-center border border-neutral-800 bg-black px-6 text-center"
				>
					<p class="text-xs uppercase tracking-widest text-neutral-500 mb-4">
						No torrent results for current query.
					</p>
					<button
						onclick={searchTorrents}
						class="border border-neutral-700 px-4 py-2 text-xs uppercase tracking-widest text-neutral-400 transition-colors hover:text-white hover:border-neutral-500"
					>
						Retry Search
					</button>
				</div>
			{/if}
		</section>
	{/if}

	<!-- Manage Indexers Tab -->
	{#if activeTab === 'indexers'}
		<section class="border border-neutral-800 p-6">
			<div class="flex items-center justify-between">
				<div>
					<h2 class="text-sm font-semibold uppercase tracking-widest text-white">
						Jackett Indexers
					</h2>
					<p class="mt-1 text-xs text-neutral-500">Configured indexers: {indexers.length}</p>
				</div>
				<button
					onclick={() => (showAddIndexerPopup = true)}
					class="flex h-8 w-8 items-center justify-center border border-neutral-700 bg-neutral-900 text-white transition-colors hover:border-neutral-500 hover:bg-neutral-800"
					aria-label="Add indexer"
				>
					<svg class="h-4 w-4" viewBox="0 0 24 24" fill="currentColor">
						<path d="M19 13h-6v6h-2v-6H5v-2h6V5h2v6h6v2z" />
					</svg>
				</button>
			</div>

			{#if indexerMessage}
				<p
					class="mt-4 text-xs tracking-wide {indexerMessage.startsWith('Error')
						? 'text-red-500'
						: 'text-neutral-400'}"
				>
					{indexerMessage}
				</p>
			{/if}

			<div class="mt-6">
				{#if indexersLoading}
					<div class="flex min-h-32 items-center justify-center border border-neutral-800 bg-black">
						<span class="loading loading-spinner loading-md text-white"></span>
					</div>
				{:else if indexers.length === 0}
					<div
						class="flex min-h-32 flex-col items-center justify-center border border-neutral-800 bg-black px-4 py-5 text-center"
					>
						<p class="text-sm text-neutral-500 mb-4">No Jackett indexers saved yet.</p>
						<button
							onclick={() => (showAddIndexerPopup = true)}
							class="border border-neutral-700 px-4 py-2 text-xs uppercase tracking-widest text-neutral-400 transition-colors hover:text-white hover:border-neutral-500"
						>
							Add Your First Indexer
						</button>
					</div>
				{:else}
					<div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
						{#each indexers as indexer (indexer.id)}
							<div class="border border-neutral-800 bg-black p-4">
								<div class="flex items-start justify-between gap-4">
									<div class="min-w-0 space-y-1">
										<p class="text-xs font-semibold uppercase tracking-[0.3em] text-white">
											{getIndexerDisplayName(indexer)}
										</p>
										<p class="break-all text-[11px] text-neutral-500">{indexer.torznab_url}</p>
										<p class="text-[10px] uppercase tracking-[0.25em] text-neutral-600">
											API Key {maskAPIKey(indexer.masked_api_key)}
										</p>
									</div>
									<button
										onclick={() => removeIndexer(indexer.id)}
										disabled={deletingIndexerId === indexer.id}
										class="shrink-0 border border-red-900 px-2 py-1 text-[10px] font-semibold uppercase tracking-[0.25em] text-red-300 transition-colors hover:bg-red-950 disabled:border-neutral-800 disabled:text-neutral-500"
									>
										{deletingIndexerId === indexer.id ? '...' : 'Remove'}
									</button>
								</div>
							</div>
						{/each}
					</div>
				{/if}
			</div>
		</section>
	{/if}

	<!-- History Tab -->
	{#if activeTab === 'history'}
		<section class="border border-neutral-800 p-6">
			<div class="flex flex-col gap-4 mb-6 lg:flex-row lg:items-center lg:justify-between">
				<div>
					<h2 class="text-sm font-semibold uppercase tracking-widest text-white">
						Download History
					</h2>
					<p class="mt-1 text-xs text-neutral-500">Track your downloaded torrents.</p>
				</div>
				<div class="flex flex-col gap-3 sm:flex-row sm:items-center">
					<select
						bind:value={historyTrackerFilter}
						onchange={loadHistory}
						class="border border-neutral-800 bg-black px-3 py-2 text-sm outline-none focus:border-neutral-500"
					>
						<option value="">All Trackers</option>
						{#each uniqueTrackers as tracker}
							<option value={tracker}>{tracker}</option>
						{/each}
					</select>
					<input
						bind:value={historySearchFilter}
						oninput={() => {
							historyPage = 1;
							loadHistory();
						}}
						type="text"
						placeholder="Search history..."
						class="border border-neutral-800 bg-black px-3 py-2 text-sm outline-none focus:border-neutral-500"
					/>
					<p class="text-xs uppercase tracking-[0.25em] text-neutral-500">
						{historyTotalCount} items
					</p>
					{#if historyItems.length > 0}
						<button
							onclick={clearHistory}
							class="border border-red-900 px-3 py-2 text-[11px] font-semibold uppercase tracking-[0.25em] text-red-300 transition-colors hover:bg-red-950"
						>
							Clear All
						</button>
					{/if}
				</div>
			</div>

			{#if historyMessage}
				<p
					class="mb-4 text-xs tracking-wide {historyMessage.startsWith('Error')
						? 'text-red-500'
						: 'text-neutral-400'}"
				>
					{historyMessage}
				</p>
			{/if}

			{#if historyLoading}
				<div class="flex min-h-64 items-center justify-center border border-neutral-800 bg-black">
					<span class="loading loading-spinner loading-lg text-white"></span>
				</div>
			{:else if historyItems.length === 0}
				<div
					class="flex min-h-64 flex-col items-center justify-center border border-neutral-800 bg-black px-6 text-center"
				>
					<p class="text-xs uppercase tracking-widest text-neutral-500 mb-4">
						No download history yet. Download torrents from the Search tab.
					</p>
					<button
						onclick={() => (activeTab = 'search')}
						class="border border-neutral-700 px-4 py-2 text-xs uppercase tracking-widest text-neutral-400 transition-colors hover:text-white hover:border-neutral-500"
					>
						Go to Search
					</button>
				</div>
			{:else}
				<div class="overflow-x-auto border border-neutral-800 bg-black">
					<table class="min-w-full divide-y divide-neutral-800 text-left text-sm">
						<thead class="bg-neutral-950 text-[11px] uppercase tracking-[0.25em] text-neutral-400">
							<tr>
								<th class="px-4 py-3">Title</th>
								<th class="px-4 py-3">Tracker</th>
								<th class="px-4 py-3">Size</th>
								<th class="px-4 py-3">Downloaded</th>
								<th class="px-4 py-3"></th>
							</tr>
						</thead>
						<tbody class="divide-y divide-neutral-900">
							{#each historyItems as item (item.id)}
								<tr class="align-top hover:bg-neutral-950/70">
									<td class="max-w-xl px-4 py-4 text-white">
										<div class="flex items-center gap-2 flex-wrap">
											{#if item.url}
												<a
													href={item.url}
													target="_blank"
													rel="noreferrer"
													class="font-medium leading-6 transition-colors hover:text-neutral-300"
													>{item.title || 'Untitled torrent'}</a
												>
											{:else}
												<p class="font-medium leading-6">{item.title || 'Untitled torrent'}</p>
											{/if}
											{#if item.freeleech}
												<span
													class="inline-flex border border-emerald-700 bg-emerald-50 px-2 py-0.5 text-[10px] uppercase tracking-[0.25em] text-emerald-900 dark:border-emerald-800 dark:bg-transparent dark:text-emerald-300"
													>Freeleech</span
												>
											{/if}
										</div>
									</td>
									<td class="px-4 py-4 text-neutral-300">{item.tracker || 'Unknown'}</td>
									<td class="px-4 py-4 text-neutral-300">{formatBytes(item.size)}</td>
									<td class="px-4 py-4 text-neutral-300">{formatDate(item.downloaded_at)}</td>
									<td class="px-4 py-4">
										<div class="flex items-center gap-2">
											{#if item.download_url}
												<button
													onclick={(e) => recordDownload(item, e)}
													class="inline-flex items-center justify-center text-white transition-colors hover:text-neutral-300"
													aria-label="Re-download"
												>
													<svg class="h-5 w-5" viewBox="0 0 24 24" fill="currentColor">
														<path d="M19 9h-4V3H9v6H5l7 7 7-7zM5 18v2h14v-2H5z" />
													</svg>
												</button>
											{:else}
												<span class="text-xs uppercase tracking-[0.25em] text-neutral-600">N/A</span
												>
											{/if}
											<button
												onclick={() => deleteHistoryItem(item.id)}
												disabled={deletingHistoryId === item.id}
												class="inline-flex items-center justify-center text-neutral-500 transition-colors hover:text-red-400"
												aria-label="Delete"
											>
												<svg class="h-4 w-4" viewBox="0 0 24 24" fill="currentColor">
													<path
														d="M6 19c0 1.1.9 2 2 2h8c1.1 0 2-.9 2-2V7H6v12zM19 4h-3.5l-1-1h-5l-1 1H5v2h14V4z"
													/>
												</svg>
											</button>
										</div>
									</td>
								</tr>
							{/each}
						</tbody>
					</table>
				</div>

				<!-- History Pagination -->
				{#if historyTotalPages > 1}
					<div class="mt-6 flex items-center justify-center gap-2">
						<button
							onclick={() => goToHistoryPage(historyPage - 1)}
							disabled={historyPage === 1}
							class="px-3 py-2 border border-neutral-800 text-xs uppercase tracking-widest text-neutral-400 transition-colors hover:text-white hover:border-neutral-600 disabled:opacity-50 disabled:cursor-not-allowed"
						>
							← Prev
						</button>

						{#each Array.from({ length: Math.min(5, historyTotalPages) }, (_, i) => {
							let start = Math.max(1, Math.min(historyPage - 2, historyTotalPages - 4));
							return start + i;
						}) as pageNum}
							<button
								onclick={() => goToHistoryPage(pageNum)}
								class="px-3 py-2 border text-xs uppercase tracking-widest transition-colors {historyPage ===
								pageNum
									? 'bg-white text-black border-white'
									: 'border-neutral-800 text-neutral-400 hover:text-white hover:border-neutral-600'}"
							>
								{pageNum}
							</button>
						{/each}

						{#if historyTotalPages > 5 && historyPage < historyTotalPages - 2}
							<span class="text-neutral-600">...</span>
							<button
								onclick={() => goToHistoryPage(historyTotalPages)}
								class="px-3 py-2 border border-neutral-800 text-xs uppercase tracking-widest text-neutral-400 transition-colors hover:text-white hover:border-neutral-600"
							>
								{historyTotalPages}
							</button>
						{/if}

						<button
							onclick={() => goToHistoryPage(historyPage + 1)}
							disabled={historyPage === historyTotalPages}
							class="px-3 py-2 border border-neutral-800 text-xs uppercase tracking-widest text-neutral-400 transition-colors hover:text-white hover:border-neutral-600 disabled:opacity-50 disabled:cursor-not-allowed"
						>
							Next →
						</button>
					</div>
				{/if}
			{/if}
		</section>
	{/if}
</div>

{#if showAddIndexerPopup}
	<div
		class="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm"
		onclick={() => (showAddIndexerPopup = false)}
	>
		<div
			class="w-full max-w-md border border-neutral-800 bg-black p-6"
			onclick={(e) => e.stopPropagation()}
		>
			<div class="mb-6 flex items-center justify-between">
				<h3 class="text-sm font-semibold uppercase tracking-widest text-white">
					Add Jackett Indexer
				</h3>
				<button
					onclick={() => (showAddIndexerPopup = false)}
					class="text-neutral-500 transition-colors hover:text-white"
					aria-label="Close"
				>
					<svg class="h-5 w-5" viewBox="0 0 24 24" fill="currentColor">
						<path
							d="M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z"
						/>
					</svg>
				</button>
			</div>

			<div class="grid gap-4">
				<label class="grid gap-2">
					<span class="text-xs uppercase tracking-[0.25em] text-neutral-400">Label (optional)</span>
					<input
						bind:value={label}
						type="text"
						placeholder="e.g., Private Tracker"
						class="w-full border border-neutral-800 bg-black px-4 py-3 text-sm outline-none focus:border-neutral-500"
					/>
				</label>

				<label class="grid gap-2">
					<span class="text-xs uppercase tracking-[0.25em] text-neutral-400">Torznab Link</span>
					<input
						bind:value={torznabURL}
						type="url"
						placeholder="https://jackett.local/api/v2.0/indexers/.../results/torznab/api?t=search"
						class="w-full border border-neutral-800 bg-black px-4 py-3 text-sm outline-none focus:border-neutral-500"
					/>
				</label>

				<label class="grid gap-2">
					<span class="text-xs uppercase tracking-[0.25em] text-neutral-400">Jackett API Key</span>
					<input
						bind:value={apiKey}
						type="password"
						placeholder="Paste Jackett API key"
						class="w-full border border-neutral-800 bg-black px-4 py-3 text-sm outline-none focus:border-neutral-500"
					/>
				</label>

				<button
					onclick={addIndexer}
					disabled={savingIndexer}
					class="bg-white px-6 py-3 text-xs font-bold uppercase tracking-widest text-black transition-colors hover:bg-neutral-300 disabled:bg-neutral-800 disabled:text-neutral-500"
				>
					{#if savingIndexer}
						Adding...
					{:else}
						Add Indexer
					{/if}
				</button>
			</div>
		</div>
	</div>
{/if}
