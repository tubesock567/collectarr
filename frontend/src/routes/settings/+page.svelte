<script>
	import { auth, authFetch } from '$lib/auth';
	import MetadataTokenInput from '$lib/components/MetadataTokenInput.svelte';
	import { onMount } from 'svelte';
	import DirectoryBrowser from '$lib/components/DirectoryBrowser.svelte';

	let activeTab = $state('account');

	let scanning = $state(false);
	let message = $state('');
	let generatingThumbs = $state(false);
	let thumbMessage = $state('');
	let currentPassword = $state('');
	let newPassword = $state('');
	let confirmPassword = $state('');
	let changingPassword = $state(false);
	let passwordMessage = $state('');
	let mediaPath = $state('');
	let newMediaPath = $state('');
	let savingMediaPath = $state(false);
	let mediaPathMessage = $state('');
	let selectedThumbnails = $state(false);
	let selectedScrubberSprites = $state(false);
	let selectedHoverPreviews = $state(false);
	let selectedAutoGenerate = $state(false);
	let generatingPreviews = $state(false);
	let previewProgress = $state(null);
	let previewGenMessage = $state('');
	let previewStatusRequestInFlight = $state(false);
	let savingGenerationSettings = $state(false);
	let generationSettingsMessage = $state('');
	let clearingDatabase = $state(false);
	let clearDatabaseMessage = $state('');
	let showMediaBrowser = $state(false);
	let metadataOptions = $state({ tags: [], actors: [] });
	let metadataTagsDraft = $state([]);
	let metadataActorsDraft = $state([]);
	let savingMetadataSettings = $state(false);
	let metadataSettingsMessage = $state('');
	let logs = $state([]);
	let logsLoading = $state(false);
	let logsMessage = $state('');
	let logsRequestInFlight = $state(false);
	let latestLogsRequest = 0;

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
			const mediaPathRes = await authFetch('/api/settings/media-path');
			if (!mediaPathRes.ok)
				throw new Error(await readError(mediaPathRes, 'Failed to load media path'));
			const mediaPathData = await mediaPathRes.json();
			mediaPath = mediaPathData?.path || '';
			newMediaPath = mediaPathData?.path || '';
		} catch (err) {
			mediaPathMessage = `Error: ${err.message}`;
		}

		try {
			const generationRes = await authFetch('/api/settings/generation');
			if (!generationRes.ok)
				throw new Error(await readError(generationRes, 'Failed to load generation settings'));
			const generationData = await generationRes.json();
			selectedThumbnails = Boolean(generationData?.generate_thumbnails);
			selectedScrubberSprites = Boolean(generationData?.generate_scrubber_sprites);
			selectedHoverPreviews = Boolean(generationData?.generate_hover_previews);
			selectedAutoGenerate = Boolean(generationData?.auto_generate_during_scan);
		} catch (err) {
			generationSettingsMessage = `Error: ${err.message}`;
		}

		try {
			const previewStatusRes = await authFetch('/api/previews/status');
			if (!previewStatusRes.ok)
				throw new Error(await readError(previewStatusRes, 'Failed to load preview status'));
			applyPreviewProgress(await previewStatusRes.json());
		} catch (err) {
			previewGenMessage = `Error: ${err.message}`;
		}

		try {
			const metadataRes = await authFetch('/api/settings/metadata');
			if (!metadataRes.ok)
				throw new Error(await readError(metadataRes, 'Failed to load metadata settings'));
			metadataOptions = await metadataRes.json();
		} catch (err) {
			metadataSettingsMessage = `Error: ${err.message}`;
		}
	});

	$effect(() => {
		if (activeTab !== 'library') {
			return;
		}

		fetchPreviewGenerationStatus();
		const interval = setInterval(() => {
			fetchPreviewGenerationStatus();
		}, 2000);

		return () => clearInterval(interval);
	});

	$effect(() => {
		if (activeTab !== 'logs') {
			return;
		}

		fetchLogs(true);
		const interval = setInterval(() => {
			fetchLogs(false);
		}, 5000);

		return () => clearInterval(interval);
	});

	async function scanLibrary() {
		if (scanning) return;
		scanning = true;
		message = '';
		try {
			const res = await authFetch('/api/scan', { method: 'POST' });
			if (!res.ok) throw new Error(await readError(res, 'Scan failed'));
			message = 'Library scan initiated successfully.';
		} catch (err) {
			message = `Error: ${err.message}`;
		} finally {
			setTimeout(() => {
				scanning = false;
				setTimeout(() => (message = ''), 5000);
			}, 1000);
		}
	}

	async function changePassword() {
		if (changingPassword) return;
		passwordMessage = '';

		if (!currentPassword || !newPassword || !confirmPassword) {
			passwordMessage = 'Error: All password fields are required.';
			return;
		}
		if (newPassword !== confirmPassword) {
			passwordMessage = 'Error: New passwords do not match.';
			return;
		}
		if (newPassword.length < 4) {
			passwordMessage = 'Error: New password must be at least 4 characters.';
			return;
		}

		changingPassword = true;

		try {
			const res = await authFetch('/api/auth/change-password', {
				method: 'POST',
				body: JSON.stringify({
					current_password: currentPassword,
					new_password: newPassword
				})
			});

			if (!res.ok) throw new Error(await readError(res, 'Failed to change password'));

			currentPassword = '';
			newPassword = '';
			confirmPassword = '';
			passwordMessage = 'Password updated successfully.';
		} catch (err) {
			passwordMessage = `Error: ${err.message}`;
		} finally {
			changingPassword = false;
		}
	}

	async function saveMediaPath() {
		if (savingMediaPath) return;
		savingMediaPath = true;
		mediaPathMessage = '';

		try {
			const res = await authFetch('/api/settings/media-path', {
				method: 'POST',
				body: JSON.stringify({
					path: newMediaPath.trim()
				})
			});

			if (!res.ok) throw new Error(await readError(res, 'Failed to save media path'));

			const data = await res.json();
			mediaPath = data?.path || '';
			newMediaPath = data?.path || '';
			mediaPathMessage = 'Media path updated successfully.';
		} catch (err) {
			mediaPathMessage = `Error: ${err.message}`;
		} finally {
			savingMediaPath = false;
		}
	}

	async function saveGenerationSettings() {
		if (savingGenerationSettings) return;
		savingGenerationSettings = true;
		generationSettingsMessage = '';

		try {
			const res = await authFetch('/api/settings/generation', {
				method: 'POST',
				body: JSON.stringify({
					generate_thumbnails: selectedThumbnails,
					generate_scrubber_sprites: selectedScrubberSprites,
					generate_hover_previews: selectedHoverPreviews,
					auto_generate_during_scan: selectedAutoGenerate
				})
			});

			if (!res.ok) throw new Error(await readError(res, 'Failed to save generation settings'));
			generationSettingsMessage = 'Generation settings updated successfully.';
		} catch (err) {
			generationSettingsMessage = `Error: ${err.message}`;
		} finally {
			savingGenerationSettings = false;
		}
	}

	async function generatePreviews() {
		if (generatingPreviews) return;
		if (!selectedThumbnails && !selectedScrubberSprites && !selectedHoverPreviews) {
			previewGenMessage = 'Select at least one preview option to generate.';
			return;
		}
		generatingPreviews = true;
		previewGenMessage = '';

		try {
			const res = await authFetch('/api/previews/generate', {
				method: 'POST',
				body: JSON.stringify({
					generate_thumbnails: selectedThumbnails,
					generate_scrubber_sprites: selectedScrubberSprites,
					generate_hover_previews: selectedHoverPreviews
				})
			});
			if (!res.ok) throw new Error(await readError(res, 'Failed to start preview generation'));
			const data = await res.json();
			applyPreviewProgress(data);
			previewGenMessage = data?.message || 'Preview generation started.';
			await fetchPreviewGenerationStatus();
		} catch (err) {
			previewGenMessage = `Error: ${err.message}`;
			generatingPreviews = false;
			await fetchPreviewGenerationStatus();
		}
	}

	async function fetchPreviewGenerationStatus() {
		if (previewStatusRequestInFlight) return;
		previewStatusRequestInFlight = true;

		try {
			const res = await authFetch('/api/previews/status');
			if (!res.ok)
				throw new Error(await readError(res, 'Failed to load preview generation status'));
			applyPreviewProgress(await res.json());
		} catch (err) {
			if (generatingPreviews) {
				previewGenMessage = `Error: ${err.message}`;
			}
		} finally {
			previewStatusRequestInFlight = false;
		}
	}

	function applyPreviewProgress(status) {
		previewProgress = status;
		generatingPreviews = Boolean(status?.running);
	}

	function getPreviewProgressPercent(status) {
		if (!status) return 0;
		if (status.total_videos <= 0) return status.running ? 0 : 100;
		return Math.min(100, Math.round((status.processed_videos / status.total_videos) * 100));
	}

	function getPreviewStatusLabel(status) {
		if (!status) return '';
		if (status.status === 'running') return 'Running';
		if (status.status === 'completed') return 'Completed';
		if (status.status === 'failed') return 'Failed';
		return 'Idle';
	}

	async function clearDatabase() {
		if (clearingDatabase) return;
		if (
			!confirm(
				'Are you sure you want to clear the library database? This will remove all video metadata but will not delete any files.'
			)
		) {
			return;
		}
		clearingDatabase = true;
		clearDatabaseMessage = '';

		try {
			const res = await authFetch('/api/admin/clear-database', { method: 'POST' });
			if (!res.ok) throw new Error(await readError(res, 'Failed to clear database'));
			clearDatabaseMessage = 'Database cleared successfully. Refresh the page to see changes.';
		} catch (err) {
			clearDatabaseMessage = `Error: ${err.message}`;
		} finally {
			clearingDatabase = false;
		}
	}

	async function fetchLogs(showSpinner = true) {
		if (logsRequestInFlight) return;
		logsRequestInFlight = true;
		const requestId = ++latestLogsRequest;
		if (showSpinner) {
			logsLoading = true;
		}
		logsMessage = '';

		try {
			const res = await authFetch('/api/logs?limit=200');
			if (!res.ok) throw new Error(await readError(res, 'Failed to load logs'));
			const data = await res.json();
			if (requestId === latestLogsRequest) {
				logs = Array.isArray(data?.entries) ? data.entries : [];
			}
		} catch (err) {
			if (requestId === latestLogsRequest) {
				logsMessage = `Error: ${err.message}`;
			}
		} finally {
			if (requestId === latestLogsRequest) {
				logsLoading = false;
			}
			logsRequestInFlight = false;
		}
	}

	function formatLogTimestamp(timestamp) {
		if (!timestamp) return '';
		const value = new Date(timestamp);
		if (Number.isNaN(value.getTime())) return timestamp;
		return value.toLocaleString();
	}

	function handleMetadataTagsDraftChange(nextValues) {
		metadataTagsDraft = nextValues;
		metadataSettingsMessage = '';
	}

	function handleMetadataActorsDraftChange(nextValues) {
		metadataActorsDraft = nextValues;
		metadataSettingsMessage = '';
	}

	async function refreshMetadataSettings() {
		const res = await authFetch('/api/settings/metadata');
		if (!res.ok) throw new Error(await readError(res, 'Failed to load metadata settings'));
		metadataOptions = await res.json();
	}

	async function saveMetadataSettings(update) {
		if (savingMetadataSettings) return;
		savingMetadataSettings = true;
		metadataSettingsMessage = '';

		try {
			const res = await authFetch('/api/settings/metadata', {
				method: 'POST',
				body: JSON.stringify(update)
			});
			if (!res.ok) throw new Error(await readError(res, 'Failed to update metadata settings'));
			metadataOptions = await res.json();
			metadataSettingsMessage = 'Metadata settings updated successfully.';
		} catch (err) {
			metadataSettingsMessage = `Error: ${err.message}`;
		} finally {
			savingMetadataSettings = false;
		}
	}

	async function addMetadataEntries() {
		if (metadataTagsDraft.length === 0 && metadataActorsDraft.length === 0) {
			metadataSettingsMessage = 'Add at least one tag or actor.';
			return;
		}

		await saveMetadataSettings({
			add_tags: metadataTagsDraft,
			add_actors: metadataActorsDraft,
			remove_tags: [],
			remove_actors: []
		});

		if (!metadataSettingsMessage.startsWith('Error')) {
			metadataTagsDraft = [];
			metadataActorsDraft = [];
		}
	}

	async function removeMetadataEntry(kind, value) {
		await saveMetadataSettings({
			add_tags: [],
			add_actors: [],
			remove_tags: kind === 'tag' ? [value] : [],
			remove_actors: kind === 'actor' ? [value] : []
		});
	}

	const tabs = [
		{ id: 'account', label: 'Account' },
		{ id: 'library', label: 'Library' },
		{ id: 'logs', label: 'Logs' },
		{ id: 'system', label: 'System' }
	];
</script>

<svelte:head>
	<title>Collectarr - Settings</title>
</svelte:head>

<div class="max-w-3xl mx-auto px-4 sm:px-6 lg:px-8 py-12">
	<h1 class="text-2xl font-bold uppercase tracking-widest mb-8 border-b border-neutral-800 pb-4">
		Settings
	</h1>

	<div class="mb-2 flex items-center justify-between gap-3 sm:hidden">
		<p class="text-[10px] uppercase tracking-[0.3em] text-neutral-500">Swipe Tabs</p>
		<p class="text-[10px] uppercase tracking-[0.3em] text-neutral-600">← →</p>
	</div>

	<div class="relative -mx-4 mb-8 border-b border-neutral-800 px-4 sm:mx-0 sm:px-0">
		<div
			class="pointer-events-none absolute inset-y-0 right-0 w-8 bg-gradient-to-l from-black to-transparent sm:hidden"
		></div>
		<div
			class="overflow-x-auto [scrollbar-width:none] [&::-webkit-scrollbar]:hidden"
			role="tablist"
			aria-label="Settings sections"
		>
			<div class="flex min-w-max gap-1 sm:min-w-0 sm:flex-wrap">
				{#each tabs as tab}
					<button
						onclick={() => (activeTab = tab.id)}
						id="settings-tab-{tab.id}"
						role="tab"
						aria-selected={activeTab === tab.id}
						aria-controls="settings-panel-{tab.id}"
						tabindex={activeTab === tab.id ? 0 : -1}
						class="shrink-0 whitespace-nowrap px-6 py-3 text-xs uppercase tracking-widest font-semibold transition-colors {activeTab ===
						tab.id
							? 'bg-white text-black'
							: 'text-neutral-400 hover:text-white hover:bg-neutral-900'}"
					>
						{tab.label}
					</button>
				{/each}
			</div>
		</div>
	</div>

	{#if activeTab === 'account'}
		<div
			class="space-y-8"
			id="settings-panel-account"
			role="tabpanel"
			aria-labelledby="settings-tab-account"
		>
			<section class="border border-neutral-800 p-6 flex flex-col items-start gap-4">
				<div>
					<h2 class="text-sm font-semibold uppercase tracking-widest text-white mb-1">Password</h2>
					<p class="text-xs text-neutral-500">
						Signed in as {$auth.username}. Update your account password below.
					</p>
				</div>

				<div class="w-full grid gap-4">
					<label class="grid gap-2">
						<span class="text-xs uppercase tracking-[0.25em] text-neutral-400"
							>Current Password</span
						>
						<input
							type="password"
							bind:value={currentPassword}
							class="w-full border border-neutral-800 bg-black px-4 py-3 outline-none focus:border-neutral-500"
							autocomplete="current-password"
						/>
					</label>

					<label class="grid gap-2">
						<span class="text-xs uppercase tracking-[0.25em] text-neutral-400">New Password</span>
						<input
							type="password"
							bind:value={newPassword}
							class="w-full border border-neutral-800 bg-black px-4 py-3 outline-none focus:border-neutral-500"
							autocomplete="new-password"
						/>
					</label>

					<label class="grid gap-2">
						<span class="text-xs uppercase tracking-[0.25em] text-neutral-400"
							>Confirm New Password</span
						>
						<input
							type="password"
							bind:value={confirmPassword}
							class="w-full border border-neutral-800 bg-black px-4 py-3 outline-none focus:border-neutral-500"
							autocomplete="new-password"
						/>
					</label>
				</div>

				<button
					onclick={changePassword}
					disabled={changingPassword}
					class="mt-2 bg-white text-black hover:bg-neutral-300 disabled:bg-neutral-800 disabled:text-neutral-500 font-bold uppercase tracking-widest text-xs px-6 py-3 transition-colors flex items-center gap-3"
				>
					{#if changingPassword}
						<span class="loading loading-spinner loading-xs"></span>
						Updating...
					{:else}
						Change Password
					{/if}
				</button>

				{#if passwordMessage}
					<p
						class="text-xs tracking-wide {passwordMessage.startsWith('Error')
							? 'text-red-500'
							: 'text-neutral-400'} mt-2"
					>
						{passwordMessage}
					</p>
				{/if}
			</section>
		</div>
	{/if}

	{#if activeTab === 'library'}
		<div
			class="space-y-8"
			id="settings-panel-library"
			role="tabpanel"
			aria-labelledby="settings-tab-library"
		>
			<section class="border border-neutral-800 p-6 flex flex-col items-start gap-4">
				<div>
					<h2 class="text-sm font-semibold uppercase tracking-widest text-white mb-1">
						Library Management
					</h2>
					<p class="text-xs text-neutral-500">
						Trigger a manual rescan of your media directory to discover new files.
					</p>
				</div>

				<button
					onclick={scanLibrary}
					disabled={scanning}
					class="mt-2 bg-white text-black hover:bg-neutral-300 disabled:bg-neutral-800 disabled:text-neutral-500 font-bold uppercase tracking-widest text-xs px-6 py-3 transition-colors flex items-center gap-3"
				>
					{#if scanning}
						<span class="loading loading-spinner loading-xs"></span>
						Scanning...
					{:else}
						Rescan Library
					{/if}
				</button>

				{#if message}
					<p
						class="text-xs tracking-wide {message.startsWith('Error')
							? 'text-red-500'
							: 'text-neutral-400'} mt-2"
					>
						{message}
					</p>
				{/if}
			</section>

			<section class="border border-neutral-800 p-6 flex flex-col items-start gap-4">
				<div>
					<h2 class="text-sm font-semibold uppercase tracking-widest text-white mb-1">
						Media Path
					</h2>
					<p class="text-xs text-neutral-500">Configure the path to scan for media files.</p>
				</div>

				<div class="w-full grid gap-4">
					<div class="grid gap-2">
						<span class="text-xs uppercase tracking-[0.25em] text-neutral-400">Current Path</span>
						<p class="w-full border border-neutral-800 bg-black px-4 py-3 text-sm text-neutral-300">
							{mediaPath || 'Not configured'}
						</p>
					</div>

					<div class="grid gap-2 w-full">
						<span class="text-xs uppercase tracking-[0.25em] text-neutral-400">New Path</span>
						<div class="flex gap-2">
							<input
								type="text"
								bind:value={newMediaPath}
								placeholder="Click Browse to select directory"
								readonly
								class="flex-1 border border-neutral-800 bg-black px-4 py-3 outline-none focus:border-neutral-500 text-sm"
							/>
							<button
								onclick={() => (showMediaBrowser = true)}
								class="bg-neutral-800 text-white hover:bg-neutral-700 font-bold uppercase tracking-widest text-xs px-4 py-3 transition-colors"
							>
								Browse
							</button>
						</div>
					</div>
				</div>

				<button
					onclick={saveMediaPath}
					disabled={savingMediaPath}
					class="mt-2 bg-white text-black hover:bg-neutral-300 disabled:bg-neutral-800 disabled:text-neutral-500 font-bold uppercase tracking-widest text-xs px-6 py-3 transition-colors flex items-center gap-3"
				>
					{#if savingMediaPath}
						<span class="loading loading-spinner loading-xs"></span>
						Saving...
					{:else}
						Save Media Path
					{/if}
				</button>

				{#if mediaPathMessage}
					<p
						class="text-xs tracking-wide {mediaPathMessage.startsWith('Error')
							? 'text-red-500'
							: 'text-neutral-400'} mt-2"
					>
						{mediaPathMessage}
					</p>
				{/if}
			</section>

			<section class="border border-neutral-800 p-6 flex flex-col items-start gap-4">
				<div>
					<h2 class="text-sm font-semibold uppercase tracking-widest text-white mb-1">
						Preview Generation
					</h2>
					<p class="text-xs text-neutral-500">
						Configure and generate preview assets for your library.
					</p>
				</div>

				<div class="w-full border border-neutral-800 bg-neutral-900/30 p-4 rounded">
					<label class="flex items-center justify-between gap-4">
						<div>
							<p class="text-sm text-white uppercase tracking-widest">Auto-generate during scans</p>
							<p class="text-xs text-neutral-500 mt-1">
								Automatically generate preview assets when scanning the library.
							</p>
						</div>
						<input
							type="checkbox"
							bind:checked={selectedAutoGenerate}
							onchange={saveGenerationSettings}
							class="toggle toggle-sm rounded-none"
						/>
					</label>
				</div>

				<div class="w-full grid gap-4 border-t border-neutral-800 pt-4">
					<p class="text-xs text-neutral-400 uppercase tracking-widest mb-2">
						Manual Generation Options
					</p>

					<label
						class="flex items-center justify-between gap-4 border border-neutral-800 bg-black px-4 py-3"
					>
						<div>
							<p class="text-sm text-white uppercase tracking-widest">Generate thumbnails</p>
							<p class="text-xs text-neutral-500 mt-1">
								Include thumbnails when generating previews.
							</p>
						</div>
						<input
							type="checkbox"
							bind:checked={selectedThumbnails}
							onchange={saveGenerationSettings}
							class="toggle toggle-sm rounded-none"
						/>
					</label>

					<label
						class="flex items-center justify-between gap-4 border border-neutral-800 bg-black px-4 py-3"
					>
						<div>
							<p class="text-sm text-white uppercase tracking-widest">Generate scrubber sprites</p>
							<p class="text-xs text-neutral-500 mt-1">
								Include scrubber sprite sheets when generating previews.
							</p>
						</div>
						<input
							type="checkbox"
							bind:checked={selectedScrubberSprites}
							onchange={saveGenerationSettings}
							class="toggle toggle-sm rounded-none"
						/>
					</label>

					<label
						class="flex items-center justify-between gap-4 border border-neutral-800 bg-black px-4 py-3"
					>
						<div>
							<p class="text-sm text-white uppercase tracking-widest">Generate hover previews</p>
							<p class="text-xs text-neutral-500 mt-1">
								Include hover previews when generating previews.
							</p>
						</div>
						<input
							type="checkbox"
							bind:checked={selectedHoverPreviews}
							onchange={saveGenerationSettings}
							class="toggle toggle-sm rounded-none"
						/>
					</label>
				</div>

				{#if generationSettingsMessage}
					<p
						class="text-xs tracking-wide {generationSettingsMessage.startsWith('Error')
							? 'text-red-500'
							: 'text-neutral-400'}"
					>
						{generationSettingsMessage}
					</p>
				{/if}

				<div class="w-full border-t border-neutral-800 pt-4 mt-2">
					<p class="text-xs text-neutral-500 mb-4">
						Generate preview assets manually for all videos in your library.
					</p>

					<button
						onclick={generatePreviews}
						disabled={generatingPreviews}
						class="bg-white text-black hover:bg-neutral-300 disabled:bg-neutral-800 disabled:text-neutral-500 font-bold uppercase tracking-widest text-xs px-6 py-3 transition-colors flex items-center gap-3"
					>
						{#if generatingPreviews}
							<span class="loading loading-spinner loading-xs"></span>
							{previewProgress?.processed_videos || 0}/{previewProgress?.total_videos || 0}
						{:else}
							Generate Previews
						{/if}
					</button>

					{#if previewProgress && previewProgress.status !== 'idle'}
						<div class="mt-4 w-full border border-neutral-800 bg-black p-4 space-y-3">
							<div class="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
								<div>
									<p class="text-xs uppercase tracking-widest text-neutral-400">
										Preview Job Status
									</p>
									<p class="text-sm text-white mt-1">
										{getPreviewStatusLabel(previewProgress)} · {previewProgress.processed_videos}/{previewProgress.total_videos}
										videos
									</p>
								</div>
								<div class="text-xs uppercase tracking-widest text-neutral-500">
									{getPreviewProgressPercent(previewProgress)}%
								</div>
							</div>

							<div class="h-2 w-full border border-neutral-800 bg-neutral-950">
								<div
									class="h-full bg-white transition-all duration-300"
									style={`width: ${getPreviewProgressPercent(previewProgress)}%`}
								></div>
							</div>

							<div class="grid gap-2 text-xs text-neutral-400 sm:grid-cols-2">
								<p>
									Source: <span class="text-white uppercase"
										>{previewProgress.source || 'manual'}</span
									>
								</p>
								<p>Errors: <span class="text-white">{previewProgress.errors}</span></p>
								{#if previewProgress.current_step}
									<p>
										Current step: <span class="text-white">{previewProgress.current_step}</span>
									</p>
								{/if}
								{#if previewProgress.current_video_title}
									<p>
										Current video: <span class="text-white"
											>{previewProgress.current_video_title}</span
										>
									</p>
								{/if}
								{#if previewProgress.started_at}
									<p>
										Started: <span class="text-white"
											>{formatLogTimestamp(previewProgress.started_at)}</span
										>
									</p>
								{/if}
								{#if previewProgress.completed_at}
									<p>
										Completed: <span class="text-white"
											>{formatLogTimestamp(previewProgress.completed_at)}</span
										>
									</p>
								{/if}
							</div>

							{#if previewProgress.message}
								<p
									class="text-xs tracking-wide {previewProgress.status === 'failed'
										? 'text-red-500'
										: 'text-neutral-400'}"
								>
									{previewProgress.message}
								</p>
							{/if}
						</div>
					{/if}

					{#if previewGenMessage}
						<p
							class="text-xs tracking-wide {previewGenMessage.startsWith('Error')
								? 'text-red-500'
								: 'text-neutral-400'} mt-3"
						>
							{previewGenMessage}
						</p>
					{/if}
				</div>
			</section>

			<section class="border border-neutral-800 p-6 flex flex-col items-start gap-4">
				<div>
					<h2 class="text-sm font-semibold uppercase tracking-widest text-white mb-1">
						Metadata Library
					</h2>
					<p class="text-xs text-neutral-500">
						Manage the persistent global tag and actor catalog. Removing an entry here also removes
						it from videos that currently use it.
					</p>
				</div>

				<div class="w-full grid gap-6 lg:grid-cols-2">
					<div class="space-y-4">
						<MetadataTokenInput
							label="Add Tags"
							values={metadataTagsDraft}
							suggestions={metadataOptions.tags}
							placeholder="Type tags to add"
							helpText="These tags become available across the app immediately."
							disabled={savingMetadataSettings}
							onChange={handleMetadataTagsDraftChange}
						/>

						<div class="space-y-3">
							<p class="text-xs uppercase tracking-[0.25em] text-neutral-400">Saved Tags</p>
							{#if metadataOptions.tags.length === 0}
								<p class="border border-neutral-800 bg-black px-4 py-3 text-sm text-neutral-500">
									No tags yet.
								</p>
							{:else}
								<div class="flex flex-wrap gap-2">
									{#each metadataOptions.tags as tag (tag)}
										<button
											type="button"
											class="inline-flex items-center gap-2 border border-neutral-700 bg-neutral-900 px-3 py-2 text-xs text-white transition-colors hover:border-red-500 hover:text-red-300 disabled:cursor-not-allowed disabled:opacity-50"
											onclick={() => removeMetadataEntry('tag', tag)}
											disabled={savingMetadataSettings}
										>
											<span>{tag}</span>
											<span aria-hidden="true">×</span>
										</button>
									{/each}
								</div>
							{/if}
						</div>
					</div>

					<div class="space-y-4">
						<MetadataTokenInput
							label="Add Actors / Actresses"
							values={metadataActorsDraft}
							suggestions={metadataOptions.actors}
							placeholder="Type actors to add"
							helpText="These names become selectable anywhere metadata is edited."
							disabled={savingMetadataSettings}
							onChange={handleMetadataActorsDraftChange}
						/>

						<div class="space-y-3">
							<p class="text-xs uppercase tracking-[0.25em] text-neutral-400">
								Saved Actors / Actresses
							</p>
							{#if metadataOptions.actors.length === 0}
								<p class="border border-neutral-800 bg-black px-4 py-3 text-sm text-neutral-500">
									No actors yet.
								</p>
							{:else}
								<div class="flex flex-wrap gap-2">
									{#each metadataOptions.actors as actor (actor)}
										<button
											type="button"
											class="inline-flex items-center gap-2 border border-neutral-700 bg-neutral-900 px-3 py-2 text-xs text-white transition-colors hover:border-red-500 hover:text-red-300 disabled:cursor-not-allowed disabled:opacity-50"
											onclick={() => removeMetadataEntry('actor', actor)}
											disabled={savingMetadataSettings}
										>
											<span>{actor}</span>
											<span aria-hidden="true">×</span>
										</button>
									{/each}
								</div>
							{/if}
						</div>
					</div>
				</div>

				<button
					onclick={addMetadataEntries}
					disabled={savingMetadataSettings}
					class="mt-2 bg-white text-black hover:bg-neutral-300 disabled:bg-neutral-800 disabled:text-neutral-500 font-bold uppercase tracking-widest text-xs px-6 py-3 transition-colors flex items-center gap-3"
				>
					{#if savingMetadataSettings}
						<span class="loading loading-spinner loading-xs"></span>
						Updating...
					{:else}
						Add Metadata Entries
					{/if}
				</button>

				{#if metadataSettingsMessage}
					<p
						class="text-xs tracking-wide {metadataSettingsMessage.startsWith('Error')
							? 'text-red-500'
							: 'text-neutral-400'} mt-2"
					>
						{metadataSettingsMessage}
					</p>
				{/if}
			</section>

			<section class="border border-neutral-800 p-6 flex flex-col items-start gap-4">
				<div>
					<h2 class="text-sm font-semibold uppercase tracking-widest text-white mb-1">Database</h2>
					<p class="text-xs text-neutral-500">
						Clear the library database. This removes all video metadata but keeps your files.
					</p>
				</div>

				<button
					onclick={clearDatabase}
					disabled={clearingDatabase}
					class="mt-2 bg-red-900 text-white hover:bg-red-800 disabled:bg-neutral-800 disabled:text-neutral-500 font-bold uppercase tracking-widest text-xs px-6 py-3 transition-colors flex items-center gap-3"
				>
					{#if clearingDatabase}
						<span class="loading loading-spinner loading-xs"></span>
						Clearing...
					{:else}
						Clear Database
					{/if}
				</button>

				{#if clearDatabaseMessage}
					<p
						class="text-xs tracking-wide {clearDatabaseMessage.startsWith('Error')
							? 'text-red-500'
							: 'text-neutral-400'} mt-2"
					>
						{clearDatabaseMessage}
					</p>
				{/if}
			</section>
		</div>
	{/if}

	{#if activeTab === 'system'}
		<div
			class="space-y-8"
			id="settings-panel-system"
			role="tabpanel"
			aria-labelledby="settings-tab-system"
		>
			<section class="border border-neutral-800 p-6 flex flex-col items-start gap-4">
				<div>
					<h2 class="text-sm font-semibold uppercase tracking-widest text-white mb-1">
						System Info
					</h2>
					<p class="text-xs text-neutral-500">Collectarr Media Server v1.0.0</p>
				</div>
			</section>
		</div>
	{/if}

	{#if activeTab === 'logs'}
		<div
			class="space-y-8"
			id="settings-panel-logs"
			role="tabpanel"
			aria-labelledby="settings-tab-logs"
		>
			<section class="border border-neutral-800 p-6 flex flex-col gap-4">
				<div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
					<div>
						<h2 class="text-sm font-semibold uppercase tracking-widest text-white mb-1">
							Application Logs
						</h2>
						<p class="text-xs text-neutral-500">
							Live backend activity, API requests, scans, database changes, and preview jobs.
						</p>
					</div>

					<button
						onclick={() => fetchLogs(true)}
						disabled={logsLoading}
						class="bg-white text-black hover:bg-neutral-300 disabled:bg-neutral-800 disabled:text-neutral-500 font-bold uppercase tracking-widest text-xs px-6 py-3 transition-colors flex items-center gap-3"
					>
						{#if logsLoading}
							<span class="loading loading-spinner loading-xs"></span>
							Refreshing...
						{:else}
							Refresh Logs
						{/if}
					</button>
				</div>

				{#if logsMessage}
					<p class="text-xs tracking-wide text-red-500">{logsMessage}</p>
				{/if}

				<div class="border border-neutral-800 bg-black min-h-[28rem] max-h-[40rem] overflow-y-auto">
					{#if logsLoading && logs.length === 0}
						<div class="flex min-h-[28rem] items-center justify-center">
							<span class="loading loading-spinner loading-lg text-white"></span>
						</div>
					{:else if logs.length === 0}
						<div
							class="flex min-h-[28rem] items-center justify-center px-6 text-center text-xs uppercase tracking-widest text-neutral-500"
						>
							No logs captured yet.
						</div>
					{:else}
						<div class="divide-y divide-neutral-900 font-mono text-xs">
							{#each logs as entry}
								<div class="px-4 py-3 space-y-2">
									<div class="flex flex-col gap-2 lg:flex-row lg:items-center lg:justify-between">
										<div class="flex flex-wrap items-center gap-2">
											<span class="text-neutral-500">{formatLogTimestamp(entry.timestamp)}</span>
											<span
												class="border px-2 py-0.5 uppercase tracking-widest {entry.level === 'ERROR'
													? 'border-red-800 text-red-400'
													: entry.level === 'WARN'
														? 'border-amber-800 text-amber-400'
														: 'border-neutral-700 text-neutral-300'}">{entry.level}</span
											>
										</div>
										<p class="text-neutral-100 break-all">{entry.message}</p>
									</div>

									{#if entry.fields?.length}
										<div class="flex flex-wrap gap-2 text-[11px] text-neutral-400">
											{#each entry.fields as field}
												<span class="border border-neutral-800 bg-neutral-950 px-2 py-1 break-all"
													>{field.key}={field.value}</span
												>
											{/each}
										</div>
									{/if}
								</div>
							{/each}
						</div>
					{/if}
				</div>
			</section>
		</div>
	{/if}
</div>

<DirectoryBrowser
	bind:isOpen={showMediaBrowser}
	title="Select Media Path"
	onSelect={(path) => {
		newMediaPath = path;
	}}
/>
