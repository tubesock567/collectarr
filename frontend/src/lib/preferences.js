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
	],
	qbitColumnWidths: {
		name: 250,
		state: 80,
		progress: 100,
		total_size: 80,
		downloaded: 80,
		uploaded: 80,
		ratio: 60,
		eta: 80,
		seeds: 60,
		peers: 60,
		download_speed: 100,
		upload_speed: 100,
		category: 100,
		tags: 100,
		save_path: 200,
		added_on: 150,
		completion_on: 150,
		seeding_time: 100,
		tracker: 40
	},
	qbitColumnOrder: [
		'tracker',
		'name',
		'state',
		'progress',
		'total_size',
		'downloaded',
		'uploaded',
		'ratio',
		'eta',
		'seeds',
		'peers',
		'download_speed',
		'upload_speed',
		'category',
		'tags',
		'save_path',
		'added_on',
		'completion_on',
		'seeding_time'
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
		updateQbitColumnWidths(widths) {
			update((current) => {
				const next = { ...current, qbitColumnWidths: { ...current.qbitColumnWidths, ...widths } };
				if (browser) {
					applyPreferences(next);
				}
				return next;
			});
		},
		updateQbitColumnOrder(order) {
			update((current) => {
				const next = { ...current, qbitColumnOrder: order };
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
