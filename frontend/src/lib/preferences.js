import { browser } from '$app/environment';
import { writable } from 'svelte/store';

const PREFS_KEY = 'collectarr-preferences';

const defaultPreferences = {
	incognito: false,
	qbitColumns: [
		'name',
		'state',
		'progress',
		'total_size',
		'download_speed',
		'upload_speed',
		'eta',
		'ratio'
	]
};

function loadInitialPreferences() {
	if (!browser) return defaultPreferences;
	try {
		const raw = localStorage.getItem(PREFS_KEY);
		return raw ? { ...defaultPreferences, ...JSON.parse(raw) } : defaultPreferences;
	} catch {
		return defaultPreferences;
	}
}

function createPreferencesStore() {
	const { subscribe, set, update } = writable(loadInitialPreferences());

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
		toggleIncognito() {
			update((current) => {
				const next = { ...current, incognito: !current.incognito };
				if (browser) {
					applyPreferences(next);
				}
				return next;
			});
		},
		setIncognito(incognito) {
			update((current) => {
				const next = { ...current, incognito };
				if (browser) {
					applyPreferences(next);
				}
				return next;
			});
		},
		updateQbitColumns(columns) {
			update((current) => {
				const next = { ...current, qbitColumns: columns };
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
