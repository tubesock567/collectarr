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

	function scrambleText(text) {
		if (!text) return '';
		const chars = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789!@#$%^&*';
		let scrambled = '';
		for (let i = 0; i < text.length; i++) {
			scrambled += chars[Math.floor(Math.random() * chars.length)];
		}
		return scrambled;
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

<script>
	import { preferences } from '$lib/preferences';

	let {
		video,
		selectable = false,
		selected = false,
		playlistId = null,
		onToggleSelect = () => {}
	} = $props();

	let previewEl = $state(null);
	let hovered = $state(false);
	let previewRequested = $state(false);
	let previewLoaded = $state(false);
	let previewFailed = $state(false);

	const validVariants = $derived(
		video.variants?.filter(
			(variant) => variant.quality !== null && variant.quality !== undefined
		) || []
	);
	const displayVariants = $derived(
		validVariants.map((variant) => ({ ...variant, quality: variant.quality || 'Original' }))
	);
	const hasMultiple = $derived(displayVariants.length > 1);
	const firstVariant = $derived(displayVariants[0]);
	const hoverPreviewSrc = $derived(`/api/video/${video.id}/hover-preview`);
	const href = $derived(
		playlistId ? `/player/${video.id}?playlist=${playlistId}` : `/player/${video.id}`
	);
	const displayTitle = $derived($preferences.incognito ? scrambleText(video.title) : video.title);

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

	function handleCardClick(event) {
		if (selectable) {
			event.preventDefault();
			onToggleSelect(video.id);
		}
	}

	function handleSelectionButtonClick(event) {
		event.preventDefault();
		event.stopPropagation();
		onToggleSelect(video.id);
	}
</script>

<div
	class="mono-panel-hover group relative flex flex-col space-y-2 cursor-pointer rounded-[10px] p-2.5 bg-transparent transition-colors duration-300 {selected
		? 'mono-panel-soft'
		: ''}"
>
	<a
		{href}
		class="flex flex-col space-y-2"
		onmouseenter={startHover}
		onmouseleave={stopHover}
		onfocus={startHover}
		onblur={stopHover}
		onclick={handleCardClick}
	>
		<div
			class="w-full aspect-video overflow-hidden rounded-[8px] bg-[#1b1c1f] shadow-[inset_0_1px_0_rgba(255,255,255,0.03)] relative transition-colors duration-300 {selected
				? 'shadow-[0_0_0_1px_rgba(255,255,255,0.08)]'
				: 'group-hover:bg-[#212226]'}"
		>
			<img
				src={`/api/video/${video.id}/thumbnail`}
				alt={displayTitle}
				class="w-full h-full object-cover grayscale opacity-80 group-hover:grayscale-0 group-hover:opacity-100 transition-all duration-500"
				onerror={(e) => {
					e.target.style.display = 'none';
				}}
			/>

			{#if previewRequested && !previewFailed}
				<video
					bind:this={previewEl}
					src={hoverPreviewSrc}
					muted
					playsinline
					preload="metadata"
					data-hover-preview="true"
					class="absolute inset-0 h-full w-full object-cover transition-opacity duration-300 {previewLoaded &&
					hovered
						? 'opacity-100'
						: 'opacity-0'}"
					onloadeddata={handlePreviewReady}
					onerror={handlePreviewError}
				></video>
			{/if}

			{#if firstVariant}
				<div class="mono-chip absolute top-2 left-2 flex items-center rounded-[4px]">
					{#if hasMultiple}
						<div class="relative group/resolutions">
							<span
								class="px-1.5 py-0.5 text-[9px] font-bold tracking-[0.2em] text-neutral-400 leading-none flex items-center cursor-help transition-colors group-hover/resolutions:text-white"
							>
								{firstVariant.quality}
							</span>
							<div class="absolute top-full left-0 hidden group-hover/resolutions:block z-10 pt-1">
								<div
									class="mono-chip rounded-[4px] px-2 py-1 text-[9px] font-bold tracking-widest text-white whitespace-nowrap"
								>
									{displayVariants.map((variant) => variant.quality).join(' / ')}
								</div>
							</div>
						</div>
					{:else}
						<span
							class="px-1.5 py-0.5 text-[9px] font-bold tracking-[0.2em] text-neutral-400 leading-none flex items-center"
						>
							{firstVariant.quality}
						</span>
					{/if}
				</div>
			{/if}

			{#if selectable}
				<button
					type="button"
					class="absolute right-2 top-2 z-10 flex h-7 w-7 items-center justify-center rounded-[4px] mono-chip transition-colors {selected
						? 'bg-white text-black hover:bg-neutral-200'
						: 'text-neutral-400 hover:text-white'}"
					onclick={handleSelectionButtonClick}
					aria-label={selected ? `Deselect ${displayTitle}` : `Select ${displayTitle}`}
				>
					{#if selected}
						<svg
							class="w-3.5 h-3.5 flex-shrink-0"
							viewBox="0 0 24 24"
							fill="currentColor"
							aria-hidden="true"
						>
							<path d="M9 16.17L4.83 12l-1.41 1.41L9 19 21 7l-1.41-1.41z" />
						</svg>
					{:else}
						<svg
							class="w-3.5 h-3.5 flex-shrink-0 opacity-0 group-hover:opacity-100 transition-opacity"
							viewBox="0 0 24 24"
							fill="currentColor"
							aria-hidden="true"
						>
							<path d="M19 13h-6v6h-2v-6H5v-2h6V5h2v6h6v2z" />
						</svg>
					{/if}
				</button>
			{/if}

			<div
				class="mono-chip absolute bottom-2 right-2 rounded-[4px] px-1.5 py-0.5 text-[9px] font-bold tracking-[0.2em] text-neutral-300 group-hover:text-white transition-colors"
			>
				{formatDuration(video.duration)}
			</div>

			{#if hovered && !previewLoaded && previewRequested && !previewFailed}
				<div
					class="absolute inset-0 flex items-center justify-center bg-black/60 text-[9px] font-bold uppercase tracking-[0.3em] text-neutral-300 backdrop-blur-[2px]"
				>
					LOADING_
				</div>
			{/if}
		</div>

		<div class="flex flex-col space-y-1.5 px-0.5 pb-0.5">
			<div class="flex justify-between items-start gap-2">
				<h3
					class="text-neutral-300 text-[11px] font-bold uppercase tracking-wide leading-snug group-hover:text-white line-clamp-2 transition-colors break-words"
				>
					{displayTitle}
				</h3>
			</div>
			<p
				class="text-neutral-600 text-[9px] font-bold tracking-[0.2em] uppercase pt-1 mt-1 group-hover:text-neutral-500 transition-colors"
			>
				{formatDate(video.date_added)}
			</p>
		</div>
	</a>
</div>
