/** @type {import('tailwindcss').Config} */
module.exports = {
  darkMode: 'class',
  content: [
    "./web/**/*.html",
    "./web/**/*.js"
  ],
  theme: {
    extend: {
      colors: {
        brand: {
          50: '#eef2ff',
          100: '#e0e7ff',
          500: '#6366f1',
          600: '#4f46e5',
          700: '#4338ca',
        },
        gdrive: {
          blue: '#1a73e8',
          hover: '#1557b0',
          bg: '#f8fafd',
          sidebar: '#f8fafd',
          border: '#dadce0',
          text: '#3c4043',
          heading: '#202124'
        }
      }
    }
  },
  plugins: [],
}
