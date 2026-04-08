<script>
	import { authFetch } from '$lib/auth';
	import MetadataTokenInput from '$lib/components/MetadataTokenInput.svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { onMount, onDestroy } from 'svelte';
	
	let containerEl = $state(null);
	let videoEl = $state(null);
	let video = $state(null);
	let selectedVariantId = $state(null);
	let loading = $state(true);
	let loadError = $state(null);
	let currentTime = $state(0);
	let duration = $state(0);
	let paused = $state(true);
	let volume = $state(1);
	let muted = $state(false);
	let previewLoading = $state(false);
	let previewData = $state(null);
	let previewError = $state(null);
	let hoverPercent = $state(0);
	let hoverTime = $state(0);
	let hoverPreviewIndex = $state(-1);
	let showingPreview = $state(false);
	let showInfoPanel = $state(false);
	let tagsDraft = $state([]);
	let actorsDraft = $state([]);
	let metadataOptions = $state({ tags: [], actors: [] });
	let savingMetadata = $state(false);
	let metadataMessage = $state('');
	
	let showControls = $state(true);
	let hideTimer = null;
	
	let id = $derived($page.params.id);
	const playlistId = $derived($page.url.searchParams.get('playlist'));
	let playlist = $state(null);
	let nextVideoId = $state(null);
	let prevVideoId = $state(null);
	
	function resetTimer() {
		showControls = true;
		if (hideTimer) clearTimeout(hideTimer);
		hideTimer = setTimeout(() => {
			if (!paused) showControls = false;
		}, 3000);
	}

	function togglePlay() {
		if (videoEl) {
			if (paused) {
				videoEl.play();
			} else {
				videoEl.pause();
			}
		}
	}

	function handleVideoSurfaceClick() {
		if (!showControls && !paused) {
			resetTimer();
			return;
		}

		togglePlay();
		resetTimer();
	}

	function toggleFullscreen() {
		if (!document.fullscreenElement) {
			containerEl?.requestFullscreen();
		} else {
			document.exitFullscreen();
		}
	}

	function toggleInfoPanel() {
		showInfoPanel = !showInfoPanel;
		if (showInfoPanel) {
			showControls = true;
			if (hideTimer) clearTimeout(hideTimer);
		} else if (!paused) {
			resetTimer();
		}
	}

	function handleKeydown(e) {
		if (loading || loadError) return;
		const target = e.target;
		if (target instanceof HTMLElement) {
			const tagName = target.tagName;
			if (target.isContentEditable || tagName === 'INPUT' || tagName === 'TEXTAREA' || tagName === 'SELECT' || tagName === 'BUTTON') {
				if (e.key === 'Escape' && showInfoPanel) {
					e.preventDefault();
					showInfoPanel = false;
				}
				return;
			}
		}
		resetTimer();
		switch(e.key) {
			case ' ':
			case 'k':
			case 'K':
				e.preventDefault();
				togglePlay();
				break;
			case 'f':
			case 'F':
				e.preventDefault();
				toggleFullscreen();
				break;
			case 'ArrowLeft':
				e.preventDefault();
				if (videoEl) videoEl.currentTime = Math.max(0, currentTime - 5);
				break;
			case 'ArrowRight':
				e.preventDefault();
				if (videoEl) videoEl.currentTime = Math.min(duration, currentTime + 5);
				break;
			case 'j':
			case 'J':
				e.preventDefault();
				if (videoEl) videoEl.currentTime = Math.max(0, currentTime - 30);
				break;
			case 'l':
			case 'L':
				e.preventDefault();
				if (videoEl) videoEl.currentTime = Math.min(duration, currentTime + 30);
				break;
			case ',':
				e.preventDefault();
				if (videoEl && paused) videoEl.currentTime = Math.max(0, currentTime - (1/30));
				break;
			case '.':
				e.preventDefault();
				if (videoEl && paused) videoEl.currentTime = Math.min(duration, currentTime + (1/30));
				break;
			case 'ArrowUp':
				e.preventDefault();
				volume = Math.min(1, volume + 0.1);
				break;
			case 'ArrowDown':
				e.preventDefault();
				volume = Math.max(0, volume - 0.1);
				break;
		case 'm':
		case 'M':
			e.preventDefault();
			muted = !muted;
			break;
		case 'Escape':
			if (showInfoPanel) {
				e.preventDefault();
				showInfoPanel = false;
			} else if (document.fullscreenElement) {
				e.preventDefault();
				document.exitFullscreen();
			} else {
				e.preventDefault();
				goto(playlistId ? `/playlists/${playlistId}` : '/');
			}
			break;
	}
	}
	
	function seek(e) {
		const rect = e.currentTarget.getBoundingClientRect();
		const pos = (e.clientX - rect.left) / rect.width;
		if (videoEl) videoEl.currentTime = pos * duration;
	}

	function seekByKeyboard(event) {
		if (!videoEl) return;
		switch (event.key) {
			case 'ArrowLeft':
				event.preventDefault();
				videoEl.currentTime = Math.max(0, currentTime - 5);
				break;
			case 'ArrowRight':
				event.preventDefault();
				videoEl.currentTime = Math.min(duration, currentTime + 5);
				break;
			case 'Home':
				event.preventDefault();
				videoEl.currentTime = 0;
				break;
			case 'End':
				event.preventDefault();
				videoEl.currentTime = duration;
				break;
		}
	}

	function nearestPreviewIndex(time) {
		if (!previewData?.timestamps?.length) return -1;
		let bestIndex = 0;
		let bestDiff = Math.abs(previewData.timestamps[0] - time);
		for (let i = 1; i < previewData.timestamps.length; i += 1) {
			const diff = Math.abs(previewData.timestamps[i] - time);
			if (diff < bestDiff) {
				bestDiff = diff;
				bestIndex = i;
			}
		}
		return bestIndex;
	}

	function handleSeekHover(e) {
		if (!previewData || !duration) return;
		const rect = e.currentTarget.getBoundingClientRect();
		const pos = Math.min(Math.max((e.clientX - rect.left) / rect.width, 0), 1);
		hoverPercent = pos * 100;
		hoverTime = pos * duration;
		hoverPreviewIndex = nearestPreviewIndex(hoverTime);
		showingPreview = hoverPreviewIndex >= 0;
	}

	function clearSeekHover() {
		showingPreview = false;
		hoverPreviewIndex = -1;
	}

	function formatTime(seconds) {
		if (isNaN(seconds)) return '0:00';
		const mins = Math.floor(seconds / 60);
		const secs = Math.floor(seconds % 60);
		return `${mins}:${secs.toString().padStart(2, '0')}`;
	}

	function formatDate(value) {
		if (!value) return 'Unknown';
		const date = new Date(value);
		if (Number.isNaN(date.getTime())) return 'Unknown';
		return date.toLocaleString();
	}

	function updateSelectedVariant(id) {
		selectedVariantId = Number(id);
	}

	function syncMetadataDrafts(videoData) {
		tagsDraft = [...(videoData?.tags || [])];
		actorsDraft = [...(videoData?.actors || [])];
		metadataMessage = '';
	}

	function handleTagsChange(nextValues) {
		tagsDraft = nextValues;
		metadataMessage = '';
	}

	function handleActorsChange(nextValues) {
		actorsDraft = nextValues;
		metadataMessage = '';
	}

	async function readError(response, fallback) {
		try {
			const data = await response.json();
			return data?.error || fallback;
		} catch {
			return fallback;
		}
	}

	async function loadMetadataOptions() {
		const res = await authFetch('/api/videos/metadata/options');
		if (!res.ok) {
			throw new Error(await readError(res, 'Failed to load metadata options'));
		}
		metadataOptions = await res.json();
	}

	async function saveMetadata() {
		if (!video || savingMetadata) return;
		savingMetadata = true;
		metadataMessage = '';

		try {
			const res = await authFetch(`/api/videos/${video.id}/metadata`, {
				method: 'PUT',
				body: JSON.stringify({
					tags: tagsDraft,
					actors: actorsDraft
				})
			});
			if (!res.ok) throw new Error(await readError(res, 'Failed to save video metadata'));
			video = await res.json();
			syncMetadataDrafts(video);
			metadataMessage = 'Video details updated.';
			try {
				await loadMetadataOptions();
			} catch {
				metadataMessage = 'Video details updated. Suggestions could not be refreshed.';
			}
		} catch (err) {
			metadataMessage = err.message;
		} finally {
			savingMetadata = false;
		}
	}

	async function loadPreviewData(variantId) {
		if (!variantId) return;
		previewLoading = true;
		previewError = null;
		previewData = null;
		clearSeekHover();
		try {
			const res = await authFetch(`/api/video/${variantId}/preview`);
			if (res.status === 501) {
				previewData = null;
				return;
			}
			if (!res.ok) throw new Error('Failed to load preview data');
			const data = await res.json();
			previewData = {
				...data,
				sprite_url: data.sprite_url,
				frameWidth: data.frame_width,
				frameHeight: data.frame_height,
				columns: data.columns,
				rows: data.rows,
				timestamps: data.timestamps || []
			};
		} catch (err) {
			previewError = err.message;
			previewData = null;
		} finally {
			previewLoading = false;
		}
	}

	let videoSrc = $derived(selectedVariantId ? `/api/video/${selectedVariantId}/stream` : '');

	import { untrack } from 'svelte';

	async function loadVideoAndPlaylist(currentId, currentPlaylistId) {
		loading = true;
		loadError = null;
		video = null;
		selectedVariantId = null;
		playlist = null;
		prevVideoId = null;
		nextVideoId = null;
		previewData = null;
		previewError = null;
		currentTime = 0;
		paused = true;
		resetTimer();
		try {
			const videoRes = await authFetch(`/api/videos/${currentId}`);
			if (!videoRes.ok) throw new Error('Failed to load video');
			video = await videoRes.json();
			syncMetadataDrafts(video);
			selectedVariantId = video?.variants?.[0]?.id ?? Number(currentId);
			
			if (currentPlaylistId) {
				try {
					const res = await authFetch(`/api/playlists/${currentPlaylistId}`);
					if (res.ok) {
						playlist = await res.json();
						const items = playlist.items || [];
						const index = items.findIndex((v) => v.id === Number(currentId));
						if (index >= 0) {
							prevVideoId = index > 0 ? items[index - 1].id : null;
							nextVideoId = index < items.length - 1 ? items[index + 1].id : null;
						} else {
							prevVideoId = null;
							nextVideoId = null;
						}
					}
				} catch {
					playlist = null;
					prevVideoId = null;
					nextVideoId = null;
				}
			} else {
				playlist = null;
				prevVideoId = null;
				nextVideoId = null;
			}

			try {
				await loadMetadataOptions();
			} catch {
				metadataMessage = 'Suggestions are temporarily unavailable.';
			}
		} catch (err) {
			loadError = err.message;
		} finally {
			loading = false;
		}
	}

	$effect(() => {
		const currentId = id;
		const currentPlaylistId = playlistId;
		untrack(() => loadVideoAndPlaylist(currentId, currentPlaylistId));
	});

	onDestroy(() => {
		if (hideTimer) clearTimeout(hideTimer);
	});

	onMount(() => {
		const desktopInfoMediaQuery = window.matchMedia('(min-width: 768px)');
		const handleDesktopInfoChange = (event) => {
			if (!event.matches) {
				showInfoPanel = false;
			}
		};

		handleDesktopInfoChange(desktopInfoMediaQuery);
		desktopInfoMediaQuery.addEventListener('change', handleDesktopInfoChange);

		return () => {
			desktopInfoMediaQuery.removeEventListener('change', handleDesktopInfoChange);
		};
	});

	$effect(() => {
		if (selectedVariantId) {
			loadPreviewData(selectedVariantId);
		}
	});
	
	$effect(() => {
		if (paused) {
			showControls = true;
			if (hideTimer) clearTimeout(hideTimer);
		} else {
			resetTimer();
		}
	});

	let progress = $derived(duration > 0 ? (currentTime / duration) * 100 : 0);
	const previewDisplayWidth = 180;
	let previewDisplayHeight = $derived(previewData ? previewDisplayWidth * (previewData.frameHeight / previewData.frameWidth) : 0);
	let previewLeft = $derived(`clamp(${previewDisplayWidth / 2}px, ${hoverPercent}%, calc(100% - ${previewDisplayWidth / 2}px))`);
	let previewBackgroundPosition = $derived.by(() => {
		if (!previewData || hoverPreviewIndex < 0) return '0px 0px';
		const column = hoverPreviewIndex % previewData.columns;
		const row = Math.floor(hoverPreviewIndex / previewData.columns);
		return `${-column * previewDisplayWidth}px ${-row * previewDisplayHeight}px`;
	});
	let previewBackgroundSize = $derived(previewData ? `${previewData.columns * previewDisplayWidth}px ${previewData.rows * previewDisplayHeight}px` : 'auto');
</script>

<svelte:window on:keydown={handleKeydown} />
<svelte:head>
	<title>Player</title>
</svelte:head>

<div 
	bind:this={containerEl}
	role="presentation"
	class="fixed inset-0 bg-black z-50 flex flex-col items-center justify-center"
	onmousemove={resetTimer}
	onmouseleave={() => { if (!paused) showControls = false; }}
>
	<a
		href={playlistId ? `/playlists/${playlistId}` : "/"}
		class="absolute left-3 right-3 top-3 z-50 flex items-center gap-2 border border-white/20 bg-black/50 px-3 py-2 text-[10px] font-bold uppercase tracking-[0.25em] text-white/50 backdrop-blur transition-all hover:border-white/50 hover:text-white md:left-6 md:right-24 md:top-6 md:gap-4 md:px-4 md:text-xs md:tracking-widest {showControls ? 'opacity-100 pointer-events-auto' : 'opacity-0 pointer-events-none'} duration-300"
	>
		<span class="shrink-0">&larr;</span>
		<span class="text-white/70 min-w-0 truncate text-[10px] sm:max-w-xs sm:text-xs">
			{video?.title || 'Unknown title'}
		</span>
	</a>

	<button
		class="absolute right-3 top-3 z-30 hidden items-center gap-2 border border-white/20 bg-black/50 px-4 py-2 text-xs font-bold uppercase tracking-widest text-white/50 backdrop-blur transition-all hover:border-white/50 hover:text-white md:flex md:right-6 md:top-6 {(showControls || showInfoPanel) ? 'opacity-100 pointer-events-auto' : 'opacity-0 pointer-events-none'} duration-300"
		aria-label={showInfoPanel ? 'Hide video details' : 'Show video details'}
		onclick={toggleInfoPanel}
	>
		<svg class="w-4 h-4" viewBox="0 0 24 24" fill="currentColor">
			<path d="M11 7h2V5h-2v2zm0 12h2v-8h-2v8zm1-17C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2z" />
		</svg>
		Info
	</button>

	{#if loading}
		<div class="absolute inset-0 flex items-center justify-center text-white uppercase tracking-[0.3em] text-sm z-40">
			Loading...
		</div>
	{:else if loadError}
		<div class="absolute inset-0 flex items-center justify-center z-40 px-6">
			<div class="border border-white/20 bg-black/60 px-6 py-4 text-sm uppercase tracking-[0.2em] text-white/80">
				{loadError}
			</div>
		</div>
	{/if}
	
	<video
		bind:this={videoEl}
		bind:currentTime
		bind:duration
		bind:paused
		bind:volume
		bind:muted
		src={videoSrc}
		class="w-full h-full object-contain cursor-pointer"
		onclick={handleVideoSurfaceClick}
	>
		<track kind="captions" />
	</video>

	<div class="absolute inset-y-0 right-0 z-40 hidden w-full max-w-md border-l border-neutral-200 bg-white/95 shadow-2xl backdrop-blur transition-transform duration-300 dark:border-neutral-800 dark:bg-black/95 md:block {showInfoPanel ? 'translate-x-0' : 'translate-x-full'}">
		<div class="flex h-full flex-col">
			<div class="flex items-start justify-between gap-4 border-b border-neutral-200 dark:border-neutral-800 px-6 py-5">
				<div>
					<p class="text-[10px] uppercase tracking-[0.3em] text-neutral-400">Video Details</p>
					<h2 class="mt-2 text-lg font-semibold text-neutral-900 dark:text-white">{video?.title || 'Unknown title'}</h2>
				</div>
				<button
					class="mt-0.5 text-neutral-500 hover:text-neutral-900 dark:text-neutral-400 dark:hover:text-white transition-colors"
					aria-label="Close video details"
					onclick={() => showInfoPanel = false}
				>
					<svg class="w-6 h-6" viewBox="0 0 24 24" fill="currentColor">
						<path d="M19 6.41L17.59 5 12 10.59 6.41 5 5 6.41 10.59 12 5 17.59 6.41 19 12 13.41 17.59 19 19 17.59 13.41 12z" />
					</svg>
				</button>
			</div>

			<div class="flex-1 space-y-6 overflow-y-auto px-6 py-5 text-sm text-neutral-700 dark:text-white/80">
				<div class="grid gap-4 border border-neutral-200 dark:border-neutral-800 bg-neutral-100/60 dark:bg-neutral-950/60 p-4">
					<div>
						<p class="text-[10px] uppercase tracking-[0.3em] text-neutral-400">Duration</p>
						<p class="mt-2 text-neutral-900 dark:text-white">{formatTime(video?.duration || 0)}</p>
					</div>
					<div>
						<p class="text-[10px] uppercase tracking-[0.3em] text-neutral-400">Date Added</p>
						<p class="mt-2 text-neutral-900 dark:text-white">{formatDate(video?.date_added)}</p>
					</div>
					<div>
						<p class="text-[10px] uppercase tracking-[0.3em] text-neutral-400">Last Scanned</p>
						<p class="mt-2 text-neutral-900 dark:text-white">{formatDate(video?.date_scanned)}</p>
					</div>
					<div>
						<p class="text-[10px] uppercase tracking-[0.3em] text-neutral-400">Current Quality</p>
						<p class="mt-2 text-neutral-900 dark:text-white">{video?.variants?.find((variant) => variant.id === selectedVariantId)?.quality || 'Original'}</p>
					</div>
				</div>

				<div class="space-y-4 border-t border-neutral-200 dark:border-neutral-800 pt-6">
					<MetadataTokenInput
						label="Tags"
						values={tagsDraft}
						suggestions={metadataOptions.tags}
						placeholder="Type to add or select tags"
						helpText="Type to filter existing tags, then press Enter, Tab, or comma to add one."
						disabled={savingMetadata}
						onChange={handleTagsChange}
					/>

					<MetadataTokenInput
						label="Actors / Actresses"
						values={actorsDraft}
						suggestions={metadataOptions.actors}
						placeholder="Type to add or select actors"
						helpText="Pick existing names from the list or add new ones as chips. Changes apply to all variants with this title."
						disabled={savingMetadata}
						onChange={handleActorsChange}
					/>

					<div class="flex items-center gap-3">
						<button
							class="border border-neutral-900 bg-neutral-900 px-4 py-2 text-xs font-bold uppercase tracking-[0.25em] text-white transition-colors hover:bg-neutral-800 dark:border-white dark:bg-white dark:text-black dark:hover:bg-white/85 disabled:border-neutral-300 disabled:bg-neutral-200 disabled:text-neutral-400 dark:disabled:border-neutral-700 dark:disabled:bg-neutral-800 dark:disabled:text-neutral-500"
							onclick={saveMetadata}
							disabled={savingMetadata}
						>
							{#if savingMetadata}
								Saving...
							{:else}
								Save Details
							{/if}
						</button>
						{#if metadataMessage}
							<p class="text-xs text-neutral-500 dark:text-neutral-400">{metadataMessage}</p>
						{/if}
					</div>
				</div>

				<div>
					<p class="text-[10px] uppercase tracking-[0.3em] text-neutral-400">Available Variants</p>
					<div class="mt-3 space-y-2">
						{#each video?.variants || [] as variant (variant.id)}
							<div class="border border-neutral-200 dark:border-neutral-800 bg-neutral-100/60 dark:bg-neutral-950/60 px-4 py-3">
								<div class="flex items-start justify-between gap-3">
									<div>
										<p class="text-neutral-900 dark:text-white">{variant.quality || 'Original'}</p>
										<p class="mt-1 break-all text-xs text-neutral-500 dark:text-neutral-400">{variant.filename}</p>
									</div>
									{#if variant.id === selectedVariantId}
										<span class="border border-neutral-300 dark:border-neutral-700 px-2 py-1 text-[10px] uppercase tracking-[0.25em] text-neutral-500 dark:text-neutral-400">Playing</span>
									{/if}
								</div>
							</div>
						{/each}
					</div>
				</div>
			</div>
		</div>
	</div>

	{#if paused}
		<div 
			class="absolute inset-0 flex items-center justify-center pointer-events-none"
		>
			<button 
				class="w-24 h-24 rounded-full bg-white/10 hover:bg-white/20 backdrop-blur-md flex items-center justify-center text-white transition-all pointer-events-auto"
				aria-label="Play video"
				onclick={togglePlay}
			>
				<svg class="w-12 h-12 ml-2" viewBox="0 0 24 24" fill="currentColor">
					<path d="M8 5v14l11-7z" />
				</svg>
			</button>
		</div>
	{/if}

	<div 
		class="absolute bottom-0 left-0 right-0 bg-gradient-to-t from-black/90 to-transparent pt-12 transition-opacity duration-300 {showControls ? 'opacity-100 pointer-events-auto' : 'opacity-0 pointer-events-none'}"
	>
		<div 
			class="w-full h-2 bg-white/20 cursor-pointer group hover:h-3 transition-all relative"
			role="slider"
			tabindex="0"
			aria-label="Seek"
			aria-valuemin="0"
			aria-valuemax={Math.round(duration || 0)}
			aria-valuenow={Math.round(currentTime || 0)}
			onclick={seek}
			onkeydown={seekByKeyboard}
			onmousemove={handleSeekHover}
			onmouseleave={clearSeekHover}
		>
			{#if showingPreview && previewData}
				<div class="absolute bottom-full mb-4 -translate-x-1/2 pointer-events-none" style="left: {previewLeft}">
					<div class="border border-white/20 bg-black/80 p-2 backdrop-blur-sm">
						<div
							data-scrubber-preview="true"
							class="bg-black"
							style="width: {previewDisplayWidth}px; height: {previewDisplayHeight}px; background-image: url('{previewData.sprite_url}'); background-position: {previewBackgroundPosition}; background-size: {previewBackgroundSize};"
						></div>
						<p class="mt-2 text-center text-[10px] uppercase tracking-[0.2em] text-white/70">{formatTime(hoverTime)}</p>
					</div>
				</div>
			{/if}
				<div 
					class="absolute top-0 left-0 h-full bg-white will-change-[width]"
					style="width: {progress}%"
				></div>
			<div 
				class="absolute top-1/2 -translate-y-1/2 w-4 h-4 bg-white rounded-full opacity-0 group-hover:opacity-100 transition-opacity"
				style="left: calc({progress}% - 8px)"
			></div>
		</div>

		<div class="px-4 py-4 text-white sm:px-6">
			<div class="flex items-center gap-2 sm:justify-between sm:gap-6">
				<div class="min-w-0 flex flex-1 items-center gap-2 sm:gap-6">
					<div class="flex shrink-0 items-center gap-2 sm:gap-4">
					{#if prevVideoId}
						<a 
							href={`/player/${prevVideoId}?playlist=${playlistId}`}
							class="text-white/50 hover:text-white transition-colors focus:outline-none"
							aria-label="Previous Video"
						>
							<svg class="h-5 w-5 sm:h-6 sm:w-6" viewBox="0 0 24 24" fill="currentColor">
								<path d="M6 6h2v12H6zm3.5 6l8.5 6V6z"/>
							</svg>
						</a>
					{/if}
					
					<button 
						class="hover:text-gray-300 transition-colors focus:outline-none"
						aria-label={paused ? 'Play' : 'Pause'}
						onclick={togglePlay}
					>
						{#if paused}
							<svg class="h-7 w-7 sm:h-8 sm:w-8" viewBox="0 0 24 24" fill="currentColor">
								<path d="M8 5v14l11-7z" />
							</svg>
						{:else}
							<svg class="h-7 w-7 sm:h-8 sm:w-8" viewBox="0 0 24 24" fill="currentColor">
								<path d="M6 19h4V5H6v14zm8-14v14h4V5h-4z" />
							</svg>
						{/if}
					</button>

					{#if nextVideoId}
						<a 
							href={`/player/${nextVideoId}?playlist=${playlistId}`}
							class="text-white/50 hover:text-white transition-colors focus:outline-none"
							aria-label="Next Video"
						>
							<svg class="h-5 w-5 sm:h-6 sm:w-6" viewBox="0 0 24 24" fill="currentColor">
								<path d="M6 18l8.5-6L6 6v12zM16 6v12h2V6h-2z"/>
							</svg>
						</a>
					{/if}
					</div>

					<div class="min-w-0 truncate text-[11px] font-medium opacity-90 font-mono sm:text-sm">
					{formatTime(currentTime)} / {formatTime(duration)}
					</div>
				</div>

				<div class="ml-auto flex max-w-[45%] flex-wrap items-center justify-end gap-2 sm:max-w-none sm:flex-nowrap sm:gap-6">
				{#if video?.variants?.length > 1}
					<label class="flex min-w-0 items-center gap-2 text-[10px] uppercase tracking-[0.2em] text-white/70 sm:gap-3 sm:text-xs">
						<span class="hidden sm:inline">Quality</span>
						<select
							class="w-[4.75rem] border border-white/20 bg-black/50 px-1.5 py-1.5 text-xs text-white outline-none sm:w-auto sm:max-w-none sm:px-3 sm:py-2 sm:text-sm"
							bind:value={selectedVariantId}
							onchange={(e) => updateSelectedVariant(e.currentTarget.value)}
						>
						{#each video.variants as variant (variant.id)}
							<option value={variant.id}>{variant.quality || 'Original'}</option>
						{/each}
						</select>
					</label>
				{/if}

				<div class="flex items-center gap-2 sm:gap-3">
					<button 
						class="hover:text-gray-300 transition-colors focus:outline-none"
						aria-label={muted ? 'Unmute' : 'Mute'}
						onclick={() => muted = !muted}
					>
						{#if muted || volume === 0}
							<svg class="h-6 w-6 sm:h-7 sm:w-7" viewBox="0 0 24 24" fill="currentColor">
								<path d="M16.5 12c0-1.77-1.02-3.29-2.5-4.03v2.21l2.45 2.45c.03-.2.05-.41.05-.63zm2.5 0c0 .94-.2 1.82-.54 2.64l1.51 1.51C20.63 14.91 21 13.5 21 12c0-4.28-2.99-7.86-7-8.77v2.06c2.89.86 5 3.54 5 6.71zM4.27 3L3 4.27 7.73 9H3v6h4l5 5v-6.73l4.25 4.25c-.67.52-1.42.93-2.25 1.18v2.06c1.38-.31 2.63-.95 3.69-1.81L19.73 21 21 19.73l-9-9L4.27 3zM12 4L9.91 6.09 12 8.18V4z"/>
							</svg>
						{:else if volume < 0.5}
							<svg class="h-6 w-6 sm:h-7 sm:w-7" viewBox="0 0 24 24" fill="currentColor">
								<path d="M18.5 12c0-1.77-1.02-3.29-2.5-4.03v8.05c1.48-.73 2.5-2.25 2.5-4.02zM5 9v6h4l5 5V4L9 9H5z"/>
							</svg>
						{:else}
							<svg class="h-6 w-6 sm:h-7 sm:w-7" viewBox="0 0 24 24" fill="currentColor">
								<path d="M3 9v6h4l5 5V4L7 9H3zm13.5 3c0-1.77-1.02-3.29-2.5-4.03v8.05c1.48-.73 2.5-2.25 2.5-4.02zM14 3.23v2.06c2.89.86 5 3.54 5 6.71s-2.11 5.85-5 6.71v2.06c4.01-.91 7-4.49 7-8.77s-2.99-7.86-7-8.77z"/>
							</svg>
						{/if}
					</button>
					<input 
						type="range" 
						min="0" 
						max="1" 
						step="0.05" 
						aria-label="Volume"
						bind:value={volume}
						class="w-8 accent-white sm:w-20"
					/>
				</div>

				<button 
					class="hover:text-gray-300 transition-colors focus:outline-none"
					aria-label="Fullscreen"
					onclick={toggleFullscreen}
				>
					<svg class="h-7 w-7 sm:h-8 sm:w-8" viewBox="0 0 24 24" fill="currentColor">
						<path d="M7 14H5v5h5v-2H7v-3zm-2-4h2V7h3V5H5v5zm12 7h-3v2h5v-5h-2v3zM14 5v2h3v3h2V5h-5z"/>
					</svg>
				</button>
			</div>
			</div>
		</div>
	</div>
</div>
