<script>
	import { authFetch } from '$lib/auth';

	let { 
		isOpen = $bindable(false),
		onSelect = () => {},
		onCancel = () => {},
		endpoint = '/api/directory',
		title = 'Select Directory'
	} = $props();

	let currentPath = $state('');
	let entries = $state([]);
	let loading = $state(false);
	let message = $state('');
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

	const breadcrumbs = $derived(getBreadcrumbs(currentPath));

	async function readError(res, fallback) {
		try {
			const data = await res.json();
			return data?.error || fallback;
		} catch {
			return fallback;
		}
	}

	async function loadEntries(path) {
		const requestId = ++directoryRequestId;
		loading = true;
		message = '';

		try {
			const res = await authFetch(`${endpoint}?path=${encodeURIComponent(path)}`, { method: 'GET' });
			if (!res.ok) throw new Error(await readError(res, 'Failed to load directory'));

			const data = await res.json();
			const rawEntries = Array.isArray(data) ? data : Array.isArray(data?.entries) ? data.entries : [];
			const nextEntries = rawEntries
				.map((entry) => ({
					name: entry?.name || entry?.filename || '',
					path: normalizePath(entry?.path || joinPath(path, entry?.name || entry?.filename || '')),
					isDirectory: Boolean(entry?.isDirectory ?? entry?.is_directory)
				}))
				.filter((entry) => entry.name && entry.path && entry.isDirectory)
				.sort((a, b) => a.name.localeCompare(b.name, undefined, { sensitivity: 'base' }));

			if (requestId !== directoryRequestId) return;

			entries = nextEntries;
		} catch (err) {
			if (requestId !== directoryRequestId) return;
			entries = [];
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

	function selectCurrent() {
		onSelect(currentPath);
		isOpen = false;
	}

	function cancel() {
		onCancel();
		isOpen = false;
	}

	$effect(() => {
		if (isOpen) {
			loadEntries(currentPath);
		}
	});
</script>

{#if isOpen}
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div 
		class="fixed inset-0 bg-black/80 z-50 flex items-center justify-center p-4"
		onclick={(e) => { if (e.target === e.currentTarget) cancel(); }}
	>
		<div class="bg-black border border-neutral-800 w-full max-w-2xl max-h-[80vh] flex flex-col">
			<div class="border-b border-neutral-800 p-4">
				<h2 class="text-sm font-semibold uppercase tracking-widest text-white">{title}</h2>
			</div>

			<div class="p-4 border-b border-neutral-800">
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

			<div class="flex-1 overflow-auto">
				{#if loading}
					<div class="flex flex-col items-center justify-center gap-4 px-6 py-16 text-neutral-500">
						<span class="loading loading-spinner loading-lg text-white"></span>
						<p class="text-xs uppercase tracking-[0.25em]">Loading...</p>
					</div>
				{:else}
					<div class="divide-y divide-neutral-800">
						{#if currentPath && currentPath !== '/'}
							<button
								type="button"
								onclick={goUp}
								class="w-full px-4 py-3 text-left hover:bg-neutral-950 transition-colors"
							>
								<span class="text-neutral-500">↩</span>
								<span class="ml-2 text-sm">..</span>
							</button>
						{/if}

						{#if entries.length === 0}
							<div class="px-6 py-8 text-center text-sm text-neutral-500">No directories found.</div>
						{:else}
							{#each entries as entry (entry.path)}
								<button
									type="button"
									onclick={() => navigateTo(entry.path)}
									class="w-full px-4 py-3 text-left hover:bg-neutral-950 transition-colors flex items-center gap-3"
								>
									<span class="text-neutral-500">▸</span>
									<span class="text-sm text-white">{entry.name}</span>
								</button>
							{/each}
						{/if}
					</div>
				{/if}
			</div>

			{#if message}
				<div class="px-4 py-2 border-t border-neutral-800">
					<p class="text-xs tracking-wide {message.startsWith('Error') ? 'text-red-500' : 'text-neutral-400'}">
						{message}
					</p>
				</div>
			{/if}

			<div class="border-t border-neutral-800 p-4 flex justify-between items-center">
				<div class="text-xs text-neutral-500">
					Selected: <span class="text-white">{currentPath || 'Root'}</span>
				</div>
				<div class="flex gap-3">
					<button
						type="button"
						onclick={cancel}
						class="px-4 py-2 text-xs uppercase tracking-widest font-semibold text-neutral-400 hover:text-white transition-colors"
					>
						Cancel
					</button>
					<button
						type="button"
						onclick={selectCurrent}
						class="bg-white text-black hover:bg-neutral-300 font-bold uppercase tracking-widest text-xs px-6 py-2 transition-colors"
					>
						Select
					</button>
				</div>
			</div>
		</div>
	</div>
{/if}
