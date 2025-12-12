import { useLanguage } from '../contexts/LanguageContext'
import { t } from '../i18n/translations'
import { useSEO } from '../hooks/useSEO'
import { Container } from '../components/Container'
import { Check } from 'lucide-react'
import { OfferSchema, BreadcrumbListSchema } from '../components/StructuredData'

/**
 * Pricing Page
 * 
 * Displays current pricing information (free till testing) and
 * placeholder structure for future pricing tiers.
 * 
 * Accessibility: Full keyboard navigation support, semantic HTML structure
 * Responsive: Mobile-first design with proper breakpoints
 * Dark/Light Mode: Uses CSS variables for theme compatibility
 */
export function PricingPage() {
  const { language } = useLanguage()
  const { SEOComponent } = useSEO()
  const baseUrl = import.meta.env.VITE_BASE_URL || 'https://nofx.ai'

  return (
    <>
      <SEOComponent />
      <OfferSchema offer={{
        name: 'Free Trial',
        price: '0',
        priceCurrency: 'USD',
        availability: 'https://schema.org/InStock',
        url: `${baseUrl}/register`,
      }} />
      <BreadcrumbListSchema items={[
        { name: 'Home', url: '/' },
        { name: 'Pricing', url: '/pricing' },
      ]} />
      <Container className="py-12">
        <div className="max-w-6xl mx-auto">
          {/* Header */}
          <div className="text-center mb-12">
            <h1
              className="text-3xl md:text-4xl font-bold mb-4"
              style={{ color: 'var(--text-primary)' }}
            >
              {t('pricingTitle', language)}
            </h1>
            <p
              className="text-lg max-w-2xl mx-auto"
              style={{ color: 'var(--text-secondary)' }}
            >
              {t('pricingSubtitle', language)}
            </p>
          </div>

          {/* Current Status Banner */}
          <div
            className="rounded-lg p-6 mb-8 text-center border-2"
            style={{
              background: 'var(--navy-dark)',
              borderColor: 'var(--green-primary)',
            }}
          >
            <div className="text-4xl mb-4">🎉</div>
            <h2
              className="text-2xl font-bold mb-2"
              style={{ color: 'var(--green-primary)' }}
            >
              {t('pricingCurrentStatusTitle', language)}
            </h2>
            <p
              className="text-lg font-semibold mb-2"
              style={{ color: 'var(--text-primary)' }}
            >
              {t('pricingCurrentStatusPrice', language)}
            </p>
            <p
              className="text-sm"
              style={{ color: 'var(--text-secondary)' }}
            >
              {t('pricingCurrentStatusDescription', language)}
            </p>
          </div>

          {/* Current Plan Details */}
          <div
            className="rounded-lg p-6 md:p-8 mb-8"
            style={{
              background: 'var(--navy-dark)',
              border: '1px solid var(--panel-border)',
            }}
          >
            <h2
              className="text-2xl font-semibold mb-6 text-center"
              style={{ color: 'var(--text-primary)' }}
            >
              {t('pricingCurrentPlanTitle', language)}
            </h2>
            
            <div className="grid md:grid-cols-2 gap-6">
              <div>
                <h3
                  className="text-lg font-semibold mb-4"
                  style={{ color: 'var(--text-primary)' }}
                >
                  {t('pricingCurrentPlanFeaturesTitle', language)}
                </h3>
                <ul className="space-y-3">
                  {[
                    'pricingFeature1',
                    'pricingFeature2',
                    'pricingFeature3',
                    'pricingFeature4',
                    'pricingFeature5',
                    'pricingFeature6',
                  ].map((featureKey) => (
                    <li key={featureKey} className="flex items-start gap-3">
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
                    </li>
                  ))}
                </ul>
              </div>

              <div>
                <h3
                  className="text-lg font-semibold mb-4"
                  style={{ color: 'var(--text-primary)' }}
                >
                  {t('pricingCurrentPlanLimitationsTitle', language)}
                </h3>
                <ul className="space-y-3">
                  {[
                    'pricingLimitation1',
                    'pricingLimitation2',
                    'pricingLimitation3',
                  ].map((limitationKey) => (
                    <li key={limitationKey} className="flex items-start gap-3">
                      <span className="text-sm" style={{ color: 'var(--text-secondary)' }}>
                        • {t(limitationKey, language)}
                      </span>
                    </li>
                  ))}
                </ul>
              </div>
            </div>
          </div>

          {/* Future Pricing Tiers (Placeholder) */}
          <div className="mb-8">
            <h2
              className="text-2xl font-semibold mb-6 text-center"
              style={{ color: 'var(--text-primary)' }}
            >
              {t('pricingFutureTiersTitle', language)}
            </h2>
            
            <div className="grid md:grid-cols-3 gap-6">
              {/* Professional Tier Placeholder */}
              <div
                className="rounded-lg p-6 border-2 opacity-60"
                style={{
                  background: 'var(--navy-dark)',
                  borderColor: 'var(--panel-border)',
                }}
              >
                <h3
                  className="text-xl font-semibold mb-2"
                  style={{ color: 'var(--text-primary)' }}
                >
                  {t('pricingTierProfessionalTitle', language)}
                </h3>
                <div
                  className="text-2xl font-bold mb-4"
                  style={{ color: 'var(--text-secondary)' }}
                >
                  {t('pricingTierProfessionalPrice', language)}
                </div>
                <p
                  className="text-sm mb-4"
                  style={{ color: 'var(--text-secondary)' }}
                >
                  {t('pricingTierProfessionalDescription', language)}
                </p>
                <div className="text-xs text-center" style={{ color: 'var(--text-tertiary)' }}>
                  {t('pricingComingSoon', language)}
                </div>
              </div>

              {/* Enterprise Tier Placeholder */}
              <div
                className="rounded-lg p-6 border-2 opacity-60"
                style={{
                  background: 'var(--navy-dark)',
                  borderColor: 'var(--panel-border)',
                }}
              >
                <h3
                  className="text-xl font-semibold mb-2"
                  style={{ color: 'var(--text-primary)' }}
                >
                  {t('pricingTierEnterpriseTitle', language)}
                </h3>
                <div
                  className="text-2xl font-bold mb-4"
                  style={{ color: 'var(--text-secondary)' }}
                >
                  {t('pricingTierEnterprisePrice', language)}
                </div>
                <p
                  className="text-sm mb-4"
                  style={{ color: 'var(--text-secondary)' }}
                >
                  {t('pricingTierEnterpriseDescription', language)}
                </p>
                <div className="text-xs text-center" style={{ color: 'var(--text-tertiary)' }}>
                  {t('pricingComingSoon', language)}
                </div>
              </div>

              {/* Custom Tier Placeholder */}
              <div
                className="rounded-lg p-6 border-2 opacity-60"
                style={{
                  background: 'var(--navy-dark)',
                  borderColor: 'var(--panel-border)',
                }}
              >
                <h3
                  className="text-xl font-semibold mb-2"
                  style={{ color: 'var(--text-primary)' }}
                >
                  {t('pricingTierCustomTitle', language)}
                </h3>
                <div
                  className="text-2xl font-bold mb-4"
                  style={{ color: 'var(--text-secondary)' }}
                >
                  {t('pricingTierCustomPrice', language)}
                </div>
                <p
                  className="text-sm mb-4"
                  style={{ color: 'var(--text-secondary)' }}
                >
                  {t('pricingTierCustomDescription', language)}
                </p>
                <div className="text-xs text-center" style={{ color: 'var(--text-tertiary)' }}>
                  {t('pricingComingSoon', language)}
                </div>
              </div>
            </div>
          </div>

          {/* Additional Information */}
          <div
            className="rounded-lg p-6 md:p-8"
            style={{
              background: 'var(--navy-dark)',
              border: '1px solid var(--panel-border)',
            }}
          >
            <h2
              className="text-xl font-semibold mb-4"
              style={{ color: 'var(--text-primary)' }}
            >
              {t('pricingAdditionalInfoTitle', language)}
            </h2>
            <div
              className="text-sm leading-relaxed space-y-3"
              style={{ color: 'var(--text-secondary)' }}
            >
              <p>{t('pricingAdditionalInfo1', language)}</p>
              <p>{t('pricingAdditionalInfo2', language)}</p>
              <p>{t('pricingAdditionalInfo3', language)}</p>
            </div>
          </div>
        </div>
      </Container>
    </>
  )
}
