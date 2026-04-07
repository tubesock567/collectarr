<script>
	import { onMount } from 'svelte';
	import { authFetch } from '$lib/auth';
	import VideoCard from '$lib/components/VideoCard.svelte';
	import { preferences } from '$lib/preferences';
	import { theme } from '$lib/theme';

	let videos = $state([]);
	let loading = $state(true);
	let error = $state(null);

	async function readError(res, fallback) {
		try {
			const data = await res.json();
			return data?.error || fallback;
		} catch {
			return fallback;
		}
	}

	onMount(async () => {
		try {
			const res = await authFetch('/api/videos');
			if (!res.ok) throw new Error(await readError(res, 'Failed to fetch videos'));
			videos = await res.json();
		} catch (err) {
			error = err.message;
		} finally {
			loading = false;
		}
	});
</script>

<svelte:head>
	<title>Collectarr - Library</title>
</svelte:head>

<div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
	<div class="flex justify-between items-center gap-3 mb-6">
		<h2 class="text-sm font-semibold uppercase tracking-widest text-white">Recently added</h2>
		<div class="flex items-center gap-3">
		<button
			class="p-2 rounded border border-neutral-700 hover:border-neutral-500 transition-colors text-neutral-400 hover:text-white"
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
			class="p-2 rounded border border-neutral-700 hover:border-neutral-500 transition-colors text-neutral-400 hover:text-white"
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
	{:else}
		<div class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-6">
			{#each videos as video (video.id)}
				<VideoCard {video} />
			{/each}
		</div>
	{/if}
</div>
