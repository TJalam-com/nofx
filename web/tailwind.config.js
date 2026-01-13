import typography from '@tailwindcss/typography'

/** @type {import('tailwindcss').Config} */
export default {
  content: [
    "./index.html",
    "./src/**/*.{js,ts,jsx,tsx}",
  ],
  theme: {
    extend: {
      typography: {
        DEFAULT: {
          css: {
            color: 'var(--text-primary, #EAECEF)',
            maxWidth: 'none',
            '--tw-prose-body': 'var(--text-primary, #EAECEF)',
            '--tw-prose-headings': 'var(--text-primary, #EAECEF)',
            '--tw-prose-links': 'var(--green-primary, #00FF7F)',
            '--tw-prose-bold': 'var(--text-primary, #EAECEF)',
            '--tw-prose-captions': 'var(--text-secondary, #6b7280)',
            '--tw-prose-code': 'var(--green-primary, #00FF7F)',
            '--tw-prose-pre-code': 'var(--text-primary, #EAECEF)',
            '--tw-prose-pre-bg': 'var(--navy-dark, #00152E)',
            '--tw-prose-th-borders': 'var(--border-color, #e5e7eb)',
            '--tw-prose-td-borders': 'var(--border-color, #e5e7eb)',
            img: {
              borderRadius: '8px',
              marginTop: '1.5rem',
              marginBottom: '1.5rem',
              maxWidth: 'min(100%, 1200px)',
              width: 'auto',
              height: 'auto',
              objectFit: 'none',
              imageRendering: 'auto',
              display: 'block',
              marginLeft: 'auto',
              marginRight: 'auto',
            },
            // Ensure italic text is visible
            'em, i': {
              fontStyle: 'italic',
              fontWeight: 'inherit',
            },
            'strong, b': {
              fontWeight: '700',
            },
          },
        },
      },
    },
  },
  plugins: [typography],
}
