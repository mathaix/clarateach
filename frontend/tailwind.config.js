/** @type {import('tailwindcss').Config} */
export default {
  content: ['./index.html', './src/**/*.{js,ts,jsx,tsx}'],
  theme: {
    extend: {
      colors: {
        'vscode-bg': '#1e1e1e',
        'vscode-sidebar': '#252526',
        'vscode-header': '#323233',
        'vscode-border': '#3e3e3e',
        'vscode-text': '#cccccc',
        'vscode-accent': '#4ec9b0',
        'vscode-blue': '#569cd6',
      },
    },
  },
  plugins: [],
};
