import { useLanguage } from '../contexts/LanguageContext'
import { t } from '../i18n/translations'
import { useSEO } from '../hooks/useSEO'
import { Container } from '../components/Container'
import { OrganizationSchema, BreadcrumbListSchema } from '../components/StructuredData'

/**
 * About Us Page
 * 
 * Tells the story of AI Trading 24x7, builds credibility, and explains
 * the mission and vision of the platform.
 * 
 * Accessibility: Full keyboard navigation support, semantic HTML structure
 * Responsive: Mobile-first design with proper breakpoints
 * Dark/Light Mode: Uses CSS variables for theme compatibility
 */
export function AboutPage() {
  const { language } = useLanguage()
  const { SEOComponent } = useSEO()
  
  return (
    <>
      <SEOComponent />
      <OrganizationSchema />
      <BreadcrumbListSchema items={[
        { name: 'Home', url: '/' },
        { name: 'About', url: '/about' },
      ]} />
      <Container className="py-12">
        <div className="max-w-4xl mx-auto">
          {/* Header */}
          <div className="text-center mb-12">
            <h1
              className="text-3xl md:text-4xl font-bold mb-4"
              style={{ color: 'var(--text-primary)' }}
            >
              {t('aboutTitle', language)}
            </h1>
            <p
              className="text-lg max-w-2xl mx-auto"
              style={{ color: 'var(--text-secondary)' }}
            >
              {t('aboutSubtitle', language)}
            </p>
          </div>

          {/* Content */}
          <div className="space-y-12">
            {/* Attribution */}
            <section>
              <div
                className="rounded-lg p-6 md:p-8 border-2 text-center"
                style={{
                  background: 'var(--navy-dark)',
                  borderColor: 'var(--green-primary)',
                }}
              >
                <h2
                  className="text-xl font-semibold mb-4"
                  style={{ color: 'var(--text-primary)' }}
                >
                  Attribution & Licensing
                </h2>
                <div
                  className="text-sm leading-relaxed"
                  style={{ color: 'var(--text-secondary)' }}
                >
                  <p className="font-medium mb-2">
                    Built on NOFX (AGPL‑3.0) – original project at{' '}
                    <a
                      href="https://github.com/NoFxAiOS/nofx"
                      target="_blank"
                      rel="noopener noreferrer"
                      className="hover:opacity-80 transition-opacity underline"
                      style={{ color: 'var(--green-primary)' }}
                    >
                      github.com/NoFxAiOS/nofx
                    </a>
                  </p>
                  <p>
                    This is a modified version of the NOFX software, rebranded as AI Trading 24x7.
                    All modifications are licensed under AGPL-3.0 and the complete source code is publicly available.
                  </p>
                </div>
              </div>
            </section>

            {/* Open Source Commitment */}
            <section>
              <div
                className="rounded-lg p-6 md:p-8 border-2"
                style={{
                  background: 'var(--navy-dark)',
                  borderColor: 'var(--green-primary)',
                }}
              >
                <h2
                  className="text-2xl font-semibold mb-4"
                  style={{ color: 'var(--text-primary)' }}
                >
                  {t('aboutOpenSourceTitle', language)}
                </h2>
                <div
                  className="text-sm leading-relaxed space-y-3"
                  style={{ color: 'var(--text-secondary)' }}
                >
                  <p>{t('aboutOpenSource1', language)}</p>
                  <p>{t('aboutOpenSource2', language)}</p>
                  <p>{t('aboutOpenSource3', language)}</p>
                </div>
              </div>
            </section>

            {/* Community */}
            <section>
              <div
                className="rounded-lg p-6 md:p-8"
                style={{
                  background: 'var(--navy-dark)',
                  border: '1px solid var(--panel-border)',
                }}
              >
                <h2
                  className="text-2xl font-semibold mb-4"
                  style={{ color: 'var(--text-primary)' }}
                >
                  {t('aboutCommunityTitle', language)}
                </h2>
                <div
                  className="text-sm leading-relaxed space-y-3"
                  style={{ color: 'var(--text-secondary)' }}
                >
                  <p>{t('aboutCommunity1', language)}</p>
                  <p>{t('aboutCommunity2', language)}</p>
                </div>
              </div>
            </section>
          </div>
        </div>
      </Container>
    </>
  )
}
