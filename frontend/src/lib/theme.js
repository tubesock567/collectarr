import { browser } from '$app/environment';
import { writable } from 'svelte/store';

const THEME_KEY = 'collectarr-theme';
const defaultTheme = 'dark';

function createThemeStore() {
	const { subscribe, set } = writable(defaultTheme);

	function applyTheme(theme) {
		if (!browser) return;
		const nextTheme = theme === 'light' ? 'light' : 'dark';
		document.documentElement.dataset.theme = nextTheme;
		document.documentElement.classList.toggle('theme-light', nextTheme === 'light');
		document.documentElement.classList.toggle('theme-dark', nextTheme === 'dark');
		localStorage.setItem(THEME_KEY, nextTheme);
		set(nextTheme);
	}

	function initTheme() {
		if (!browser) return;
		applyTheme(localStorage.getItem(THEME_KEY) || defaultTheme);
	}

	return {
		subscribe,
		setTheme: applyTheme,
		toggleTheme(currentTheme) {
			applyTheme(currentTheme === 'light' ? 'dark' : 'light');
		},
		initTheme
	};
}

export const theme = createThemeStore();
