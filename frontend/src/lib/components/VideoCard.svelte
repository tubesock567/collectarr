<script>
	import { preferences } from '$lib/preferences';

	let {
		video,
		selectable = false,
		selected = false,
		onToggleSelect = () => {}
	} = $props();

	let previewEl = $state(null);
	let hovered = $state(false);
	let previewRequested = $state(false);
	let previewLoaded = $state(false);
	let previewFailed = $state(false);

	const validVariants = $derived(video.variants?.filter((variant) => variant.quality !== null && variant.quality !== undefined) || []);
	const displayVariants = $derived(validVariants.map((variant) => ({ ...variant, quality: variant.quality || 'Original' })));
	const hasMultiple = $derived(displayVariants.length > 1);
	const firstVariant = $derived(displayVariants[0]);
	const hoverPreviewSrc = $derived(`/api/video/${video.id}/hover-preview`);

	function startHover() {
		if ($preferences.incognito) return;
		hovered = true;
		previewRequested = true;
		if (previewLoaded && previewEl) {
			previewEl.currentTime = 0;
			previewEl.play().catch(() => {});
		}
	}

	function stopHover() {
		hovered = false;
		if (previewEl) {
			previewEl.pause();
			previewEl.currentTime = 0;
		}
	}

	function handlePreviewReady() {
		previewLoaded = true;
		if (hovered && previewEl) {
			previewEl.currentTime = 0;
			previewEl.play().catch(() => {});
		}
	}

	function handlePreviewError() {
		previewFailed = true;
	}

	function handleSelectionToggle(event) {
		event.preventDefault();
		event.stopPropagation();
		onToggleSelect(video.id);
	}
</script>

<div class="group relative flex flex-col space-y-3 cursor-pointer">
	<a
		href={`/player/${video.id}`}
		class="flex flex-col space-y-3"
		onmouseenter={startHover}
		onmouseleave={stopHover}
		onfocus={startHover}
		onblur={stopHover}
	>
	<div class="w-full aspect-video bg-neutral-900 border overflow-hidden relative transition-colors duration-300 {selected ? 'border-white' : 'border-neutral-800 group-hover:border-neutral-500'}">
		<img
			src={`/api/video/${video.id}/thumbnail`}
			alt={video.title}
			class="w-full h-full object-cover transition-transform duration-700 ease-out {hovered && !previewLoaded ? 'group-hover:scale-105' : ''}"
			onerror={(e) => { e.target.style.display = 'none'; }}
		/>

		{#if previewRequested && !previewFailed}
			<video
				bind:this={previewEl}
				src={hoverPreviewSrc}
				muted
				playsinline
				preload="metadata"
				data-hover-preview="true"
				class="absolute inset-0 h-full w-full object-cover transition-opacity duration-300 {previewLoaded && hovered ? 'opacity-100' : 'opacity-0'}"
				onloadeddata={handlePreviewReady}
				onerror={handlePreviewError}
			></video>
		{/if}

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
								{displayVariants.map((variant) => variant.quality).join(', ')}
							</div>
						</div>
					</div>
				{/if}
			</div>
		{/if}

		{#if selectable}
			<button
				type="button"
				class="absolute right-2 top-2 z-10 flex h-7 min-w-7 items-center justify-center border border-white/25 bg-black/80 px-2 text-[11px] font-mono leading-none text-white transition-colors hover:border-white/60 hover:bg-black {selected ? 'border-white bg-white text-black hover:bg-white' : ''}"
				onclick={handleSelectionToggle}
				aria-label={selected ? `Deselect ${video.title}` : `Select ${video.title}`}
			>
				{selected ? '✓' : '+'}
			</button>
		{/if}

		<div class="absolute bottom-2 right-2 bg-black/80 px-2 py-1 text-[10px] font-mono tracking-wider text-white">
			{formatDuration(video.duration)}
		</div>

		{#if hovered && !previewLoaded && previewRequested && !previewFailed}
			<div class="absolute inset-0 flex items-center justify-center bg-black/40 text-[10px] uppercase tracking-[0.25em] text-white/70 backdrop-blur-sm">
				Loading Preview
			</div>
		{/if}
	</div>

	<div class="flex flex-col space-y-1 px-1">
		<h3 class="text-white text-sm font-medium leading-snug group-hover:text-gray-300 line-clamp-2 transition-colors">{video.title}</h3>
		<p class="text-neutral-600 text-xs tracking-wider uppercase">{formatDate(video.date_added)}</p>
	</div>
	</a>
</div>

<script module>
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
