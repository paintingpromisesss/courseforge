/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{js,ts,jsx,tsx}'],
  theme: {
    extend: {
      fontFamily: {
        sans: ['Inter', 'system-ui', 'sans-serif'],
      },
      colors: {
        brand: {
          DEFAULT: '#8251EE',
          hover: '#9366F5',
          subtle: 'rgba(130,81,238,0.15)',
        },
        bg: {
          1: 'var(--bg-1)',
          2: 'var(--bg-2)',
          3: 'var(--bg-3)',
          4: 'var(--bg-4)',
          5: 'var(--bg-5)',
          6: 'var(--bg-6)',
        },
        tx: {
          1: 'var(--tx-1)',
          2: 'var(--tx-2)',
          3: 'var(--tx-3)',
        },
        bdr: {
          s: 'var(--bdr-s)',
          DEFAULT: 'var(--bdr)',
          e: 'var(--bdr-e)',
        },
        ok:   '#10B981',
        warn: '#F59E0B',
        err:  '#EF4444',
      },
      borderRadius: { DEFAULT: '0.5rem', lg: '0.75rem', xl: '1rem' },
    },
  },
  plugins: [require('@tailwindcss/typography')],
};
