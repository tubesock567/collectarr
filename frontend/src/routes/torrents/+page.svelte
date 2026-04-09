<script>
	import { authFetch } from '$lib/auth';
	import { onMount } from 'svelte';

	let indexers = $state([]);
	let indexersLoading = $state(true);
	let indexerMessage = $state('');
	let savingIndexer = $state(false);
	let deletingIndexerId = $state('');
	let torznabURL = $state('');
	let apiKey = $state('');
	let showAddIndexerPopup = $state(false);

	let query = $state('');
	let searching = $state(false);
	let searchMessage = $state('');
	let searchedQuery = $state('');
	let results = $state([]);
	let warnings = $state([]);

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

	onMount(async () => {
		await loadIndexers();
	});

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
					torznab_url: trimmedURL,
					api_key: trimmedKey
				})
			});

			if (!response.ok) {
				throw new Error(await readError(response, 'Failed to save torrent indexer'));
			}

			indexers = await response.json();
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
		warnings = [];

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
			const response = await authFetch(`/api/torrents/search?${params.toString()}`);
			if (!response.ok) {
				throw new Error(await readError(response, 'Torrent search failed'));
			}
			const data = await response.json();
			results = data?.results || [];
			warnings = data?.warnings || [];
			searchMessage = results.length > 0 ? `Found ${results.length} results.` : 'No torrent results found.';
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
</script>

<svelte:head>
	<title>Collectarr - Torrents</title>
</svelte:head>

<div class="mx-auto max-w-7xl px-4 py-12 sm:px-6 lg:px-8">
	<div class="mb-8 border-b border-neutral-800 pb-4">
		<h1 class="text-2xl font-bold uppercase tracking-widest">Torrent Search</h1>
		<p class="mt-2 max-w-3xl text-sm text-neutral-500">Add Jackett Torznab feeds, then search across configured indexers. Results show tracker, size, seeders, leechers, freeleech, and download link.</p>
	</div>

	<div class="grid gap-8 xl:grid-cols-[20rem_minmax(0,1fr)]">
		<section class="border border-neutral-800 p-6">
			<div class="flex items-center justify-between">
				<div>
					<h2 class="text-sm font-semibold uppercase tracking-widest text-white">Jackett Indexers</h2>
					<p class="mt-1 text-xs text-neutral-500">Configured indexers: {indexers.length}</p>
				</div>
				<button onclick={() => showAddIndexerPopup = true} class="flex h-8 w-8 items-center justify-center border border-neutral-700 bg-neutral-900 text-white transition-colors hover:border-neutral-500 hover:bg-neutral-800" aria-label="Add indexer">
					<svg class="h-4 w-4" viewBox="0 0 24 24" fill="currentColor">
						<path d="M19 13h-6v6h-2v-6H5v-2h6V5h2v6h6v2z"/>
					</svg>
				</button>
			</div>

			{#if indexerMessage}
				<p class="mt-4 text-xs tracking-wide {indexerMessage.startsWith('Error') ? 'text-red-500' : 'text-neutral-400'}">{indexerMessage}</p>
			{/if}

			<div class="mt-6">
				{#if indexersLoading}
					<div class="flex min-h-32 items-center justify-center border border-neutral-800 bg-black">
						<span class="loading loading-spinner loading-md text-white"></span>
					</div>
				{:else if indexers.length === 0}
					<div class="border border-neutral-800 bg-black px-4 py-5 text-sm text-neutral-500">No Jackett indexers saved yet. Click + to add one.</div>
				{:else}
					<div class="space-y-3">
						{#each indexers as indexer (indexer.id)}
							<div class="border border-neutral-800 bg-black p-4">
								<div class="flex items-start justify-between gap-4">
									<div class="min-w-0 space-y-2">
										<p class="text-xs font-semibold uppercase tracking-[0.3em] text-white">{indexer.tracker}</p>
										<p class="break-all text-xs text-neutral-500">{indexer.torznab_url}</p>
										<p class="text-[11px] uppercase tracking-[0.25em] text-neutral-600">API Key {maskAPIKey(indexer.masked_api_key)}</p>
									</div>
									<button onclick={() => removeIndexer(indexer.id)} disabled={deletingIndexerId === indexer.id} class="border border-red-900 px-3 py-2 text-[11px] font-semibold uppercase tracking-[0.25em] text-red-300 transition-colors hover:bg-red-950 disabled:border-neutral-800 disabled:text-neutral-500">
										{deletingIndexerId === indexer.id ? 'Removing...' : 'Remove'}
									</button>
								</div>
							</div>
						{/each}
					</div>
				{/if}
			</div>
		</section>

		<section class="space-y-6">
			<section class="border border-neutral-800 p-6">
				<div class="flex flex-col gap-6 lg:flex-row lg:items-end lg:justify-between">
					<div>
						<h2 class="text-sm font-semibold uppercase tracking-widest text-white">Search Torrents</h2>
						<p class="mt-1 text-xs text-neutral-500">Search all saved indexers in parallel through Jackett Torznab.</p>
					</div>

					{#if searchedQuery}
						<p class="text-xs uppercase tracking-[0.25em] text-neutral-500">Last query {searchedQuery}</p>
					{/if}
				</div>

				<form class="mt-6 grid gap-4 lg:grid-cols-[minmax(0,1fr)_auto]" onsubmit={handleSearchSubmit}>
					<input bind:value={query} type="search" placeholder="Search torrents..." class="w-full border border-neutral-800 bg-black px-4 py-3 text-sm outline-none focus:border-neutral-500" />
					<button type="submit" disabled={searching} class="bg-white px-6 py-3 text-xs font-bold uppercase tracking-widest text-black transition-colors hover:bg-neutral-300 disabled:bg-neutral-800 disabled:text-neutral-500">
						{#if searching}
							Searching...
						{:else}
							Search
						{/if}
					</button>
				</form>

				{#if searchMessage}
					<p class="mt-4 text-xs tracking-wide {searchMessage.startsWith('Error') ? 'text-red-500' : 'text-neutral-400'}">{searchMessage}</p>
				{/if}

				{#if warnings.length > 0}
					<div class="mt-4 space-y-2 border border-amber-900 bg-amber-950/20 p-4 text-xs text-amber-300">
						<p class="uppercase tracking-[0.25em] text-amber-400">Indexer warnings</p>
						{#each warnings as warning}
							<p>{warning}</p>
						{/each}
					</div>
				{/if}
			</section>

			<section class="border border-neutral-800 p-6">
				<div class="flex items-center justify-between gap-4">
					<div>
						<h2 class="text-sm font-semibold uppercase tracking-widest text-white">Results</h2>
						<p class="mt-1 text-xs text-neutral-500">Sorted by seeders descending across all configured trackers.</p>
					</div>
					{#if results.length > 0}
						<p class="text-xs uppercase tracking-[0.25em] text-neutral-500">{results.length} results</p>
					{/if}
				</div>

				{#if searching && results.length === 0}
					<div class="mt-6 flex min-h-64 items-center justify-center border border-neutral-800 bg-black">
						<span class="loading loading-spinner loading-lg text-white"></span>
					</div>
				{:else if results.length === 0}
					<div class="mt-6 flex min-h-64 items-center justify-center border border-neutral-800 bg-black px-6 text-center text-xs uppercase tracking-widest text-neutral-500">
						{searchedQuery ? 'No torrent results for current query.' : 'Run search to see torrent results.'}
					</div>
				{:else}
					<div class="mt-6 overflow-x-auto border border-neutral-800 bg-black">
						<table class="min-w-full divide-y divide-neutral-800 text-left text-sm">
							<thead class="bg-neutral-950 text-[11px] uppercase tracking-[0.25em] text-neutral-400">
								<tr>
									<th class="px-4 py-3">Title</th>
									<th class="px-4 py-3">Tracker</th>
									<th class="px-4 py-3">Size</th>
									<th class="px-4 py-3">Seeders</th>
									<th class="px-4 py-3">Leechers</th>
									<th class="px-4 py-3"></th>
								</tr>
							</thead>
							<tbody class="divide-y divide-neutral-900">
								{#each results as result, index (`${result.tracker}-${result.download_url || result.url || result.title}-${index}`)}
									<tr class="align-top hover:bg-neutral-950/70">
										<td class="max-w-xl px-4 py-4 text-white">
											<div class="space-y-2">
												<div class="flex items-center gap-2">
													{#if result.url}
														<a href={result.url} target="_blank" rel="noreferrer" class="font-medium leading-6 transition-colors hover:text-neutral-300">{result.title || 'Untitled torrent'}</a>
													{:else}
														<p class="font-medium leading-6">{result.title || 'Untitled torrent'}</p>
													{/if}
													{#if result.freeleech}
														<span class="inline-flex border border-emerald-800 px-2 py-0.5 text-[10px] uppercase tracking-[0.25em] text-emerald-300">Freeleech</span>
													{/if}
												</div>
											</div>
										</td>
										<td class="px-4 py-4 text-neutral-300">{result.tracker || 'Unknown'}</td>
										<td class="px-4 py-4 text-neutral-300">{formatBytes(result.size)}</td>
										<td class="px-4 py-4 text-neutral-300">{result.seeders ?? 0}</td>
										<td class="px-4 py-4 text-neutral-300">{result.leechers ?? 0}</td>
										<td class="px-4 py-4">
											{#if result.download_url}
												<a href={result.download_url} download class="inline-flex items-center justify-center text-white transition-colors hover:text-neutral-300" aria-label="Download">
													<svg class="h-5 w-5" viewBox="0 0 24 24" fill="currentColor">
														<path d="M19 9h-4V3H9v6H5l7 7 7-7zM5 18v2h14v-2H5z"/>
													</svg>
												</a>
											{:else}
												<span class="text-xs uppercase tracking-[0.25em] text-neutral-600">N/A</span>
											{/if}
										</td>
									</tr>
								{/each}
							</tbody>
						</table>
					</div>
				{/if}
			</section>
		</section>
	</div>
</div>

{#if showAddIndexerPopup}
	<div class="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm" onclick={() => showAddIndexerPopup = false}>
		<div class="w-full max-w-md border border-neutral-800 bg-black p-6" onclick={(e) => e.stopPropagation()}>
			<div class="mb-6 flex items-center justify-between">
				<h3 class="text-sm font-semibold uppercase tracking-widest text-white">Add Jackett Indexer</h3>
				<button onclick={() => showAddIndexerPopup = false} class="text-neutral-500 transition-colors hover:text-white" aria-label="Close">
					<svg class="h-5 w-5" viewBox="0 0 24 24" fill="currentColor">
						<path d="M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z"/>
					</svg>
				</button>
			</div>

			<div class="grid gap-4">
				<label class="grid gap-2">
					<span class="text-xs uppercase tracking-[0.25em] text-neutral-400">Torznab Link</span>
					<input bind:value={torznabURL} type="url" placeholder="https://jackett.local/api/v2.0/indexers/.../results/torznab/api?t=search" class="w-full border border-neutral-800 bg-black px-4 py-3 text-sm outline-none focus:border-neutral-500" />
				</label>

				<label class="grid gap-2">
					<span class="text-xs uppercase tracking-[0.25em] text-neutral-400">Jackett API Key</span>
					<input bind:value={apiKey} type="password" placeholder="Paste Jackett API key" class="w-full border border-neutral-800 bg-black px-4 py-3 text-sm outline-none focus:border-neutral-500" />
				</label>

				<button onclick={addIndexer} disabled={savingIndexer} class="bg-white px-6 py-3 text-xs font-bold uppercase tracking-widest text-black transition-colors hover:bg-neutral-300 disabled:bg-neutral-800 disabled:text-neutral-500">
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
