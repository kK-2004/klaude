import type { Config } from 'tailwindcss'

export default {
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  theme: {
    extend: {
      colors: {
        ink: 'var(--text)',
        panel: 'var(--bg-sidebar)',
        panelRaised: 'var(--bg-hover)',
        line: 'var(--border)',
        accent: 'var(--text)',
        success: 'var(--ok)',
        warning: 'var(--notice-fg)',
      },
      fontFamily: {
        sans: ['-apple-system', 'BlinkMacSystemFont', 'Segoe UI', 'PingFang SC', 'Hiragino Sans GB', 'Noto Sans SC', 'Microsoft YaHei', 'sans-serif'],
      },
    },
  },
  plugins: [],
} satisfies Config
