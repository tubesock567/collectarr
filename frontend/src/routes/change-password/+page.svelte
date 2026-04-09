<script>
	import { goto } from '$app/navigation';
	import { auth, authFetch } from '$lib/auth';

	let currentPassword = $state('');
	let newPassword = $state('');
	let confirmPassword = $state('');
	let submitting = $state(false);
	let error = $state('');
	let success = $state(false);

	$effect(() => {
		if (!$auth.isAuthenticated) {
			goto('/login');
		}
	});

	async function submitChangePassword(event) {
		event.preventDefault();
		if (submitting) return;

		if (newPassword !== confirmPassword) {
			error = 'New passwords do not match';
			return;
		}

		if (newPassword.length < 8) {
			error = 'Password must be at least 8 characters';
			return;
		}

		submitting = true;
		error = '';

		try {
			const response = await authFetch('/api/auth/change-password', {
				method: 'POST',
				body: JSON.stringify({
					current_password: currentPassword,
					new_password: newPassword
				})
			});

			const data = await response.json();
			if (!response.ok) {
				throw new Error(data?.error || 'Failed to change password');
			}

			success = true;
			setTimeout(() => {
				goto('/');
			}, 1500);
		} catch (err) {
			error = err.message;
		} finally {
			submitting = false;
		}
	}
</script>

<svelte:head>
	<title>Collectarr - Change Password</title>
</svelte:head>

<div class="min-h-screen flex items-center justify-center px-4">
	<form
		class="w-full max-w-md border border-neutral-800 bg-neutral-950 p-8 space-y-6"
		onsubmit={submitChangePassword}
	>
		<div class="space-y-2">
			<p class="text-xs uppercase tracking-[0.3em] text-neutral-500">Collectarr</p>
			<h1 class="text-2xl font-bold uppercase tracking-widest">Change Password</h1>
			{#if $auth.forcePasswordChange}
				<p class="text-sm text-amber-400">You must change your password before continuing.</p>
			{/if}
		</div>

		<label class="block space-y-2">
			<span class="text-xs uppercase tracking-[0.25em] text-neutral-400">Current Password</span>
			<input
				type="password"
				bind:value={currentPassword}
				class="w-full border border-neutral-800 bg-black px-4 py-3 outline-none focus:border-neutral-500"
				autocomplete="current-password"
				required
			/>
		</label>

		<label class="block space-y-2">
			<span class="text-xs uppercase tracking-[0.25em] text-neutral-400">New Password</span>
			<input
				type="password"
				bind:value={newPassword}
				class="w-full border border-neutral-800 bg-black px-4 py-3 outline-none focus:border-neutral-500"
				autocomplete="new-password"
				required
			/>
		</label>

		<label class="block space-y-2">
			<span class="text-xs uppercase tracking-[0.25em] text-neutral-400">Confirm New Password</span>
			<input
				type="password"
				bind:value={confirmPassword}
				class="w-full border border-neutral-800 bg-black px-4 py-3 outline-none focus:border-neutral-500"
				autocomplete="new-password"
				required
			/>
		</label>

		{#if error}
			<p class="text-sm text-red-500">{error}</p>
		{/if}

		{#if success}
			<p class="text-sm text-emerald-400">Password changed successfully. Redirecting...</p>
		{/if}

		<button
			type="submit"
			disabled={submitting || success}
			class="w-full bg-white text-black hover:bg-neutral-300 disabled:bg-neutral-800 disabled:text-neutral-500 font-bold uppercase tracking-widest text-xs px-6 py-3 transition-colors"
		>
			{#if submitting}Changing...{:else}Change Password{/if}
		</button>
	</form>
</div>
