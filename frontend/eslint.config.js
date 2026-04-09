import js from '@eslint/js';
import svelte from 'eslint-plugin-svelte';
import globals from 'globals';

export default [
	js.configs.recommended,
	...svelte.configs['flat/recommended'],
	{
		languageOptions: {
			globals: {
				...globals.browser,
				...globals.node
			}
		}
	},
	{
		ignores: ['build/', '.svelte-kit/', 'node_modules/']
	},
	{
		rules: {
			'svelte/no-at-html-tags': 'off',
			'svelte/valid-compile': 'off',
			'no-unused-vars': 'off'
		}
	}
];
