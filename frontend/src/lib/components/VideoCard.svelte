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

<div class="group relative flex flex-col space-y-3 cursor-pointer">
	<a
		{href}
		class="flex flex-col space-y-3"
		onmouseenter={startHover}
		onmouseleave={stopHover}
		onfocus={startHover}
		onblur={stopHover}
		onclick={handleCardClick}
	>
		<div
			class="w-full aspect-video bg-neutral-900 border overflow-hidden relative transition-all duration-300 {selected
				? 'border-white ring-2 ring-white ring-offset-2 ring-offset-black'
				: 'border-neutral-800 group-hover:border-neutral-500'}"
		>
			<img
				src={`/api/video/${video.id}/thumbnail`}
				alt={displayTitle}
				class="w-full h-full object-cover transition-transform duration-700 ease-out {hovered &&
				!previewLoaded
					? 'group-hover:scale-105'
					: ''}"
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
				<div class="absolute top-2 left-2">
					{#if hasMultiple}
						<div class="relative group/resolutions">
							<span
								class="bg-black/80 px-2 py-1 text-[10px] font-mono tracking-wider text-white leading-none flex items-center h-5 cursor-help"
							>
								{firstVariant.quality}
							</span>
							<div class="absolute top-full left-0 mt-1 hidden group-hover/resolutions:block z-10">
								<div
									class="bg-black/90 border border-neutral-700 px-2 py-1.5 text-[10px] font-mono text-white whitespace-nowrap"
								>
									{displayVariants.map((variant) => variant.quality).join(', ')}
								</div>
							</div>
						</div>
					{:else}
						<span
							class="bg-black/80 px-2 py-1 text-[10px] font-mono tracking-wider text-white leading-none flex items-center h-5"
						>
							{firstVariant.quality}
						</span>
					{/if}
				</div>
			{/if}

			{#if selectable}
				<button
					type="button"
					class="absolute right-2 top-2 z-10 flex h-7 w-7 items-center justify-center border transition-all {selected
						? 'border-white bg-black text-white hover:bg-black'
						: 'border-white/40 bg-black/80 text-white hover:border-white hover:bg-black'}"
				onclick={handleSelectionButtonClick}
				aria-label={selected ? `Deselect ${displayTitle}` : `Select ${displayTitle}`}
				>
					{#if selected}
						<svg
							class="w-5 h-5 flex-shrink-0"
							viewBox="0 0 24 24"
							fill="currentColor"
							aria-hidden="true"
						>
							<path d="M9 16.17L4.83 12l-1.41 1.41L9 19 21 7l-1.41-1.41z" />
						</svg>
					{:else}
						<span class="text-sm font-mono leading-none">+</span>
					{/if}
				</button>
			{/if}

			<div
				class="absolute bottom-2 right-2 bg-black/80 px-2 py-1 text-[10px] font-mono tracking-wider text-white"
			>
				{formatDuration(video.duration)}
			</div>

			{#if hovered && !previewLoaded && previewRequested && !previewFailed}
				<div
					class="absolute inset-0 flex items-center justify-center bg-black/40 text-[10px] uppercase tracking-[0.25em] text-white/70 backdrop-blur-sm"
				>
					Loading Preview
				</div>
			{/if}
		</div>

		<div class="flex flex-col space-y-1 px-1">
		<h3
			class="text-white text-sm font-medium leading-snug group-hover:text-gray-300 line-clamp-2 transition-colors"
		>
			{displayTitle}
		</h3>
			<p class="text-neutral-600 text-xs tracking-wider uppercase">
				{formatDate(video.date_added)}
			</p>
		</div>
	</a>
</div>
