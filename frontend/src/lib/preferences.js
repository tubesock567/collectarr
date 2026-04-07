import { browser } from '$app/environment';
import { writable } from 'svelte/store';

const PREFS_KEY = 'collectarr-preferences';

const defaultPreferences = {
	incognito: false
};

function createPreferencesStore() {
	const { subscribe, set, update } = writable(defaultPreferences);

	function applyPreferences(next) {
		if (!browser) return;
		const preferences = { ...defaultPreferences, ...next };
		document.documentElement.classList.toggle('incognito-mode', preferences.incognito);
		localStorage.setItem(PREFS_KEY, JSON.stringify(preferences));
		set(preferences);
	}

	function initPreferences() {
		if (!browser) return;
		try {
			const raw = localStorage.getItem(PREFS_KEY);
			applyPreferences(raw ? JSON.parse(raw) : defaultPreferences);
		} catch {
			applyPreferences(defaultPreferences);
		}
	}

	return {
		subscribe,
		setPreferences: applyPreferences,
		setIncognito(incognito) {
			update((current) => {
				const next = { ...current, incognito };
				if (browser) {
					applyPreferences(next);
				}
				return next;
			});
		},
		initPreferences
	};
}

export const preferences = createPreferencesStore();
