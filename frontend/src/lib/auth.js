import { browser } from '$app/environment';
import { goto } from '$app/navigation';
import { get, writable } from 'svelte/store';

const initialState = {
	isAuthenticated: false,
	username: '',
	loading: true
};

export const auth = writable(initialState);

export async function authFetch(url, options = {}) {
	const headers = new Headers(options.headers ?? {});
	if (options.body && !headers.has('Content-Type')) {
		headers.set('Content-Type', 'application/json');
	}

	const response = await fetch(url, {
		...options,
		headers,
		credentials: 'include'
	});

	if (response.status === 401) {
		auth.set({ ...initialState, loading: false });
		if (browser && window.location.pathname !== '/login') {
			goto('/login');
		}
	}

	return response;
}

export async function checkAuth() {
	try {
		const response = await fetch('/api/auth/me', { credentials: 'include' });
		if (!response.ok) {
			auth.set({ ...initialState, loading: false });
			return false;
		}

		const user = await response.json();
		auth.set({
			isAuthenticated: true,
			username: user.username,
			loading: false
		});
		return true;
	} catch {
		auth.set({ ...initialState, loading: false });
		return false;
	}
}

export async function login(username, password) {
	const response = await authFetch('/api/auth/login', {
		method: 'POST',
		body: JSON.stringify({ username, password })
	});

	const data = await readJSON(response);
	if (!response.ok) {
		throw new Error(data?.error || 'Login failed');
	}

	auth.set({
		isAuthenticated: true,
		username: data.username,
		forcePasswordChange: data.force_password_change,
		loading: false
	});

	return data;
}

export async function logout({ redirect = true } = {}) {
	try {
		await authFetch('/api/auth/logout', { method: 'POST' });
	} finally {
		auth.set({ ...initialState, loading: false });
		if (browser && redirect) {
			goto('/login');
		}
	}
}

export function getAuthState() {
	return get(auth);
}

async function readJSON(response) {
	try {
		return await response.json();
	} catch {
		return null;
	}
}
