<script>
	import '../app.css';
	import { browser } from '$app/environment';
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { auth, checkAuth, logout } from '$lib/auth';
	import { preferences } from '$lib/preferences';
	import { theme } from '$lib/theme';
	import Toast from '$lib/components/Toast.svelte';

	let { children } = $props();
	let showMobileNav = $state(false);
	let mobileNavPanelEl = $state(null);
	let mobileNavTriggerEl = $state(null);
	let mobileNavWasOpen = $state(false);
	let previousPathname = $state('');
	let sidebarCollapsed = $state(false);

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
			if (
				target.isContentEditable ||
				tagName === 'INPUT' ||
				tagName === 'TEXTAREA' ||
				tagName === 'SELECT'
			) {
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
		previousPathname = window.location.pathname;

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
			if ($auth.forcePasswordChange) {
				goto('/change-password');
			} else {
				goto('/');
			}
			return;
		}

		if ($auth.isAuthenticated && $auth.forcePasswordChange && pathname !== '/change-password') {
			goto('/change-password');
			return;
		}
	});

	$effect(() => {
		const pathname = $page.url.pathname;
		if (previousPathname && previousPathname !== pathname && showMobileNav) {
			closeMobileNav({ restoreFocus: false });
		}
		previousPathname = pathname;
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


{#if $page.url.pathname.startsWith('/player') || $page.url.pathname === '/login'}
	<Toast />
	<main class="min-h-screen bg-black text-white w-full">
		{@render children()}
	</main>
{:else}
	<div class="flex h-screen overflow-hidden bg-black text-white w-full">
		<!-- Mobile Top Nav -->
		<div class="sm:hidden absolute top-0 left-0 right-0 z-30 border-b border-neutral-800 bg-black text-white flex items-center justify-between px-4 py-3">
			<a
				href="/"
				class="text-lg font-bold tracking-widest uppercase transition-colors hover:text-gray-300"
			>
				Collectarr
			</a>
			<button
				bind:this={mobileNavTriggerEl}
				class="h-9 px-3 border border-neutral-700 bg-neutral-900 text-[10px] font-semibold uppercase tracking-[0.3em] text-neutral-300 transition-colors hover:border-neutral-500 hover:text-white"
				aria-label={showMobileNav ? 'Close navigation panel' : 'Open navigation panel'}
				aria-expanded={showMobileNav}
				aria-controls="mobile-navigation-panel"
				onclick={() => (showMobileNav = !showMobileNav)}
			>
				{showMobileNav ? 'Close' : 'Menu'}
			</button>
		</div>

		{#if showMobileNav}
			<div
				class="fixed inset-0 z-40 sm:hidden bg-black/70 backdrop-blur-sm"
				aria-hidden="true"
				onclick={() => closeMobileNav()}
			></div>

			<div
				id="mobile-navigation-panel"
				class="fixed inset-x-0 top-0 z-50 border-b border-neutral-800 bg-black text-white shadow-2xl transition-transform duration-300 ease-out sm:hidden translate-y-0"
				bind:this={mobileNavPanelEl}
				role="dialog"
				aria-modal="true"
				aria-label="Mobile navigation"
				tabindex="-1"
			>
				<div
					class="mx-auto flex max-w-7xl items-center justify-between gap-3 border-b border-neutral-800 px-4 py-3"
				>
					<p class="text-[10px] font-semibold uppercase tracking-[0.3em] text-neutral-500">
						Navigation
					</p>
					<button
						class="text-[10px] font-semibold uppercase tracking-[0.3em] text-neutral-400 transition-colors hover:text-white"
						aria-label="Close navigation panel"
						onclick={() => closeMobileNav()}
					>
						Close
					</button>
				</div>
				<div class="mx-auto flex max-w-7xl flex-col px-4 py-4">
					<a
						href="/"
						class="flex items-center justify-between border border-neutral-800 px-4 py-4 text-xs font-semibold uppercase tracking-[0.3em] transition-colors {$page
							.url.pathname === '/'
							? 'bg-neutral-900 text-white'
							: 'text-neutral-400 hover:border-neutral-600 hover:text-white'}"
					>
						<span>Library</span>
						{#if $page.url.pathname === '/'}
							<span class="text-[10px] tracking-[0.25em] text-neutral-500">Active</span>
						{/if}
					</a>
					<a
						href="/playlists"
						class="mt-3 flex items-center justify-between border border-neutral-800 px-4 py-4 text-xs font-semibold uppercase tracking-[0.3em] transition-colors {$page.url.pathname.startsWith(
							'/playlists'
						)
							? 'bg-neutral-900 text-white'
							: 'text-neutral-400 hover:border-neutral-600 hover:text-white'}"
					>
						<span>Playlists</span>
						{#if $page.url.pathname.startsWith('/playlists')}
							<span class="text-[10px] tracking-[0.25em] text-neutral-500">Active</span>
						{/if}
					</a>
					<a
						href="/torrents"
						class="mt-3 flex items-center justify-between border border-neutral-800 px-4 py-4 text-xs font-semibold uppercase tracking-[0.3em] transition-colors {$page
							.url.pathname === '/torrents'
							? 'bg-neutral-900 text-white'
							: 'text-neutral-400 hover:border-neutral-600 hover:text-white'}"
					>
						<span>Torrents</span>
						{#if $page.url.pathname === '/torrents'}
							<span class="text-[10px] tracking-[0.25em] text-neutral-500">Active</span>
						{/if}
					</a>
					<a
						href="/settings"
						class="mt-3 flex items-center justify-between border border-neutral-800 px-4 py-4 text-xs font-semibold uppercase tracking-[0.3em] transition-colors {$page
							.url.pathname === '/settings'
							? 'bg-neutral-900 text-white'
							: 'text-neutral-400 hover:border-neutral-600 hover:text-white'}"
					>
						<span>Settings</span>
						{#if $page.url.pathname === '/settings'}
							<span class="text-[10px] tracking-[0.25em] text-neutral-500">Active</span>
						{/if}
					</a>
					
					<div class="mt-4 flex gap-2 border-t border-neutral-800 pt-4">
						<button
							class="flex-1 h-10 flex items-center justify-center border border-neutral-700 bg-neutral-900 text-neutral-400 transition-colors hover:border-neutral-500 hover:text-white"
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
							class="flex-1 h-10 flex items-center justify-center border border-neutral-700 bg-neutral-900 text-neutral-400 transition-colors hover:border-neutral-500 hover:text-white"
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

					{#if $auth.isAuthenticated}
						<div
							class="mt-4 flex items-center justify-between border-t border-neutral-800 pt-4 text-[10px] font-semibold uppercase tracking-[0.3em] text-neutral-500"
						>
							<span>{$auth.username}</span>
							<button
								class="text-neutral-400 transition-colors hover:text-white"
								onclick={() => logout()}
							>
								LOGOUT
							</button>
						</div>
					{/if}
				</div>
			</div>
		{/if}

		<!-- Desktop Left Sidebar -->
		<aside class="hidden sm:flex {sidebarCollapsed ? 'w-16' : 'w-64'} flex-col border-r border-neutral-800 bg-neutral-950/30 flex-shrink-0 z-20 transition-all duration-200">
			<div class="p-4 flex items-center justify-between">
				{#if !sidebarCollapsed}
					<a
						href="/"
						class="text-xl font-bold tracking-widest uppercase transition-colors hover:text-gray-300"
					>
						Collectarr
					</a>
				{/if}
				<button
					class="h-8 w-8 flex items-center justify-center border border-neutral-800 bg-neutral-900 text-neutral-400 transition-colors hover:border-neutral-600 hover:text-white"
					aria-label={sidebarCollapsed ? 'Expand sidebar' : 'Collapse sidebar'}
					onclick={() => sidebarCollapsed = !sidebarCollapsed}
				>
					<svg class="w-4 h-4 transition-transform duration-200" class:rotate-180={sidebarCollapsed} viewBox="0 0 24 24" fill="currentColor">
						<path d="M15.41 7.41L14 6l-6 6 6 6 1.41-1.41L10.83 12z"/>
					</svg>
				</button>
			</div>
			
			<div class="flex-1 flex flex-col gap-1 px-2 overflow-y-auto">
				{#if !sidebarCollapsed}
					<div class="text-[10px] font-semibold tracking-[0.25em] text-neutral-600 uppercase mb-2 mt-4 px-2">Menu</div>
				{/if}
				<a
					href="/"
					class="flex items-center {sidebarCollapsed ? 'justify-center' : 'gap-3 px-3'} py-2.5 text-sm font-medium tracking-wide uppercase transition-colors {$page.url.pathname === '/' ? 'bg-neutral-800/50 text-white' : 'text-neutral-400 hover:bg-neutral-900 hover:text-neutral-200'}"
					title="Library"
				>
					<svg class="w-5 h-5" viewBox="0 0 24 24" fill="currentColor">
						<path d="M4 6H2v14c0 1.1.9 2 2 2h14v-2H4V6zm16-4H8c-1.1 0-2 .9-2 2v12c0 1.1.9 2 2 2h12c1.1 0 2-.9 2-2V4c0-1.1-.9-2-2-2zm-1 9h-4v4h-2v-4H9V9h4V5h2v4h4v2z"/>
					</svg>
					{#if !sidebarCollapsed}Library{/if}
				</a>
				<a
					href="/playlists"
					class="flex items-center {sidebarCollapsed ? 'justify-center' : 'gap-3 px-3'} py-2.5 text-sm font-medium tracking-wide uppercase transition-colors {$page.url.pathname.startsWith('/playlists') ? 'bg-neutral-800/50 text-white' : 'text-neutral-400 hover:bg-neutral-900 hover:text-neutral-200'}"
					title="Playlists"
				>
					<svg class="w-5 h-5" viewBox="0 0 24 24" fill="currentColor">
						<path d="M3 6h18v2H3zm0 5h18v2H3zm0 5h18v2H3z"/>
					</svg>
					{#if !sidebarCollapsed}Playlists{/if}
				</a>
				<a
					href="/torrents"
					class="flex items-center {sidebarCollapsed ? 'justify-center' : 'gap-3 px-3'} py-2.5 text-sm font-medium tracking-wide uppercase transition-colors {$page.url.pathname === '/torrents' ? 'bg-neutral-800/50 text-white' : 'text-neutral-400 hover:bg-neutral-900 hover:text-neutral-200'}"
					title="Torrents"
				>
					<svg class="w-5 h-5" viewBox="0 0 24 24" fill="currentColor">
						<path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-1 17.93c-3.95-.49-7-3.85-7-7.93 0-.62.08-1.21.21-1.79L9 15v1c0 1.1.9 2 2 2v1.93zm6.9-2.54c-.26-.81-1-1.39-1.9-1.39h-1v-3c0-.55-.45-1-1-1H8v-2h2c.55 0 1-.45 1-1V7h2c1.1 0 2-.9 2-2v-.41c2.93 1.19 5 4.06 5 7.41 0 2.08-.8 3.97-2.1 5.39z"/>
					</svg>
					{#if !sidebarCollapsed}Torrents{/if}
				</a>
				<a
					href="/settings"
					class="flex items-center {sidebarCollapsed ? 'justify-center' : 'gap-3 px-3'} py-2.5 text-sm font-medium tracking-wide uppercase transition-colors {$page.url.pathname === '/settings' ? 'bg-neutral-800/50 text-white' : 'text-neutral-400 hover:bg-neutral-900 hover:text-neutral-200'}"
					title="Settings"
				>
					<svg class="w-5 h-5" viewBox="0 0 24 24" fill="currentColor">
						<path d="M19.14 12.94c.04-.3.06-.61.06-.94 0-.32-.02-.64-.07-.94l2.03-1.58c.18-.14.23-.41.12-.61l-1.92-3.32c-.12-.22-.37-.29-.59-.22l-2.39.96c-.5-.38-1.03-.7-1.62-.94l-.36-2.54c-.04-.24-.24-.41-.48-.41h-3.84c-.24 0-.43.17-.47.41l-.36 2.54c-.59.24-1.13.57-1.62.94l-2.39-.96c-.22-.08-.47 0-.59.22L5.05 8.87c-.12.21-.08.47.12.61l2.03 1.58c-.05.3-.09.63-.09.94s.02.64.07.94l-2.03 1.58c-.18.14-.23.41-.12.61l1.92 3.32c.12.22.37.29.59.22l2.39-.96c.5.38 1.03.7 1.62.94l.36 2.54c.05.24.24.41.48.41h3.84c.24 0 .44-.17.47-.41l.36-2.54c.59-.24 1.13-.56 1.62-.94l2.39.96c.22.08.47 0 .59-.22l1.92-3.32c.12-.22.07-.47-.12-.61l-2.01-1.58zM12 15.6c-1.98 0-3.6-1.62-3.6-3.6s1.62-3.6 3.6-3.6 3.6 1.62 3.6 3.6-1.62 3.6-3.6 3.6z"/>
					</svg>
					{#if !sidebarCollapsed}Settings{/if}
				</a>
			</div>
			
			<div class="p-2 border-t border-neutral-800 flex flex-col gap-3">
				<div class="flex items-center {sidebarCollapsed ? 'justify-center' : 'gap-2'}">
					<button
						class="h-8 w-8 flex items-center justify-center border border-neutral-800 bg-neutral-900 text-neutral-400 transition-colors hover:border-neutral-600 hover:text-white"
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
					{#if !sidebarCollapsed}
						<button
							class="h-8 w-8 flex items-center justify-center border border-neutral-800 bg-neutral-900 text-neutral-400 transition-colors hover:border-neutral-600 hover:text-white"
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
					{/if}
				</div>
				{#if $auth.isAuthenticated && !sidebarCollapsed}
					<div class="flex items-center justify-between text-xs">
						<span class="font-medium text-neutral-400 truncate pr-2">{$auth.username}</span>
						<button
							class="text-neutral-500 font-semibold uppercase tracking-wider hover:text-white transition-colors flex-shrink-0"
							onclick={() => logout()}
						>
							Logout
						</button>
					</div>
				{/if}
			</div>
		</aside>

		<!-- Main Content Area -->
		<main data-shell-scroll="true" class="flex-1 flex flex-col relative z-10 overflow-y-auto bg-[#0a0a0a] sm:border-l sm:border-t sm:border-neutral-800 pt-14 sm:pt-0">
			<Toast />
			<div class="flex-1 w-full h-full flex flex-col">
				{#if !$auth.loading && $auth.isAuthenticated}
					{@render children()}
				{:else}
					<div class="flex-1 flex flex-col items-center justify-center min-h-[50vh]">
						<span class="loading loading-spinner loading-lg text-neutral-500"></span>
					</div>
				{/if}
			</div>
			
			{#if $auth.isAuthenticated}
				<footer class="mt-auto py-6 px-4 sm:px-8">
					<div class="flex flex-col items-center justify-center gap-2 text-neutral-600">
						<span class="text-[10px] uppercase tracking-widest font-medium">© 2026 Collectarr</span>
					</div>
				</footer>
			{/if}
		</main>
	</div>
{/if}
