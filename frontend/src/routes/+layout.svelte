<script>
	import '../app.css';
	import { browser } from '$app/environment';
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { auth, checkAuth, logout } from '$lib/auth';
	import { preferences } from '$lib/preferences';
	import { theme } from '$lib/theme';

	let { children } = $props();

	onMount(() => {
		checkAuth();
		theme.initTheme();
		preferences.initPreferences();
	});

	$effect(() => {
		if (!browser || $auth.loading) {
			return;
		}

		const pathname = $page.url.pathname;
		if (!$auth.isAuthenticated && pathname !== '/login') {
			goto('/login');
			return;
		}

		if ($auth.isAuthenticated && pathname === '/login') {
			goto('/');
		}
	});
</script>

{#if !$page.url.pathname.startsWith('/player') && $page.url.pathname !== '/login'}
	<nav class="navbar bg-black border-b border-neutral-800 text-white max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-2">
		<div class="flex-1">
			<a href="/" class="text-xl font-bold tracking-widest uppercase hover:text-gray-300 transition-colors">Collectarr</a>
		</div>
		<div class="flex-none flex gap-6 items-center">
			{#if $auth.isAuthenticated}
				<span class="text-[10px] font-semibold tracking-[0.25em] uppercase text-neutral-500">{$auth.username}</span>
			{/if}
			<button class="text-xs font-semibold tracking-widest uppercase text-neutral-500 hover:text-white transition-colors" onclick={() => theme.toggleTheme($theme)}>
				{$theme === 'light' ? 'Dark Mode' : 'Light Mode'}
			</button>
			<a href="/" class="text-xs font-semibold tracking-widest uppercase {$page.url.pathname === '/' ? 'text-white' : 'text-neutral-500'} hover:text-white transition-colors">Library</a>
			<a href="/settings" class="text-xs font-semibold tracking-widest uppercase {$page.url.pathname === '/settings' ? 'text-white' : 'text-neutral-500'} hover:text-white transition-colors">Settings</a>
			{#if $auth.isAuthenticated}
				<button class="text-xs font-semibold tracking-widest uppercase text-neutral-500 hover:text-white transition-colors" onclick={() => logout()}>
					Logout
				</button>
			{/if}
		</div>
	</nav>
{/if}

<main class="min-h-screen bg-black text-white w-full">
	{#if !$auth.loading || $page.url.pathname === '/login'}
		{@render children()}
	{:else}
		<div class="min-h-screen flex items-center justify-center">
			<span class="loading loading-spinner loading-lg text-white"></span>
		</div>
	{/if}
</main>
