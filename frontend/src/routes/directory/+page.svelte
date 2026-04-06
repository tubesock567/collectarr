<script>
	import { onMount } from 'svelte';
	import { authFetch } from '$lib/auth';

	const VIDEO_EXTENSIONS = new Set(['.mp4', '.mkv', '.avi', '.mov', '.webm']);

	let currentPath = $state('');
	let entries = $state([]);
	let selectedFiles = $state(new Set());
	let loading = $state(false);
	let destination = $state('');
	let message = $state('');
	let linking = $state(false);
	let directoryRequestId = 0;

	function normalizePath(path) {
		if (!path || path === '.') return '';
		const cleaned = String(path).replace(/\\/g, '/').replace(/\/+/g, '/');
		if (cleaned === '/') return '/';
		return cleaned.endsWith('/') && cleaned.length > 1 ? cleaned.slice(0, -1) : cleaned;
	}

	function splitPath(path) {
		return normalizePath(path)
			.split('/')
			.filter(Boolean);
	}

	function joinPath(base, name) {
		const normalizedBase = normalizePath(base);
		if (!normalizedBase || normalizedBase === '/') {
			return normalizedBase === '/' ? `/${name}` : name;
		}
		return `${normalizedBase}/${name}`;
	}

	function getParentPath(path) {
		const normalized = normalizePath(path);
		const parts = splitPath(normalized);
		if (parts.length === 0) return normalized === '/' ? '/' : '';
		if (normalized.startsWith('/')) {
			return parts.length === 1 ? '/' : `/${parts.slice(0, -1).join('/')}`;
		}
		return parts.slice(0, -1).join('/');
	}

	function getBreadcrumbs(path) {
		const normalized = normalizePath(path);
		const parts = splitPath(normalized);
		const breadcrumbs = [{ label: 'Root', path: normalized.startsWith('/') ? '/' : '' }];

		let accumulated = normalized.startsWith('/') ? '/' : '';
		for (const part of parts) {
			accumulated = accumulated === '/' ? `/${part}` : accumulated ? `${accumulated}/${part}` : part;
			breadcrumbs.push({ label: part, path: accumulated });
		}

		return breadcrumbs;
	}

	function isVideoFile(name) {
		const extension = name.slice(name.lastIndexOf('.')).toLowerCase();
		return VIDEO_EXTENSIONS.has(extension);
	}

	const breadcrumbs = $derived(getBreadcrumbs(currentPath));

	function formatSize(size) {
		if (!Number.isFinite(size) || size < 0) return '—';
		if (size < 1024) return `${size} B`;
		const units = ['KB', 'MB', 'GB', 'TB'];
		let value = size / 1024;
		let unitIndex = 0;
		while (value >= 1024 && unitIndex < units.length - 1) {
			value /= 1024;
			unitIndex += 1;
		}
		return `${value.toFixed(value >= 100 ? 0 : value >= 10 ? 1 : 2)} ${units[unitIndex]}`;
	}

	async function readError(res, fallback) {
		try {
			const data = await res.json();
			return data?.error || fallback;
		} catch {
			return fallback;
		}
	}

	async function loadDestination() {
		try {
			const res = await authFetch('/api/settings/hardlink-dest', { method: 'GET' });
			if (!res.ok) throw new Error(await readError(res, 'Failed to load hard link destination'));

			const data = await res.json();
			destination = data?.destinationDir || data?.destination || data?.path || '';
		} catch (err) {
			destination = '';
			message = `Error: ${err.message}`;
		}
	}

	async function loadEntries(path) {
		const requestId = ++directoryRequestId;
		loading = true;
		message = '';

		try {
			const res = await authFetch(`/api/directory?path=${encodeURIComponent(path)}`, { method: 'GET' });
			if (!res.ok) throw new Error(await readError(res, 'Failed to load directory'));

			const data = await res.json();
			const rawEntries = Array.isArray(data) ? data : Array.isArray(data?.entries) ? data.entries : [];
			const nextEntries = rawEntries
				.map((entry) => ({
					name: entry?.name || entry?.filename || '',
					path: normalizePath(entry?.path || joinPath(path, entry?.name || entry?.filename || '')),
					isDirectory: Boolean(entry?.isDirectory ?? entry?.is_directory),
					size: Number(entry?.size ?? 0)
				}))
				.filter((entry) => entry.name && entry.path && (entry.isDirectory || isVideoFile(entry.name)))
				.sort((a, b) => {
					if (a.isDirectory !== b.isDirectory) return a.isDirectory ? -1 : 1;
					return a.name.localeCompare(b.name, undefined, { sensitivity: 'base' });
				});

			if (requestId !== directoryRequestId) return;

			entries = nextEntries;
			selectedFiles = new Set();
		} catch (err) {
			if (requestId !== directoryRequestId) return;
			entries = [];
			selectedFiles = new Set();
			message = `Error: ${err.message}`;
		} finally {
			if (requestId === directoryRequestId) {
				loading = false;
			}
		}
	}

	function navigateTo(path) {
		if (loading) return;
		currentPath = normalizePath(path);
	}

	function goUp() {
		navigateTo(getParentPath(currentPath));
	}

	function toggleFile(path, checked) {
		const next = new Set(selectedFiles);
		if (checked) {
			next.add(path);
		} else {
			next.delete(path);
		}
		selectedFiles = next;
	}

	async function linkSelected() {
		if (linking || selectedFiles.size === 0) return;

		linking = true;
		message = '';

		try {
			await loadDestination();
			if (!destination) {
				throw new Error('Hard link destination is not configured');
			}

			const res = await authFetch('/api/hardlink', {
				method: 'POST',
				body: JSON.stringify({
					sourcePaths: [...selectedFiles],
					destinationDir: destination
				})
			});

			if (!res.ok) throw new Error(await readError(res, 'Failed to create hard links'));

			const count = selectedFiles.size;
			selectedFiles = new Set();
			message = `${count} file${count === 1 ? '' : 's'} linked successfully.`;
		} catch (err) {
			message = `Error: ${err.message}`;
		} finally {
			linking = false;
		}
	}

	onMount(() => {
		loadDestination();
	});

	$effect(() => {
		loadEntries(currentPath);
	});
</script>

<svelte:head>
	<title>Collectarr - Directory Viewer</title>
</svelte:head>

<div class="max-w-5xl mx-auto px-4 sm:px-6 lg:px-8 py-12">
	<div class="flex flex-col gap-8">
		<div class="flex flex-col gap-3 border-b border-neutral-800 pb-4">
			<h1 class="text-2xl font-bold uppercase tracking-widest">Directory Viewer</h1>
			<p class="text-xs uppercase tracking-[0.25em] text-neutral-500">Browse source files and create hard links into your configured destination.</p>
		</div>

		<section class="border border-neutral-800 p-6 flex flex-col gap-4">
			<div class="flex flex-col gap-2">
				<span class="text-xs font-semibold uppercase tracking-[0.25em] text-neutral-400">Current Path</span>
				<div class="flex flex-wrap items-center gap-2 text-sm text-neutral-300">
					{#each breadcrumbs as crumb, index (crumb.path || `${crumb.label}-${index}`)}
						<button
							type="button"
							onclick={() => navigateTo(crumb.path)}
							class="hover:text-white transition-colors"
						>
							{crumb.label}
						</button>
						{#if index < breadcrumbs.length - 1}
							<span class="text-neutral-600">/</span>
						{/if}
					{/each}
				</div>
			</div>

			<div class="grid gap-2">
				<span class="text-xs font-semibold uppercase tracking-[0.25em] text-neutral-400">Hard Link Destination</span>
				<p class="border border-neutral-800 bg-neutral-950 px-4 py-3 text-sm text-neutral-300 break-all">
					{destination || 'Not configured in settings.'}
				</p>
			</div>

			<div class="flex flex-wrap items-center gap-3">
				<button
					type="button"
					onclick={linkSelected}
					disabled={linking || selectedFiles.size === 0 || !destination}
					class="bg-white text-black hover:bg-neutral-300 disabled:bg-neutral-800 disabled:text-neutral-500 font-bold uppercase tracking-widest text-xs px-6 py-3 transition-colors flex items-center gap-3"
				>
					{#if linking}
						<span class="loading loading-spinner loading-xs"></span>
						Linking...
					{:else}
						Link Selected
					{/if}
				</button>
				<span class="text-xs uppercase tracking-[0.25em] text-neutral-500">{selectedFiles.size} selected</span>
			</div>

			{#if message}
				<p class="text-xs tracking-wide {message.startsWith('Error') ? 'text-red-500' : 'text-neutral-400'}">
					{message}
				</p>
			{/if}
		</section>

		<section class="border border-neutral-800">
			<div class="grid grid-cols-[minmax(0,1fr)_140px] gap-4 border-b border-neutral-800 px-4 py-3 text-[11px] uppercase tracking-[0.25em] text-neutral-500">
				<span>Name</span>
				<span class="text-right">Size</span>
			</div>

			{#if loading}
				<div class="flex flex-col items-center justify-center gap-4 px-6 py-16 text-neutral-500">
					<span class="loading loading-spinner loading-lg text-white"></span>
					<p class="text-xs uppercase tracking-[0.25em]">Loading directory...</p>
				</div>
			{:else}
				<div class="divide-y divide-neutral-800">
					{#if currentPath && currentPath !== '/'}
						<button
							type="button"
							onclick={goUp}
							class="w-full grid grid-cols-[minmax(0,1fr)_140px] gap-4 px-4 py-4 text-left hover:bg-neutral-950 transition-colors"
						>
							<div class="min-w-0 flex items-center gap-3">
								<span class="text-neutral-500">↩</span>
								<span class="truncate text-sm">..</span>
							</div>
							<span class="text-right text-sm text-neutral-600">—</span>
						</button>
					{/if}

					{#if entries.length === 0}
						<div class="px-6 py-16 text-center text-sm text-neutral-500">No directories or supported video files found.</div>
					{:else}
						{#each entries as entry (entry.path)}
							{#if entry.isDirectory}
								<button
									type="button"
									onclick={() => navigateTo(entry.path)}
									class="w-full grid grid-cols-[minmax(0,1fr)_140px] gap-4 px-4 py-4 text-left hover:bg-neutral-950 transition-colors"
								>
									<div class="min-w-0 flex items-center gap-3">
										<span class="text-neutral-500">▸</span>
										<span class="truncate text-sm text-white">{entry.name}</span>
									</div>
									<span class="text-right text-sm text-neutral-600">Directory</span>
								</button>
							{:else}
								<label class="grid grid-cols-[minmax(0,1fr)_140px] gap-4 px-4 py-4 hover:bg-neutral-950 transition-colors cursor-pointer">
									<div class="min-w-0 flex items-center gap-3">
										<input
											type="checkbox"
											checked={selectedFiles.has(entry.path)}
											onchange={(event) => toggleFile(entry.path, event.currentTarget.checked)}
											class="h-4 w-4 rounded-none border-neutral-700 bg-black text-white focus:ring-0"
										/>
										<div class="min-w-0">
											<p class="truncate text-sm text-white">{entry.name}</p>
											<p class="truncate text-xs text-neutral-600">{entry.path}</p>
										</div>
									</div>
									<span class="text-right text-sm text-neutral-400">{formatSize(entry.size)}</span>
								</label>
							{/if}
						{/each}
					{/if}
				</div>
			{/if}
		</section>
	</div>
</div>
