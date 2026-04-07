<script>
	import { onMount } from 'svelte';
	import { authFetch } from '$lib/auth';
	import VideoCard from '$lib/components/VideoCard.svelte';

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
