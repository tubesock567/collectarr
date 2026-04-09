import { writable } from 'svelte/store';

export const toasts = writable([]);

export function addToast(message, type = 'info', duration = 3000) {
	const id = Date.now();
	toasts.update((items) => [...items, { id, message, type }]);

	if (duration > 0) {
		setTimeout(() => {
			removeToast(id);
		}, duration);
	}
}

export function removeToast(id) {
	toasts.update((items) => items.filter((t) => t.id !== id));
}

export const toast = {
	success: (message, duration) => addToast(message, 'success', duration),
	error: (message, duration) => addToast(message, 'error', duration),
	info: (message, duration) => addToast(message, 'info', duration),
	warning: (message, duration) => addToast(message, 'warning', duration)
};
