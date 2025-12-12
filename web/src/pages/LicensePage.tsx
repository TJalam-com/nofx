import { useLanguage } from '../contexts/LanguageContext'
import { t } from '../i18n/translations'
import { useSEO } from '../hooks/useSEO'
import { Container } from '../components/Container'
import { ExternalLink } from 'lucide-react'

/**
 * License Page
 * 
 * AGPL-3.0 license disclosure page (required by AGPL-3.0).
 * Provides information about source code rights and links to GitHub repository.
 * 
 * Accessibility: Full keyboard navigation support, semantic HTML structure
 * Responsive: Mobile-first design with proper breakpoints
 * Dark/Light Mode: Uses CSS variables for theme compatibility
 */
export function LicensePage() {
  const { language } = useLanguage()
  const { SEOComponent } = useSEO()

  const githubRepoUrl = 'https://github.com/TJalam-com/nofx'
  const licenseFileUrl = 'https://github.com/TJalam-com/nofx/blob/main/LICENSE'
  const agplLicenseUrl = 'https://www.gnu.org/licenses/agpl-3.0.html'

  return (
    <>
      <SEOComponent />
      <Container className="py-12">
        <div className="max-w-4xl mx-auto">
          {/* Header */}
          <div className="mb-8">
            <h1
              className="text-3xl md:text-4xl font-bold mb-4"
              style={{ color: 'var(--text-primary)' }}
            >
              {t('licenseTitle', language)}
            </h1>
            <p
              className="text-sm"
              style={{ color: 'var(--text-secondary)' }}
            >
              {t('licenseSubtitle', language)}
            </p>
          </div>

          {/* Content */}
          <div
            className="rounded-lg p-6 md:p-8 space-y-8"
            style={{
              background: 'var(--navy-dark)',
              border: '1px solid var(--panel-border)',
            }}
          >
            {/* License Overview */}
            <section>
              <h2
                className="text-xl font-semibold mb-4"
                style={{ color: 'var(--text-primary)' }}
              >
                {t('licenseOverviewTitle', language)}
              </h2>
              <div
                className="text-sm leading-relaxed space-y-3"
                style={{ color: 'var(--text-secondary)' }}
              >
                <p>{t('licenseOverview1', language)}</p>
                <p>{t('licenseOverview2', language)}</p>
                <p>{t('licenseOverview3', language)}</p>
              </div>
            </section>

            {/* Source Code Access */}
            <section>
              <h2
                className="text-xl font-semibold mb-4"
                style={{ color: 'var(--text-primary)' }}
              >
                {t('licenseSourceCodeTitle', language)}
              </h2>
              <div
                className="text-sm leading-relaxed space-y-4"
                style={{ color: 'var(--text-secondary)' }}
              >
                <p>{t('licenseSourceCode1', language)}</p>
                
                {/* GitHub Repository Link */}
                <div
                  className="rounded-lg p-4"
                  style={{
                    background: 'var(--navy-primary)',
                    border: '1px solid var(--panel-border)',
                  }}
                >
                  <a
                    href={githubRepoUrl}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="inline-flex items-center gap-2 text-sm font-semibold hover:opacity-80 transition-opacity"
                    style={{ color: 'var(--green-primary)' }}
                  >
                    <svg
                      width="18"
                      height="18"
                      viewBox="0 0 16 16"
                      fill="currentColor"
                    >
                      <path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z" />
                    </svg>
                    {githubRepoUrl}
                    <ExternalLink size={14} />
                  </a>
                </div>

                <p>{t('licenseSourceCode2', language)}</p>
              </div>
            </section>

            {/* AGPL-3.0 Rights */}
            <section>
              <h2
                className="text-xl font-semibold mb-4"
                style={{ color: 'var(--text-primary)' }}
              >
                {t('licenseRightsTitle', language)}
              </h2>
              <div
                className="text-sm leading-relaxed space-y-3"
                style={{ color: 'var(--text-secondary)' }}
              >
                <p>{t('licenseRights1', language)}</p>
                <ul className="list-disc list-inside ml-4 space-y-2">
                  <li>{t('licenseRights2', language)}</li>
                  <li>{t('licenseRights3', language)}</li>
                  <li>{t('licenseRights4', language)}</li>
                  <li>{t('licenseRights5', language)}</li>
                </ul>
                <p>{t('licenseRights6', language)}</p>
              </div>
            </section>

            {/* Copyleft Requirement */}
            <section>
              <h2
                className="text-xl font-semibold mb-4"
                style={{ color: 'var(--text-primary)' }}
              >
                {t('licenseCopyleftTitle', language)}
              </h2>
              <div
                className="text-sm leading-relaxed space-y-3"
                style={{ color: 'var(--text-secondary)' }}
              >
                <p>{t('licenseCopyleft1', language)}</p>
                <p>{t('licenseCopyleft2', language)}</p>
                <p>{t('licenseCopyleft3', language)}</p>
              </div>
            </section>

            {/* License File */}
            <section>
              <h2
                className="text-xl font-semibold mb-4"
                style={{ color: 'var(--text-primary)' }}
              >
                {t('licenseFileTitle', language)}
              </h2>
              <div
                className="text-sm leading-relaxed space-y-4"
                style={{ color: 'var(--text-secondary)' }}
              >
                <p>{t('licenseFile1', language)}</p>
                
                {/* LICENSE File Link */}
                <div
                  className="rounded-lg p-4"
                  style={{
                    background: 'var(--navy-primary)',
                    border: '1px solid var(--panel-border)',
                  }}
                >
                  <a
                    href={licenseFileUrl}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="inline-flex items-center gap-2 text-sm font-semibold hover:opacity-80 transition-opacity"
                    style={{ color: 'var(--green-primary)' }}
                  >
                    <ExternalLink size={14} />
                    {t('licenseFileLink', language)}
                  </a>
                </div>
              </div>
            </section>

            {/* Full License Text */}
            <section>
              <h2
                className="text-xl font-semibold mb-4"
                style={{ color: 'var(--text-primary)' }}
              >
                {t('licenseFullTextTitle', language)}
              </h2>
              <div
                className="text-sm leading-relaxed space-y-3"
                style={{ color: 'var(--text-secondary)' }}
              >
                <p>{t('licenseFullText1', language)}</p>
                <a
                  href={agplLicenseUrl}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="inline-flex items-center gap-2 text-sm font-semibold hover:opacity-80 transition-opacity underline"
                  style={{ color: 'var(--green-primary)' }}
                >
                  {t('licenseFullTextLink', language)}
                  <ExternalLink size={14} />
                </a>
              </div>
            </section>

            {/* Contact */}
            <section>
              <h2
                className="text-xl font-semibold mb-4"
                style={{ color: 'var(--text-primary)' }}
              >
                {t('licenseContactTitle', language)}
              </h2>
              <div
                className="text-sm leading-relaxed"
                style={{ color: 'var(--text-secondary)' }}
              >
                <p>{t('licenseContact1', language)}</p>
              </div>
            </section>
          </div>
        </div>
      </Container>
    </>
  )
}
