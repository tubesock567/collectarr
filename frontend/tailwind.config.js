import daisyui from 'daisyui';

/** @type {import('tailwindcss').Config} */
export default {
  content: ['./src/**/*.{html,js,svelte,ts}'],
  theme: {
    extend: {
      screens: {
        '3xl': '1920px',
      },
    },
  },
  plugins: [daisyui],
  daisyui: {
    themes: ['black'],
    darkTheme: 'black',
  },
};
