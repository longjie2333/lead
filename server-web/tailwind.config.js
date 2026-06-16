/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{vue,js}'],
  theme: {
    extend: {
      colors: {
        primary: '#3482ff',
        ink: '#10151d',
        panel: '#f7f7f7',
        line: '#e8ecf2',
        surface: '#ffffff',
        muted: '#6b7280',
        danger: '#dc2626',
      },
      boxShadow: {
        soft: '0 12px 30px rgb(15 23 42 / 0.08)',
      },
    },
  },
  plugins: [],
}
