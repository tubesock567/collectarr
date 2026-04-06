<script>
	import { auth, authFetch } from '$lib/auth';
	import { onMount } from 'svelte';

	let scanning = $state(false);
	let message = $state('');
	let generatingThumbs = $state(false);
	let thumbMessage = $state('');
	let currentPassword = $state('');
	let newPassword = $state('');
	let confirmPassword = $state('');
	let changingPassword = $state(false);
	let passwordMessage = $state('');
	let hardLinkDestination = $state('');
	let newDestination = $state('');
	let savingDestination = $state(false);
	let destinationMessage = $state('');

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
			const res = await authFetch('/api/settings/hardlink-dest');
			if (!res.ok) throw new Error(await readError(res, 'Failed to load hard link destination'));

			const data = await res.json();
			hardLinkDestination = data?.destination || '';
			newDestination = data?.destination || '';
		} catch (err) {
			destinationMessage = `Error: ${err.message}`;
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

	async function saveDestination() {
		if (savingDestination) return;
		savingDestination = true;
		destinationMessage = '';

		try {
			const res = await authFetch('/api/settings/hardlink-dest', {
				method: 'POST',
				body: JSON.stringify({
					destination: newDestination.trim()
				})
			});

			if (!res.ok) throw new Error(await readError(res, 'Failed to save hard link destination'));

			const data = await res.json();
			hardLinkDestination = data?.destination || '';
			newDestination = data?.destination || '';
			destinationMessage = 'Hard link destination updated successfully.';
		} catch (err) {
			destinationMessage = `Error: ${err.message}`;
		} finally {
			savingDestination = false;
		}
	}
</script>

<svelte:head>
	<title>Collectarr - Settings</title>
</svelte:head>

<div class="max-w-3xl mx-auto px-4 sm:px-6 lg:px-8 py-12">
	<h1 class="text-2xl font-bold uppercase tracking-widest mb-12 border-b border-neutral-800 pb-4">Settings</h1>
	
	<div class="space-y-8">
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

		<section class="border border-neutral-800 p-6 flex flex-col items-start gap-4">
			<div>
				<h2 class="text-sm font-semibold uppercase tracking-widest text-white mb-1">Hard Link Settings</h2>
				<p class="text-xs text-neutral-500">Configure the destination directory for hard linking files.</p>
			</div>

			<div class="w-full grid gap-4">
				<div class="grid gap-2">
					<span class="text-xs uppercase tracking-[0.25em] text-neutral-400">Current Destination</span>
					<p class="w-full border border-neutral-800 bg-black px-4 py-3 text-sm text-neutral-300">
						{hardLinkDestination || 'Not configured'}
					</p>
				</div>

				<label class="grid gap-2 w-full">
					<span class="text-xs uppercase tracking-[0.25em] text-neutral-400">Destination Path</span>
					<input
						type="text"
						bind:value={newDestination}
						placeholder="/path/to/destination"
						class="w-full border border-neutral-800 bg-black px-4 py-3 outline-none focus:border-neutral-500"
					/>
				</label>
			</div>

			<button
				onclick={saveDestination}
				disabled={savingDestination}
				class="mt-2 bg-white text-black hover:bg-neutral-300 disabled:bg-neutral-800 disabled:text-neutral-500 font-bold uppercase tracking-widest text-xs px-6 py-3 transition-colors flex items-center gap-3"
			>
				{#if savingDestination}
					<span class="loading loading-spinner loading-xs"></span>
					Saving...
				{:else}
					Save Destination
				{/if}
			</button>

			{#if destinationMessage}
				<p class="text-xs tracking-wide {destinationMessage.startsWith('Error') ? 'text-red-500' : 'text-neutral-400'} mt-2">
					{destinationMessage}
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
				<h2 class="text-sm font-semibold uppercase tracking-widest text-white mb-1">System Info</h2>
				<p class="text-xs text-neutral-500">Collectarr Media Server v1.0.0</p>
			</div>
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
</div>
