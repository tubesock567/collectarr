import daisyui from 'daisyui';

/** @type {import('tailwindcss').Config} */
export default {
	content: ['./src/**/*.{html,js,svelte,ts}'],
	darkMode: 'class',
	theme: {
		extend: {}
	},
	plugins: [daisyui],
	daisyui: {
		themes: ['black'],
		darkTheme: 'black'
	}
};
