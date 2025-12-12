import { useLanguage } from '../contexts/LanguageContext'
import { t } from '../i18n/translations'
import { useSEO } from '../hooks/useSEO'
import { Container } from '../components/Container'
import { Shield, Lock, Key, Eye, AlertTriangle, CheckCircle } from 'lucide-react'

/**
 * Security Page
 * 
 * Builds trust by explaining security measures, encryption,
 * and best practices implemented in the platform.
 * 
 * Accessibility: Full keyboard navigation support, semantic HTML structure
 * Responsive: Mobile-first design with proper breakpoints
 * Dark/Light Mode: Uses CSS variables for theme compatibility
 */
export function SecurityPage() {
  const { language } = useLanguage()
  const { SEOComponent } = useSEO()

  const securityFeatures = [
    {
      icon: Lock,
      titleKey: 'securityEncryptionTitle',
      descKey: 'securityEncryptionDesc',
      items: [
        'securityEncryption1',
        'securityEncryption2',
        'securityEncryption3',
        'securityEncryption4',
      ],
    },
    {
      icon: Key,
      titleKey: 'securityCredentialsTitle',
      descKey: 'securityCredentialsDesc',
      items: [
        'securityCredentials1',
        'securityCredentials2',
        'securityCredentials3',
        'securityCredentials4',
      ],
    },
    {
      icon: Shield,
      titleKey: 'securityAuthenticationTitle',
      descKey: 'securityAuthenticationDesc',
      items: [
        'securityAuthentication1',
        'securityAuthentication2',
        'securityAuthentication3',
        'securityAuthentication4',
      ],
    },
    {
      icon: Eye,
      titleKey: 'securityMonitoringTitle',
      descKey: 'securityMonitoringDesc',
      items: [
        'securityMonitoring1',
        'securityMonitoring2',
        'securityMonitoring3',
        'securityMonitoring4',
      ],
    },
  ]

  return (
    <>
      <SEOComponent />
      <Container className="py-12">
        <div className="max-w-4xl mx-auto">
          {/* Header */}
          <div className="text-center mb-12">
            <div className="flex justify-center mb-4">
              <div
                className="rounded-full p-4"
                style={{ background: 'var(--navy-dark)' }}
              >
                <Shield size={48} style={{ color: 'var(--green-primary)' }} />
              </div>
            </div>
            <h1
              className="text-3xl md:text-4xl font-bold mb-4"
              style={{ color: 'var(--text-primary)' }}
            >
              {t('securityTitle', language)}
            </h1>
            <p
              className="text-lg max-w-2xl mx-auto"
              style={{ color: 'var(--text-secondary)' }}
            >
              {t('securitySubtitle', language)}
            </p>
          </div>

          {/* Security Commitment Banner */}
          <div
            className="rounded-lg p-6 mb-8 border-2"
            style={{
              background: 'var(--navy-dark)',
              borderColor: 'var(--green-primary)',
            }}
          >
            <div className="flex items-start gap-4">
              <CheckCircle size={24} style={{ color: 'var(--green-primary)' }} className="flex-shrink-0 mt-1" />
              <div>
                <h2
                  className="text-xl font-semibold mb-2"
                  style={{ color: 'var(--text-primary)' }}
                >
                  {t('securityCommitmentTitle', language)}
                </h2>
                <p
                  className="text-sm leading-relaxed"
                  style={{ color: 'var(--text-secondary)' }}
                >
                  {t('securityCommitment', language)}
                </p>
              </div>
            </div>
          </div>

          {/* Security Features */}
          <div className="space-y-6 mb-8">
            {securityFeatures.map((feature) => {
              const Icon = feature.icon
              return (
                <div
                  key={feature.titleKey}
                  className="rounded-lg p-6"
                  style={{
                    background: 'var(--navy-dark)',
                    border: '1px solid var(--panel-border)',
                  }}
                >
                  <div className="flex items-start gap-4 mb-4">
                    <Icon size={24} style={{ color: 'var(--green-primary)' }} className="flex-shrink-0 mt-1" />
                    <div className="flex-1">
                      <h3
                        className="text-lg font-semibold mb-2"
                        style={{ color: 'var(--text-primary)' }}
                      >
                        {t(feature.titleKey, language)}
                      </h3>
                      <p
                        className="text-sm mb-4"
                        style={{ color: 'var(--text-secondary)' }}
                      >
                        {t(feature.descKey, language)}
                      </p>
                      <ul className="space-y-2">
                        {feature.items.map((itemKey) => (
                          <li key={itemKey} className="flex items-start gap-2">
                            <CheckCircle size={16} style={{ color: 'var(--green-primary)' }} className="flex-shrink-0 mt-0.5" />
                            <span
                              className="text-sm"
                              style={{ color: 'var(--text-secondary)' }}
                            >
                              {t(itemKey, language)}
                            </span>
                          </li>
                        ))}
                      </ul>
                    </div>
                  </div>
                </div>
              )
            })}
          </div>

          {/* Best Practices */}
          <div
            className="rounded-lg p-6 mb-8"
            style={{
              background: 'var(--navy-dark)',
              border: '1px solid var(--panel-border)',
            }}
          >
            <h2
              className="text-xl font-semibold mb-4"
              style={{ color: 'var(--text-primary)' }}
            >
              {t('securityBestPracticesTitle', language)}
            </h2>
            <div
              className="text-sm leading-relaxed space-y-3"
              style={{ color: 'var(--text-secondary)' }}
            >
              <p>{t('securityBestPractices1', language)}</p>
              <p>{t('securityBestPractices2', language)}</p>
              <p>{t('securityBestPractices3', language)}</p>
            </div>
          </div>

          {/* Security Reporting */}
          <div
            className="rounded-lg p-6 border-2"
            style={{
              background: 'var(--navy-dark)',
              borderColor: 'var(--error)',
            }}
          >
            <div className="flex items-start gap-4">
              <AlertTriangle size={24} style={{ color: 'var(--error)' }} className="flex-shrink-0 mt-1" />
              <div>
                <h2
                  className="text-xl font-semibold mb-2"
                  style={{ color: 'var(--text-primary)' }}
                >
                  {t('securityReportingTitle', language)}
                </h2>
                <div
                  className="text-sm leading-relaxed space-y-3"
                  style={{ color: 'var(--text-secondary)' }}
                >
                  <p>{t('securityReporting1', language)}</p>
                  <p>{t('securityReporting2', language)}</p>
                  <p>{t('securityReporting3', language)}</p>
                </div>
              </div>
            </div>
          </div>
        </div>
      </Container>
    </>
  )
}