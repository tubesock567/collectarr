<script>
	import { auth, authFetch } from '$lib/auth';
	import { onMount } from 'svelte';
	import DirectoryBrowser from '$lib/components/DirectoryBrowser.svelte';
	import { theme } from '$lib/theme';

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
	let clearingDatabase = $state(false);
	let clearDatabaseMessage = $state('');
	let showMediaBrowser = $state(false);

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
			if (!mediaPathRes.ok) throw new Error(await readError(mediaPathRes, 'Failed to load media path'));
			const mediaPathData = await mediaPathRes.json();
			mediaPath = mediaPathData?.path || '';
			newMediaPath = mediaPathData?.path || '';
		} catch (err) {
			mediaPathMessage = `Error: ${err.message}`;
		}
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
				setTimeout(() => message = '', 5000);
			}, 1000);
		}
	}

	async function generateThumbnails() {
		if (generatingThumbs) return;
		generatingThumbs = true;
		thumbMessage = '';

		try {
			const res = await authFetch('/api/thumbnails/generate', { method: 'POST' });
			if (!res.ok) throw new Error(await readError(res, 'Failed to start thumbnail generation'));
			await res.json();
			thumbMessage = 'Thumbnail generation started. This may take a while...';
			setTimeout(() => {
				generatingThumbs = false;
			}, 3000);
		} catch (err) {
			thumbMessage = `Error: ${err.message}`;
			generatingThumbs = false;
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

	async function clearDatabase() {
		if (clearingDatabase) return;
		if (!confirm('Are you sure you want to clear the library database? This will remove all video metadata but will not delete any files.')) {
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

	const tabs = [
		{ id: 'account', label: 'Account' },
		{ id: 'library', label: 'Library' },
		{ id: 'system', label: 'System' }
	];
</script>

<svelte:head>
	<title>Collectarr - Settings</title>
</svelte:head>

<div class="max-w-3xl mx-auto px-4 sm:px-6 lg:px-8 py-12">
	<h1 class="text-2xl font-bold uppercase tracking-widest mb-8 border-b border-neutral-800 pb-4">Settings</h1>

	<div class="flex gap-1 mb-8 border-b border-neutral-800">
		{#each tabs as tab}
			<button
				onclick={() => activeTab = tab.id}
				class="px-6 py-3 text-xs uppercase tracking-widest font-semibold transition-colors {activeTab === tab.id ? 'bg-white text-black' : 'text-neutral-400 hover:text-white hover:bg-neutral-900'}"
			>
				{tab.label}
			</button>
		{/each}
	</div>

	{#if activeTab === 'account'}
		<div class="space-y-8">
			<section class="border border-neutral-800 p-6 flex flex-col items-start gap-4">
				<div>
					<h2 class="text-sm font-semibold uppercase tracking-widest text-white mb-1">Appearance</h2>
					<p class="text-xs text-neutral-500">Choose how Collectarr should look across the app.</p>
				</div>

				<div class="flex flex-wrap gap-3">
					<button
						onclick={() => theme.setTheme('dark')}
						class="px-4 py-2 text-xs uppercase tracking-widest border transition-colors { $theme === 'dark' ? 'bg-white text-black border-white' : 'border-neutral-800 text-neutral-400 hover:text-white hover:border-neutral-500' }"
					>
						Dark
					</button>
					<button
						onclick={() => theme.setTheme('light')}
						class="px-4 py-2 text-xs uppercase tracking-widest border transition-colors { $theme === 'light' ? 'bg-white text-black border-white' : 'border-neutral-800 text-neutral-400 hover:text-white hover:border-neutral-500' }"
					>
						Light
					</button>
				</div>
			</section>

			<section class="border border-neutral-800 p-6 flex flex-col items-start gap-4">
				<div>
					<h2 class="text-sm font-semibold uppercase tracking-widest text-white mb-1">Password</h2>
					<p class="text-xs text-neutral-500">Signed in as {$auth.username}. Update your account password below.</p>
				</div>

				<div class="w-full grid gap-4">
					<label class="grid gap-2">
						<span class="text-xs uppercase tracking-[0.25em] text-neutral-400">Current Password</span>
						<input type="password" bind:value={currentPassword} class="w-full border border-neutral-800 bg-black px-4 py-3 outline-none focus:border-neutral-500" autocomplete="current-password" />
					</label>

					<label class="grid gap-2">
						<span class="text-xs uppercase tracking-[0.25em] text-neutral-400">New Password</span>
						<input type="password" bind:value={newPassword} class="w-full border border-neutral-800 bg-black px-4 py-3 outline-none focus:border-neutral-500" autocomplete="new-password" />
					</label>

					<label class="grid gap-2">
						<span class="text-xs uppercase tracking-[0.25em] text-neutral-400">Confirm New Password</span>
						<input type="password" bind:value={confirmPassword} class="w-full border border-neutral-800 bg-black px-4 py-3 outline-none focus:border-neutral-500" autocomplete="new-password" />
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
					<p class="text-xs tracking-wide {passwordMessage.startsWith('Error') ? 'text-red-500' : 'text-neutral-400'} mt-2">
						{passwordMessage}
					</p>
				{/if}
			</section>
		</div>
	{/if}

	{#if activeTab === 'library'}
		<div class="space-y-8">
			<section class="border border-neutral-800 p-6 flex flex-col items-start gap-4">
				<div>
					<h2 class="text-sm font-semibold uppercase tracking-widest text-white mb-1">Media Path</h2>
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
								onclick={() => showMediaBrowser = true}
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
					<p class="text-xs tracking-wide {mediaPathMessage.startsWith('Error') ? 'text-red-500' : 'text-neutral-400'} mt-2">
						{mediaPathMessage}
					</p>
				{/if}
			</section>

			<section class="border border-neutral-800 p-6 flex flex-col items-start gap-4">
				<div>
					<h2 class="text-sm font-semibold uppercase tracking-widest text-white mb-1">Library Management</h2>
					<p class="text-xs text-neutral-500">Trigger a manual rescan of your media directory to discover new files.</p>
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
					<p class="text-xs tracking-wide {message.startsWith('Error') ? 'text-red-500' : 'text-neutral-400'} mt-2">
						{message}
					</p>
				{/if}
			</section>

			<section class="border border-neutral-800 p-6 flex flex-col items-start gap-4">
				<div>
					<h2 class="text-sm font-semibold uppercase tracking-widest text-white mb-1">Thumbnails</h2>
					<p class="text-xs text-neutral-500">Generate thumbnails for all videos in your library.</p>
				</div>

				<button
					onclick={generateThumbnails}
					disabled={generatingThumbs}
					class="mt-2 bg-white text-black hover:bg-neutral-300 disabled:bg-neutral-800 disabled:text-neutral-500 font-bold uppercase tracking-widest text-xs px-6 py-3 transition-colors flex items-center gap-3"
				>
					{#if generatingThumbs}
						<span class="loading loading-spinner loading-xs"></span>
						Generating...
					{:else}
						Generate All Thumbnails
					{/if}
				</button>

				{#if thumbMessage}
					<p class="text-xs tracking-wide {thumbMessage.startsWith('Error') ? 'text-red-500' : 'text-neutral-400'} mt-2">
						{thumbMessage}
					</p>
				{/if}
			</section>
		</div>
	{/if}

	{#if activeTab === 'system'}
		<div class="space-y-8">
			<section class="border border-neutral-800 p-6 flex flex-col items-start gap-4">
				<div>
					<h2 class="text-sm font-semibold uppercase tracking-widest text-white mb-1">Database</h2>
					<p class="text-xs text-neutral-500">Clear the library database. This removes all video metadata but keeps your files.</p>
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
					<p class="text-xs tracking-wide {clearDatabaseMessage.startsWith('Error') ? 'text-red-500' : 'text-neutral-400'} mt-2">
						{clearDatabaseMessage}
					</p>
				{/if}
			</section>

			<section class="border border-neutral-800 p-6 flex flex-col items-start gap-4">
				<div>
					<h2 class="text-sm font-semibold uppercase tracking-widest text-white mb-1">System Info</h2>
					<p class="text-xs text-neutral-500">Collectarr Media Server v1.0.0</p>
				</div>
			</section>
		</div>
	{/if}
</div>

<DirectoryBrowser
	bind:isOpen={showMediaBrowser}
	title="Select Media Path"
	onSelect={(path) => { newMediaPath = path; }}
/>
