import { useLanguage } from '../contexts/LanguageContext'
import { t } from '../i18n/translations'
import { useSEO } from '../hooks/useSEO'
import { Container } from '../components/Container'
import { Check, Zap, Shield, GitBranch, BarChart3, Webhook, Users, Cpu } from 'lucide-react'
import { SoftwareApplicationSchema, BreadcrumbListSchema } from '../components/StructuredData'

/**
 * Features Page
 * 
 * Deep dive into platform capabilities, showcasing all features
 * and functionality of AI Trading 24x7.
 * 
 * Accessibility: Full keyboard navigation support, semantic HTML structure
 * Responsive: Mobile-first design with proper breakpoints
 * Dark/Light Mode: Uses CSS variables for theme compatibility
 */
export function FeaturesPage() {
  const { language } = useLanguage()
  const { SEOComponent } = useSEO()
  const baseUrl = import.meta.env.VITE_BASE_URL || 'https://nofx.ai'

  const featureCategories = [
    {
      icon: Cpu,
      titleKey: 'featuresAI',
      descKey: 'featuresAIDesc',
      features: [
        'featuresAI1',
        'featuresAI2',
        'featuresAI3',
        'featuresAI4',
        'featuresAI5',
      ],
    },
    {
      icon: BarChart3,
      titleKey: 'featuresTrading',
      descKey: 'featuresTradingDesc',
      features: [
        'featuresTrading1',
        'featuresTrading2',
        'featuresTrading3',
        'featuresTrading4',
        'featuresTrading5',
      ],
    },
    {
      icon: GitBranch,
      titleKey: 'featuresMultiAgent',
      descKey: 'featuresMultiAgentDesc',
      features: [
        'featuresMultiAgent1',
        'featuresMultiAgent2',
        'featuresMultiAgent3',
        'featuresMultiAgent4',
      ],
    },
    {
      icon: Users,
      titleKey: 'featuresCopyTrading',
      descKey: 'featuresCopyTradingDesc',
      features: [
        'featuresCopyTrading1',
        'featuresCopyTrading2',
        'featuresCopyTrading3',
        'featuresCopyTrading4',
      ],
    },
    {
      icon: Webhook,
      titleKey: 'featuresIntegration',
      descKey: 'featuresIntegrationDesc',
      features: [
        'featuresIntegration1',
        'featuresIntegration2',
        'featuresIntegration3',
        'featuresIntegration4',
      ],
    },
    {
      icon: Shield,
      titleKey: 'featuresSecurity',
      descKey: 'featuresSecurityDesc',
      features: [
        'featuresSecurity1',
        'featuresSecurity2',
        'featuresSecurity3',
        'featuresSecurity4',
      ],
    },
  ]

  return (
    <>
      <SEOComponent />
      <SoftwareApplicationSchema
        name="AI Trading 24x7"
        description="AI-powered copy trading platform with multi-agent competition, webhook integration, and advanced security features"
        applicationCategory="FinanceApplication"
        operatingSystem="Web"
        url={`${baseUrl}/features`}
      />
      <BreadcrumbListSchema items={[
        { name: 'Home', url: '/' },
        { name: 'Features', url: '/features' },
      ]} />
      <Container className="py-12">
        <div className="max-w-6xl mx-auto">
          {/* Header */}
          <div className="text-center mb-12">
            <h1
              className="text-3xl md:text-4xl font-bold mb-4"
              style={{ color: 'var(--text-primary)' }}
            >
              {t('featuresTitle', language)}
            </h1>
            <p
              className="text-lg max-w-2xl mx-auto"
              style={{ color: 'var(--text-secondary)' }}
            >
              {t('featuresSubtitle', language)}
            </p>
          </div>

          {/* Feature Categories */}
          <div className="space-y-8">
            {featureCategories.map((category) => {
              const Icon = category.icon
              return (
                <div
                  key={category.titleKey}
                  className="rounded-lg p-6 md:p-8"
                  style={{
                    background: 'var(--navy-dark)',
                    border: '1px solid var(--panel-border)',
                  }}
                >
                  <div className="flex items-start gap-4 mb-6">
                    <div
                      className="rounded-lg p-3 flex-shrink-0"
                      style={{ background: 'var(--navy-primary)' }}
                    >
                      <Icon size={32} style={{ color: 'var(--green-primary)' }} />
                    </div>
                    <div className="flex-1">
                      <h2
                        className="text-2xl font-semibold mb-2"
                        style={{ color: 'var(--text-primary)' }}
                      >
                        {t(category.titleKey, language)}
                      </h2>
                      <p
                        className="text-sm"
                        style={{ color: 'var(--text-secondary)' }}
                      >
                        {t(category.descKey, language)}
                      </p>
                    </div>
                  </div>

                  <div className="grid md:grid-cols-2 gap-4">
                    {category.features.map((featureKey) => (
                      <div key={featureKey} className="flex items-start gap-3">
                        <Check
                          size={20}
                          className="flex-shrink-0 mt-0.5"
                          style={{ color: 'var(--green-primary)' }}
                        />
                        <span
                          className="text-sm"
                          style={{ color: 'var(--text-secondary)' }}
                        >
                          {t(featureKey, language)}
                        </span>
                      </div>
                    ))}
                  </div>
                </div>
              )
            })}
          </div>

          {/* Technical Highlights */}
          <div className="mt-12">
            <div
              className="rounded-lg p-6 md:p-8 border-2"
              style={{
                background: 'var(--navy-dark)',
                borderColor: 'var(--green-primary)',
              }}
            >
              <h2
                className="text-2xl font-semibold mb-6"
                style={{ color: 'var(--text-primary)' }}
              >
                {t('featuresTechnicalTitle', language)}
              </h2>
              <div className="grid md:grid-cols-3 gap-6">
                {[
                  'featuresTechnical1',
                  'featuresTechnical2',
                  'featuresTechnical3',
                  'featuresTechnical4',
                  'featuresTechnical5',
                  'featuresTechnical6',
                ].map((key) => (
                  <div key={key} className="flex items-start gap-3">
                    <Zap size={18} style={{ color: 'var(--green-primary)' }} className="flex-shrink-0 mt-1" />
                    <span
                      className="text-sm"
                      style={{ color: 'var(--text-secondary)' }}
                    >
                      {t(key, language)}
                    </span>
                  </div>
                ))}
              </div>
            </div>
          </div>
        </div>
      </Container>
    </>
  )
}