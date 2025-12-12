import { useLanguage } from '../contexts/LanguageContext'
import { t } from '../i18n/translations'
import { useSEO } from '../hooks/useSEO'
import { Container } from '../components/Container'
import { Target, Users, Zap, Code, TrendingUp } from 'lucide-react'
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

  const baseUrl = import.meta.env.VITE_BASE_URL || 'https://nofx.ai'
  
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
            {/* Our Story */}
            <section>
              <div
                className="rounded-lg p-6 md:p-8"
                style={{
                  background: 'var(--navy-dark)',
                  border: '1px solid var(--panel-border)',
                }}
              >
                <h2
                  className="text-2xl font-semibold mb-6"
                  style={{ color: 'var(--text-primary)' }}
                >
                  {t('aboutStoryTitle', language)}
                </h2>
                <div
                  className="text-sm leading-relaxed space-y-4"
                  style={{ color: 'var(--text-secondary)' }}
                >
                  <p>{t('aboutStory1', language)}</p>
                  <p>{t('aboutStory2', language)}</p>
                  <p>{t('aboutStory3', language)}</p>
                </div>
              </div>
            </section>

            {/* Mission & Vision */}
            <section>
              <div className="grid md:grid-cols-2 gap-6">
                <div
                  className="rounded-lg p-6 border-2"
                  style={{
                    background: 'var(--navy-dark)',
                    borderColor: 'var(--green-primary)',
                  }}
                >
                  <div className="flex items-center gap-3 mb-4">
                    <Target size={24} style={{ color: 'var(--green-primary)' }} />
                    <h3
                      className="text-xl font-semibold"
                      style={{ color: 'var(--text-primary)' }}
                    >
                      {t('aboutMissionTitle', language)}
                    </h3>
                  </div>
                  <p
                    className="text-sm leading-relaxed"
                    style={{ color: 'var(--text-secondary)' }}
                  >
                    {t('aboutMission', language)}
                  </p>
                </div>

                <div
                  className="rounded-lg p-6 border-2"
                  style={{
                    background: 'var(--navy-dark)',
                    borderColor: 'var(--green-primary)',
                  }}
                >
                  <div className="flex items-center gap-3 mb-4">
                    <TrendingUp size={24} style={{ color: 'var(--green-primary)' }} />
                    <h3
                      className="text-xl font-semibold"
                      style={{ color: 'var(--text-primary)' }}
                    >
                      {t('aboutVisionTitle', language)}
                    </h3>
                  </div>
                  <p
                    className="text-sm leading-relaxed"
                    style={{ color: 'var(--text-secondary)' }}
                  >
                    {t('aboutVision', language)}
                  </p>
                </div>
              </div>
            </section>

            {/* Core Values */}
            <section>
              <div
                className="rounded-lg p-6 md:p-8"
                style={{
                  background: 'var(--navy-dark)',
                  border: '1px solid var(--panel-border)',
                }}
              >
                <h2
                  className="text-2xl font-semibold mb-6"
                  style={{ color: 'var(--text-primary)' }}
                >
                  {t('aboutValuesTitle', language)}
                </h2>
                <div className="grid md:grid-cols-2 gap-6">
                  {[
                    { icon: Code, key: 'aboutValue1' },
                    { icon: Zap, key: 'aboutValue2' },
                    { icon: Users, key: 'aboutValue3' },
                    { icon: Target, key: 'aboutValue4' },
                  ].map(({ icon: Icon, key }) => (
                    <div key={key} className="flex items-start gap-4">
                      <Icon size={24} style={{ color: 'var(--green-primary)' }} className="flex-shrink-0 mt-1" />
                      <div>
                        <h3
                          className="text-lg font-semibold mb-2"
                          style={{ color: 'var(--text-primary)' }}
                        >
                          {t(`${key}Title`, language)}
                        </h3>
                        <p
                          className="text-sm leading-relaxed"
                          style={{ color: 'var(--text-secondary)' }}
                        >
                          {t(`${key}Desc`, language)}
                        </p>
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            </section>

            {/* Team & Backing */}
            <section>
              <div
                className="rounded-lg p-6 md:p-8"
                style={{
                  background: 'var(--navy-dark)',
                  border: '1px solid var(--panel-border)',
                }}
              >
                <h2
                  className="text-2xl font-semibold mb-6"
                  style={{ color: 'var(--text-primary)' }}
                >
                  {t('aboutTeamTitle', language)}
                </h2>
                <div
                  className="text-sm leading-relaxed space-y-4"
                  style={{ color: 'var(--text-secondary)' }}
                >
                  <p>{t('aboutTeam1', language)}</p>
                  <p>{t('aboutTeam2', language)}</p>
                  <div className="mt-6 p-4 rounded-lg" style={{ background: 'var(--navy-primary)' }}>
                    <p className="font-semibold mb-2" style={{ color: 'var(--text-primary)' }}>
                      {t('aboutBackingTitle', language)}
                    </p>
                    <p style={{ color: 'var(--text-secondary)' }}>
                      {t('aboutBacking', language)}
                    </p>
                  </div>
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
