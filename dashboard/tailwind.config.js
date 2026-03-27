/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{js,ts,jsx,tsx}'],
  theme: {
    extend: {
      colors: {
        brand: {
          50:  '#f0fdf4',
          100: '#dcfce7',
          200: '#bbf7d0',
          300: '#86efac',
          400: '#4ade80',
          500: '#22c55e',
          600: '#16a34a',
          700: '#15803d',
          800: '#166534',
          900: '#14532d',
          950: '#052e16',
        },
        surface: {
          base:    '#09090b',
          raised:  '#111113',
          overlay: '#18181b',
          border:  '#27272a',
          muted:   '#3f3f46',
        },
      },
      keyframes: {
        heartbeat: {
          '0%, 100%': { opacity: '1', transform: 'scale(1)' },
          '50%':      { opacity: '0.4', transform: 'scale(0.85)' },
        },
      },
      animation: {
        heartbeat: 'heartbeat 1.4s ease-in-out infinite',
      },
    },
  },
  plugins: [],
}
