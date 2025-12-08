import { useLanguage } from '../contexts/LanguageContext'
import { t } from '../i18n/translations'

interface FooterProps {
  variant?: 'full' | 'simple'
}

export default function Footer({ variant = 'full' }: FooterProps) {
  const { language } = useLanguage()

  return (
    <footer
      className="mt-16"
      style={{
        borderTop: '1px solid var(--panel-border)',
        background: 'var(--navy-dark)',
      }}
    >
      <div className="max-w-[1200px] mx-auto px-6 py-10">
        {variant === 'full' && (
          <>
            {/* GitHub Links */}
            <div className="flex flex-col items-center gap-4 mb-8">
              <h3
                className="text-sm font-semibold"
                style={{ color: 'var(--text-primary)' }}
              >
                {t('links', language)}
              </h3>
              <div className="flex flex-wrap items-center justify-center gap-4">
                <a
                  className="inline-flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-semibold transition-all duration-300 hover:scale-105"
                  href="https://github.com/NoFxAiOS/nofx"
                  target="_blank"
                  rel="noopener noreferrer"
                  style={{
                    background: 'var(--panel-bg)',
                    color: 'var(--text-primary)',
                    border: '1px solid var(--panel-border)',
                  }}
                  onMouseEnter={(e) => {
                    e.currentTarget.style.borderColor = 'var(--green-primary)'
                    e.currentTarget.style.color = 'var(--green-primary)'
                    e.currentTarget.style.background = 'var(--panel-bg-hover)'
                  }}
                  onMouseLeave={(e) => {
                    e.currentTarget.style.borderColor = 'var(--panel-border)'
                    e.currentTarget.style.color = 'var(--text-primary)'
                    e.currentTarget.style.background = 'var(--panel-bg)'
                  }}
                >
                  <svg
                    width="18"
                    height="18"
                    viewBox="0 0 16 16"
                    fill="currentColor"
                  >
                    <path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z" />
                  </svg>
                  Original Repository
                </a>
                <a
                  className="inline-flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-semibold transition-all duration-300 hover:scale-105"
                  href="https://github.com/NoFxAiOS/nofx/tree/dev"
                  target="_blank"
                  rel="noopener noreferrer"
                  style={{
                    background: 'var(--panel-bg)',
                    color: 'var(--text-primary)',
                    border: '1px solid var(--panel-border)',
                  }}
                  onMouseEnter={(e) => {
                    e.currentTarget.style.borderColor = 'var(--green-primary)'
                    e.currentTarget.style.color = 'var(--green-primary)'
                    e.currentTarget.style.background = 'var(--panel-bg-hover)'
                  }}
                  onMouseLeave={(e) => {
                    e.currentTarget.style.borderColor = 'var(--panel-border)'
                    e.currentTarget.style.color = 'var(--text-primary)'
                    e.currentTarget.style.background = 'var(--panel-bg)'
                  }}
                >
                  <svg
                    width="18"
                    height="18"
                    viewBox="0 0 16 16"
                    fill="currentColor"
                  >
                    <path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z" />
                  </svg>
                  Dev Branch
                </a>
              </div>
            </div>
          </>
        )}

        {/* Risk Warning */}
        <div
          className={`text-center text-xs max-w-4xl mx-auto ${
            variant === 'full' ? 'pt-6 mt-8' : 'pt-4'
          }`}
          style={{
            color: 'var(--text-tertiary)',
            borderTop: variant === 'full' ? '1px solid var(--panel-border)' : 'none',
          }}
        >
          <p className="font-semibold mb-2" style={{ color: 'var(--text-secondary)' }}>
            {t('footerTitle', language)}
          </p>
          <p className="leading-relaxed" style={{ color: 'var(--error)' }}>{t('footerWarning', language)}</p>
          
          {variant === 'simple' && (
            <div className="mt-4">
              <a
                href="https://github.com/NoFxAiOS/nofx"
                target="_blank"
                rel="noopener noreferrer"
                className="inline-flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-semibold transition-all duration-300 hover:scale-105"
                style={{
                  background: 'var(--panel-bg)',
                  color: 'var(--text-primary)',
                  border: '1px solid var(--panel-border)',
                }}
                onMouseEnter={(e) => {
                  e.currentTarget.style.borderColor = 'var(--green-primary)'
                  e.currentTarget.style.color = 'var(--green-primary)'
                  e.currentTarget.style.background = 'var(--panel-bg-hover)'
                }}
                onMouseLeave={(e) => {
                  e.currentTarget.style.borderColor = 'var(--panel-border)'
                  e.currentTarget.style.color = 'var(--text-primary)'
                  e.currentTarget.style.background = 'var(--panel-bg)'
                }}
              >
                <svg
                  width="18"
                  height="18"
                  viewBox="0 0 16 16"
                  fill="currentColor"
                >
                  <path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z" />
                </svg>
                GitHub
              </a>
            </div>
          )}
        </div>
      </div>
    </footer>
  )
}

