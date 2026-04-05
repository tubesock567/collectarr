<script>
	import { onMount } from 'svelte';
	import { authFetch } from '$lib/auth';

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
				{@const validVariants = video.variants?.filter(v => v.quality && v.quality !== 'Original') || []}
				{@const hasMultiple = validVariants.length > 1}
				{@const firstVariant = validVariants[0]}
				<a href={`/player/${video.id}`} class="group flex flex-col space-y-3 cursor-pointer">
					<div class="w-full aspect-video bg-neutral-900 border border-neutral-800 overflow-hidden relative group-hover:border-neutral-500 transition-colors duration-300">
						<img 
							src={`/api/video/${video.id}/thumbnail`} 
							alt={video.title} 
							class="w-full h-full object-cover group-hover:scale-105 transition-transform duration-700 ease-out"
							onerror={(e) => { e.target.style.display = 'none'; }}
						/>
						{#if firstVariant}
							<div class="absolute top-2 left-2 flex items-center gap-1">
								<span class="bg-black/80 px-2 py-1 text-[10px] font-mono tracking-wider text-white leading-none flex items-center h-5">
									{firstVariant.quality}
								</span>
								{#if hasMultiple}
									<div class="relative hover:block group/plus">
										<span class="bg-black/80 px-1.5 py-1 text-[10px] font-mono tracking-wider text-white cursor-help leading-none flex items-center justify-center h-5 w-5">+</span>
										<div class="absolute top-full left-0 mt-1 hidden group-hover/plus:block z-10">
											<div class="bg-black/90 border border-neutral-700 px-2 py-1.5 text-[10px] font-mono text-white whitespace-nowrap">
												{validVariants.map(v => v.quality).join(', ')}
											</div>
										</div>
									</div>
								{/if}
							</div>
						{/if}
						<div class="absolute bottom-2 right-2 bg-black/80 px-2 py-1 text-[10px] font-mono tracking-wider text-white">
							{formatDuration(video.duration)}
						</div>
					</div>
					<div class="flex flex-col space-y-1 px-1">
						<h3 class="text-white text-sm font-medium leading-snug group-hover:text-gray-300 line-clamp-2 transition-colors">{video.title}</h3>
						<p class="text-neutral-600 text-xs tracking-wider uppercase">{formatDate(video.date_added)}</p>
					</div>
				</a>
			{/each}
		</div>
	{/if}
</div>

<script context="module">
	function formatDuration(seconds) {
		if (!seconds || isNaN(seconds)) return '0:00';
		const mins = Math.floor(seconds / 60);
		const secs = Math.floor(seconds % 60);
		if (mins >= 60) {
			const hours = Math.floor(mins / 60);
			const remainingMins = mins % 60;
			return `${hours}:${remainingMins.toString().padStart(2, '0')}:${secs.toString().padStart(2, '0')}`;
		}
		return `${mins}:${secs.toString().padStart(2, '0')}`;
	}
	
	function formatDate(dateStr) {
		if (!dateStr) return 'Unknown date';
		const date = new Date(dateStr);
		const now = new Date();
		const diff = now - date;
		const days = Math.floor(diff / (1000 * 60 * 60 * 24));
		
		if (days === 0) return 'Today';
		if (days === 1) return 'Yesterday';
		if (days < 7) return `${days} days ago`;
		if (days < 30) return `${Math.floor(days / 7)} weeks ago`;
		if (days < 365) return `${Math.floor(days / 30)} months ago`;
		return `${Math.floor(days / 365)} years ago`;
	}
</script>
