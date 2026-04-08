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
	let showMobileNav = $state(false);
	let mobileNavPanelEl = $state(null);
	let mobileNavTriggerEl = $state(null);
	let mobileNavWasOpen = $state(false);

	function closeMobileNav({ restoreFocus = true } = {}) {
		showMobileNav = false;
		if (restoreFocus) {
			queueMicrotask(() => mobileNavTriggerEl?.focus());
		}
	}

	function trapMobileNavFocus(event) {
		if (!showMobileNav || event.key !== 'Tab' || !mobileNavPanelEl) {
			return;
		}

		const focusableElements = Array.from(
			mobileNavPanelEl.querySelectorAll(
				'a[href], button:not([disabled]), [tabindex]:not([tabindex="-1"])'
			)
		);

		if (focusableElements.length === 0) {
			event.preventDefault();
			mobileNavPanelEl.focus();
			return;
		}

		const firstElement = focusableElements[0];
		const lastElement = focusableElements[focusableElements.length - 1];

		if (event.shiftKey && document.activeElement === firstElement) {
			event.preventDefault();
			lastElement.focus();
			return;
		}

		if (!event.shiftKey && document.activeElement === lastElement) {
			event.preventDefault();
			firstElement.focus();
		}
	}

	function handleGlobalKeydown(event) {
		if (event.key === 'Escape' && showMobileNav) {
			event.preventDefault();
			closeMobileNav();
			return;
		}

		if (showMobileNav) {
			trapMobileNavFocus(event);
			return;
		}

		if ($page.url.pathname.startsWith('/player')) {
			return;
		}

		const target = event.target;
		if (target instanceof HTMLElement) {
			const tagName = target.tagName;
			if (target.isContentEditable || tagName === 'INPUT' || tagName === 'TEXTAREA' || tagName === 'SELECT') {
				return;
			}
			if (target !== document.body && tagName !== 'BUTTON' && tagName !== 'A') {
				return;
			}
		}

		if (event.key === 'i' || event.key === 'I') {
			event.preventDefault();
			preferences.toggleIncognito();
			if (document.activeElement instanceof HTMLElement) {
				document.activeElement.blur();
			}
		}
	}

	onMount(() => {
		checkAuth();
		theme.initTheme();
		preferences.initPreferences();

		const desktopMediaQuery = window.matchMedia('(min-width: 640px)');
		const handleDesktopMediaChange = (event) => {
			if (event.matches && showMobileNav) {
				closeMobileNav({ restoreFocus: false });
			}
		};

		desktopMediaQuery.addEventListener('change', handleDesktopMediaChange);

		return () => {
			desktopMediaQuery.removeEventListener('change', handleDesktopMediaChange);
			document.body.style.overflow = '';
		};
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

	$effect(() => {
		$page.url.pathname;
		if (showMobileNav) {
			closeMobileNav({ restoreFocus: false });
		}
	});

	$effect(() => {
		const wasOpen = mobileNavWasOpen;
		mobileNavWasOpen = showMobileNav;

		if (!showMobileNav) {
			if (wasOpen) {
				document.body.style.overflow = '';
			}
			return;
		}
		document.body.style.overflow = 'hidden';
		queueMicrotask(() => mobileNavPanelEl?.focus());
		return () => {
			document.body.style.overflow = '';
		};
	});
</script>

<svelte:window on:keydown={handleGlobalKeydown} />

{#if !$page.url.pathname.startsWith('/player') && $page.url.pathname !== '/login'}
	<div class="sm:hidden border-b border-neutral-800 bg-black text-white">
		<div class="mx-auto flex max-w-7xl items-center gap-3 px-4 py-3">
			<a href="/" class="min-w-0 flex-1 truncate text-xl font-bold tracking-widest uppercase transition-colors hover:text-gray-300">Collectarr</a>
			<button
				class="h-9 w-9 flex items-center justify-center border border-neutral-700 bg-neutral-900 text-neutral-400 transition-colors hover:border-neutral-500 hover:text-white"
				aria-label={$theme === 'light' ? 'Switch to dark mode' : 'Switch to light mode'}
				onclick={() => theme.toggleTheme($theme)}
			>
				{#if $theme === 'light'}
					<svg class="w-4 h-4" viewBox="0 0 24 24" fill="currentColor">
						<path d="M9 2c-1.05 0-2.05.16-3 .46 1.69 1.23 2.8 3.24 2.8 5.54 0 3.87-3.13 7-7 7-1.11 0-2.16-.26-3.09-.72C.56 16.2 3.5 19 7 19c4.97 0 9-4.03 9-9 0-4.97-4.03-9-9-9z"/>
					</svg>
				{:else}
					<svg class="w-4 h-4" viewBox="0 0 24 24" fill="currentColor">
						<path d="M6.76 4.84l-1.8-1.79-1.41 1.41 1.79 1.79 1.42-1.41zM4 10.5H1v2h3v-2zm9-9.95h-2V3.5h2V.55zm7.45 3.91l-1.41-1.41-1.79 1.79 1.41 1.41 1.79-1.79zm-3.21 13.7l1.79 1.8 1.41-1.41-1.8-1.79-1.4 1.4zM20 10.5v2h3v-2h-3zm-8-5c-3.31 0-6 2.69-6 6s2.69 6 6 6 6-2.69 6-6-2.69-6-6-6zm-1 16.95h2V22h-2v5.05zm-7.45-3.91l1.41 1.41 1.79-1.8-1.41-1.41-1.79 1.8z"/>
					</svg>
				{/if}
			</button>
			<button
				class="h-9 w-9 flex items-center justify-center border border-neutral-700 bg-neutral-900 text-neutral-400 transition-colors hover:border-neutral-500 hover:text-white"
				aria-label={$preferences.incognito ? 'Disable incognito mode' : 'Enable incognito mode'}
				onclick={() => preferences.toggleIncognito()}
			>
				{#if $preferences.incognito}
					<svg class="w-4 h-4" viewBox="0 0 24 24" fill="currentColor">
						<path d="M12 4.5C7 4.5 2.73 7.61 1 12c1.73 4.39 6 7.5 11 7.5s9.27-3.11 11-7.5c-1.73-4.39-6-7.5-11-7.5zM12 17c-2.76 0-5-2.24-5-5s2.24-5 5-5 5 2.24 5 5-2.24 5-5 5zm0-8c-1.66 0-3 1.34-3 3s1.34 3 3 3 3-1.34 3-3-1.34-3-3-3z"/>
					</svg>
				{:else}
					<svg class="w-4 h-4" viewBox="0 0 24 24" fill="currentColor">
						<path d="M12 7c2.76 0 5 2.24 5 5 0 .65-.13 1.26-.36 1.83l2.92 2.92c1.51-1.26 2.7-2.89 3.43-4.75-1.73-4.39-6-7.5-11-7.5-1.4 0-2.74.25-3.98.7l2.16 2.16C10.74 7.13 11.35 7 12 7zM2 4.27l2.28 2.28.46.46C3.08 8.3 1.78 10.02 1 12c1.73 4.39 6 7.5 11 7.5 1.55 0 3.03-.3 4.38-.84l.42.42L19.73 22 21 20.73 3.27 3 2 4.27zM7.53 9.8l1.55 1.55c-.05.21-.08.43-.08.65 0 1.66 1.34 3 3 3 .22 0 .44-.03.65-.08l1.55 1.55c-.67.33-1.41.53-2.2.53-2.76 0-5-2.24-5-5 0-.79.2-1.53.53-2.2zm4.31-.78l3.15 3.15.02-.16c0-1.66-1.34-3-3-3l-.17.01z"/>
					</svg>
				{/if}
			</button>
			<button
				bind:this={mobileNavTriggerEl}
				class="h-9 px-3 border border-neutral-700 bg-neutral-900 text-[10px] font-semibold uppercase tracking-[0.3em] text-neutral-300 transition-colors hover:border-neutral-500 hover:text-white"
				aria-label={showMobileNav ? 'Close navigation panel' : 'Open navigation panel'}
				aria-expanded={showMobileNav}
				aria-controls="mobile-navigation-panel"
				onclick={() => showMobileNav = !showMobileNav}
			>
				{showMobileNav ? 'Close' : 'Menu'}
			</button>
		</div>
	</div>

	{#if showMobileNav}
		<div class="fixed inset-0 z-40 sm:hidden bg-black/70 backdrop-blur-sm" aria-hidden="true" onclick={() => closeMobileNav()}></div>

		<div
			id="mobile-navigation-panel"
			class="fixed inset-x-0 top-0 z-50 border-b border-neutral-800 bg-black text-white shadow-2xl transition-transform duration-300 ease-out sm:hidden translate-y-0"
			bind:this={mobileNavPanelEl}
			role="dialog"
			aria-modal="true"
			aria-label="Mobile navigation"
			tabindex="-1"
		>
			<div class="mx-auto flex max-w-7xl items-center justify-between gap-3 border-b border-neutral-800 px-4 py-3">
				<p class="text-[10px] font-semibold uppercase tracking-[0.3em] text-neutral-500">Navigation</p>
				<button
					class="text-[10px] font-semibold uppercase tracking-[0.3em] text-neutral-400 transition-colors hover:text-white"
					aria-label="Close navigation panel"
					onclick={() => closeMobileNav()}
				>
					Close
				</button>
			</div>
			<div class="mx-auto flex max-w-7xl flex-col px-4 py-4">
				<a href="/" class="flex items-center justify-between border border-neutral-800 px-4 py-4 text-xs font-semibold uppercase tracking-[0.3em] transition-colors {$page.url.pathname === '/' ? 'bg-neutral-900 text-white' : 'text-neutral-400 hover:border-neutral-600 hover:text-white'}">
					<span>Library</span>
					{#if $page.url.pathname === '/'}
						<span class="text-[10px] tracking-[0.25em] text-neutral-500">Active</span>
					{/if}
				</a>
				<a href="/playlists" class="mt-3 flex items-center justify-between border border-neutral-800 px-4 py-4 text-xs font-semibold uppercase tracking-[0.3em] transition-colors {$page.url.pathname.startsWith('/playlists') ? 'bg-neutral-900 text-white' : 'text-neutral-400 hover:border-neutral-600 hover:text-white'}">
					<span>Playlists</span>
					{#if $page.url.pathname.startsWith('/playlists')}
						<span class="text-[10px] tracking-[0.25em] text-neutral-500">Active</span>
					{/if}
				</a>
				<a href="/settings" class="mt-3 flex items-center justify-between border border-neutral-800 px-4 py-4 text-xs font-semibold uppercase tracking-[0.3em] transition-colors {$page.url.pathname === '/settings' ? 'bg-neutral-900 text-white' : 'text-neutral-400 hover:border-neutral-600 hover:text-white'}">
					<span>Settings</span>
					{#if $page.url.pathname === '/settings'}
						<span class="text-[10px] tracking-[0.25em] text-neutral-500">Active</span>
					{/if}
				</a>
				{#if $auth.isAuthenticated}
					<div class="mt-4 flex items-center justify-between border-t border-neutral-800 pt-4 text-[10px] font-semibold uppercase tracking-[0.3em] text-neutral-500">
						<span>{$auth.username}</span>
						<button class="text-neutral-400 transition-colors hover:text-white" onclick={() => logout()}>
							Logout
						</button>
					</div>
				{/if}
			</div>
		</div>
	{/if}

	<nav class="navbar hidden max-w-7xl mx-auto border-b border-neutral-800 bg-black px-4 py-2 text-white sm:flex sm:px-6 lg:px-8">
		<div class="flex-1 flex items-center gap-3">
			<a href="/" class="text-xl font-bold tracking-widest uppercase hover:text-gray-300 transition-colors">Collectarr</a>
			<button
				class="h-8 w-8 flex items-center justify-center border border-neutral-700 hover:border-neutral-500 transition-colors text-neutral-400 hover:text-white bg-neutral-900"
				aria-label={$theme === 'light' ? 'Switch to dark mode' : 'Switch to light mode'}
				onclick={() => theme.toggleTheme($theme)}
			>
				{#if $theme === 'light'}
					<svg class="w-4 h-4" viewBox="0 0 24 24" fill="currentColor">
						<path d="M9 2c-1.05 0-2.05.16-3 .46 1.69 1.23 2.8 3.24 2.8 5.54 0 3.87-3.13 7-7 7-1.11 0-2.16-.26-3.09-.72C.56 16.2 3.5 19 7 19c4.97 0 9-4.03 9-9 0-4.97-4.03-9-9-9z"/>
					</svg>
				{:else}
					<svg class="w-4 h-4" viewBox="0 0 24 24" fill="currentColor">
						<path d="M6.76 4.84l-1.8-1.79-1.41 1.41 1.79 1.79 1.42-1.41zM4 10.5H1v2h3v-2zm9-9.95h-2V3.5h2V.55zm7.45 3.91l-1.41-1.41-1.79 1.79 1.41 1.41 1.79-1.79zm-3.21 13.7l1.79 1.8 1.41-1.41-1.8-1.79-1.4 1.4zM20 10.5v2h3v-2h-3zm-8-5c-3.31 0-6 2.69-6 6s2.69 6 6 6 6-2.69 6-6-2.69-6-6-6zm-1 16.95h2V22h-2v5.05zm-7.45-3.91l1.41 1.41 1.79-1.8-1.41-1.41-1.79 1.8z"/>
					</svg>
				{/if}
			</button>
			<button
				class="h-8 w-8 flex items-center justify-center border border-neutral-700 hover:border-neutral-500 transition-colors text-neutral-400 hover:text-white bg-neutral-900"
				aria-label={$preferences.incognito ? 'Disable incognito mode' : 'Enable incognito mode'}
				onclick={() => preferences.toggleIncognito()}
			>
				{#if $preferences.incognito}
					<svg class="w-4 h-4" viewBox="0 0 24 24" fill="currentColor">
						<path d="M12 4.5C7 4.5 2.73 7.61 1 12c1.73 4.39 6 7.5 11 7.5s9.27-3.11 11-7.5c-1.73-4.39-6-7.5-11-7.5zM12 17c-2.76 0-5-2.24-5-5s2.24-5 5-5 5 2.24 5 5-2.24 5-5 5zm0-8c-1.66 0-3 1.34-3 3s1.34 3 3 3 3-1.34 3-3-1.34-3-3-3z"/>
					</svg>
				{:else}
					<svg class="w-4 h-4" viewBox="0 0 24 24" fill="currentColor">
						<path d="M12 7c2.76 0 5 2.24 5 5 0 .65-.13 1.26-.36 1.83l2.92 2.92c1.51-1.26 2.7-2.89 3.43-4.75-1.73-4.39-6-7.5-11-7.5-1.4 0-2.74.25-3.98.7l2.16 2.16C10.74 7.13 11.35 7 12 7zM2 4.27l2.28 2.28.46.46C3.08 8.3 1.78 10.02 1 12c1.73 4.39 6 7.5 11 7.5 1.55 0 3.03-.3 4.38-.84l.42.42L19.73 22 21 20.73 3.27 3 2 4.27zM7.53 9.8l1.55 1.55c-.05.21-.08.43-.08.65 0 1.66 1.34 3 3 3 .22 0 .44-.03.65-.08l1.55 1.55c-.67.33-1.41.53-2.2.53-2.76 0-5-2.24-5-5 0-.79.2-1.53.53-2.2zm4.31-.78l3.15 3.15.02-.16c0-1.66-1.34-3-3-3l-.17.01z"/>
					</svg>
				{/if}
			</button>
		</div>
		<div class="flex-none flex gap-6 items-center">
			{#if $auth.isAuthenticated}
				<span class="text-[10px] font-semibold tracking-[0.25em] uppercase text-neutral-500">{$auth.username}</span>
			{/if}
			<a href="/" class="text-xs font-semibold tracking-widest uppercase {$page.url.pathname === '/' ? 'text-white' : 'text-neutral-500'} hover:text-white transition-colors">Library</a>
			<a href="/playlists" class="text-xs font-semibold tracking-widest uppercase {$page.url.pathname.startsWith('/playlists') ? 'text-white' : 'text-neutral-500'} hover:text-white transition-colors">Playlists</a>
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

{#if $auth.isAuthenticated}
	<footer class="bg-black border-t border-neutral-800 text-neutral-500 py-4 px-4 sm:px-6 lg:px-8">
		<div class="max-w-7xl mx-auto flex flex-col items-center justify-center gap-2">
			<span class="text-xs tracking-widest">© 2026 Collectarr</span>
			<svg class="w-5 h-5 text-neutral-600" viewBox="0 0 24 24" fill="currentColor">
				<path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z"/>
			</svg>
		</div>
	</footer>
{/if}
