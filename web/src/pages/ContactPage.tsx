import { useLanguage } from '../contexts/LanguageContext'
import { t } from '../i18n/translations'
import { useSEO } from '../hooks/useSEO'
import { Container } from '../components/Container'
import { Mail, AlertCircle, HelpCircle, Bug } from 'lucide-react'
import {
  OrganizationSchema,
  BreadcrumbListSchema,
} from '../components/StructuredData'

/**
 * Contact Page
 *
 * Provides support channels and contact information for users
 * to get help, report issues, or reach out to the team.
 *
 * Accessibility: Full keyboard navigation support, semantic HTML structure
 * Responsive: Mobile-first design with proper breakpoints
 * Dark/Light Mode: Uses CSS variables for theme compatibility
 */
export function ContactPage() {
  const { language } = useLanguage()
  const { SEOComponent } = useSEO()

  const contactEmail = 'contact@tjalam.com'

  const supportTypes = [
    {
      icon: HelpCircle,
      titleKey: 'contactGeneralSupportTitle',
      descKey: 'contactGeneralSupportDesc',
    },
    {
      icon: Bug,
      titleKey: 'contactBugReportTitle',
      descKey: 'contactBugReportDesc',
    },
    {
      icon: AlertCircle,
      titleKey: 'contactSecurityTitle',
      descKey: 'contactSecurityDesc',
    },
  ]

  return (
    <>
      <SEOComponent />
      <OrganizationSchema />
      <BreadcrumbListSchema
        items={[
          { name: 'Home', url: '/' },
          { name: 'Contact', url: '/contact' },
        ]}
      />
      <Container className="py-12">
        <div className="max-w-4xl mx-auto">
          {/* Header */}
          <div className="text-center mb-12">
            <h1
              className="text-3xl md:text-4xl font-bold mb-4"
              style={{ color: 'var(--text-primary)' }}
            >
              {t('contactTitle', language)}
            </h1>
            <p
              className="text-lg max-w-2xl mx-auto"
              style={{ color: 'var(--text-secondary)' }}
            >
              {t('contactSubtitle', language)}
            </p>
          </div>

          {/* Email Contact */}
          <div className="mb-12">
            <a
              href={`mailto:${contactEmail}`}
              className="rounded-lg p-8 border-2 hover:border-green-primary transition-colors block text-center"
              style={{
                background: 'var(--navy-dark)',
                borderColor: 'var(--green-primary)',
              }}
            >
              <Mail
                size={48}
                style={{ color: 'var(--green-primary)' }}
                className="mx-auto mb-4"
              />
              <h3
                className="text-xl font-semibold mb-2"
                style={{ color: 'var(--text-primary)' }}
              >
                {t('contactEmailTitle', language)}
              </h3>
              <p
                className="text-sm mb-4"
                style={{ color: 'var(--text-secondary)' }}
              >
                {t('contactEmailDesc', language)}
              </p>
              <span
                className="text-lg font-semibold"
                style={{ color: 'var(--green-primary)' }}
              >
                {contactEmail}
              </span>
            </a>
          </div>

          {/* Support Types */}
          <div
            className="rounded-lg p-6 md:p-8 mb-8"
            style={{
              background: 'var(--navy-dark)',
              border: '1px solid var(--panel-border)',
            }}
          >
            <h2
              className="text-2xl font-semibold mb-6"
              style={{ color: 'var(--text-primary)' }}
            >
              {t('contactSupportTypesTitle', language)}
            </h2>
            <div className="space-y-6">
              {supportTypes.map((type) => {
                const Icon = type.icon
                return (
                  <div key={type.titleKey} className="flex items-start gap-4">
                    <Icon
                      size={24}
                      style={{ color: 'var(--green-primary)' }}
                      className="flex-shrink-0 mt-1"
                    />
                    <div>
                      <h3
                        className="text-lg font-semibold mb-2"
                        style={{ color: 'var(--text-primary)' }}
                      >
                        {t(type.titleKey, language)}
                      </h3>
                      <p
                        className="text-sm leading-relaxed"
                        style={{ color: 'var(--text-secondary)' }}
                      >
                        {t(type.descKey, language)}
                      </p>
                    </div>
                  </div>
                )
              })}
            </div>
          </div>

          {/* Response Times */}
          <div
            className="rounded-lg p-6 border-2"
            style={{
              background: 'var(--navy-dark)',
              borderColor: 'var(--green-primary)',
            }}
          >
            <h2
              className="text-xl font-semibold mb-4"
              style={{ color: 'var(--text-primary)' }}
            >
              {t('contactResponseTitle', language)}
            </h2>
            <div
              className="text-sm leading-relaxed space-y-3"
              style={{ color: 'var(--text-secondary)' }}
            >
              <p>{t('contactResponse1', language)}</p>
              <p>{t('contactResponse2', language)}</p>
              <p>{t('contactResponse3', language)}</p>
            </div>
          </div>
        </div>
      </Container>
    </>
  )
}
