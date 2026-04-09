<script>
	import { goto } from '$app/navigation';
	import { auth, login } from '$lib/auth';

	let username = $state('');
	let password = $state('');
	let submitting = $state(false);
	let error = $state('');

	$effect(() => {
		if ($auth.isAuthenticated) {
			if ($auth.forcePasswordChange) {
				goto('/change-password');
			} else {
				goto('/');
			}
		}
	});

	async function submitLogin(event) {
		event.preventDefault();
		if (submitting) return;

		submitting = true;
		error = '';

		try {
			const data = await login(username.trim(), password);
			if (data.force_password_change) {
				goto('/change-password');
			} else {
				goto('/');
			}
		} catch (err) {
			error = err.message;
		} finally {
			submitting = false;
		}
	}
</script>

<svelte:head>
	<title>Collectarr - Login</title>
</svelte:head>

<div class="min-h-screen flex items-center justify-center px-4">
	<form
		class="w-full max-w-md border border-neutral-800 bg-neutral-950 p-8 space-y-6"
		onsubmit={submitLogin}
	>
		<div class="space-y-2">
			<p class="text-xs uppercase tracking-[0.3em] text-neutral-500">Collectarr</p>
			<h1 class="text-2xl font-bold uppercase tracking-widest">Login</h1>
		</div>

		<label class="block space-y-2">
			<span class="text-xs uppercase tracking-[0.25em] text-neutral-400">Username</span>
			<input
				type="text"
				bind:value={username}
				class="w-full border border-neutral-800 bg-black px-4 py-3 outline-none focus:border-neutral-500"
				autocomplete="username"
				required
			/>
		</label>

		<label class="block space-y-2">
			<span class="text-xs uppercase tracking-[0.25em] text-neutral-400">Password</span>
			<input
				type="password"
				bind:value={password}
				class="w-full border border-neutral-800 bg-black px-4 py-3 outline-none focus:border-neutral-500"
				autocomplete="current-password"
				required
			/>
		</label>

		{#if error}
			<p class="text-sm text-red-500">{error}</p>
		{/if}

		<button
			type="submit"
			disabled={submitting}
			class="w-full bg-white text-black hover:bg-neutral-300 disabled:bg-neutral-800 disabled:text-neutral-500 font-bold uppercase tracking-widest text-xs px-6 py-3 transition-colors"
		>
			{#if submitting}Signing in...{:else}Login{/if}
		</button>
	</form>
</div>
