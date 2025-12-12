import { useLanguage } from '../contexts/LanguageContext'
import { t } from '../i18n/translations'
import { useSEO } from '../hooks/useSEO'
import { Container } from '../components/Container'

/**
 * Risk Disclaimer Page
 * 
 * Critical trading risk warnings for fintech compliance.
 * Displays comprehensive risk disclaimers about cryptocurrency trading.
 * 
 * Accessibility: Full keyboard navigation support, semantic HTML structure
 * Responsive: Mobile-first design with proper breakpoints
 * Dark/Light Mode: Uses CSS variables for theme compatibility
 */
export function RiskDisclaimerPage() {
  const { language } = useLanguage()
  const { SEOComponent } = useSEO()

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
              {t('riskDisclaimerTitle', language)}
            </h1>
            <p
              className="text-sm"
              style={{ color: 'var(--text-secondary)' }}
            >
              {t('riskDisclaimerLastUpdated', language)}
            </p>
          </div>

          {/* Critical Warning Banner */}
          <div
            className="rounded-lg p-6 mb-8 border-2"
            style={{
              background: 'var(--navy-dark)',
              borderColor: 'var(--error)',
            }}
          >
            <div className="flex items-start gap-4">
              <div className="text-3xl">⚠️</div>
              <div>
                <h2
                  className="text-xl font-bold mb-2"
                  style={{ color: 'var(--error)' }}
                >
                  {t('riskDisclaimerCriticalWarning', language)}
                </h2>
                <p
                  className="text-sm leading-relaxed"
                  style={{ color: 'var(--text-secondary)' }}
                >
                  {t('riskDisclaimerCriticalWarningText', language)}
                </p>
              </div>
            </div>
          </div>

          {/* Testing Phase Warning */}
          <section className="mb-8">
            <div
              className="rounded-lg p-6 border-2"
              style={{
                background: 'var(--navy-dark)',
                borderColor: 'var(--error)',
              }}
            >
              <h2
                className="text-xl font-bold mb-4"
                style={{ color: 'var(--error)' }}
              >
                {t('riskDisclaimerTestingPhaseTitle', language)}
              </h2>
              <div
                className="text-sm leading-relaxed space-y-3"
                style={{ color: 'var(--text-secondary)' }}
              >
                <p>{t('riskDisclaimerTestingPhase1', language)}</p>
                <p>{t('riskDisclaimerTestingPhase2', language)}</p>
                <p
                  className="font-semibold"
                  style={{ color: 'var(--error)' }}
                >
                  {t('riskDisclaimerTestingPhase3', language)}
                </p>
                <p>{t('riskDisclaimerTestingPhase4', language)}</p>
                <p>{t('riskDisclaimerTestingPhase5', language)}</p>
              </div>
            </div>
          </section>

          {/* Content */}
          <div
            className="rounded-lg p-6 md:p-8 space-y-8"
            style={{
              background: 'var(--navy-dark)',
              border: '1px solid var(--panel-border)',
            }}
          >
            {/* Introduction */}
            <section>
              <h2
                className="text-xl font-semibold mb-4"
                style={{ color: 'var(--text-primary)' }}
              >
                {t('riskDisclaimerIntroductionTitle', language)}
              </h2>
              <div
                className="text-sm leading-relaxed space-y-3"
                style={{ color: 'var(--text-secondary)' }}
              >
                <p>{t('riskDisclaimerIntroduction1', language)}</p>
                <p>{t('riskDisclaimerIntroduction2', language)}</p>
              </div>
            </section>

            {/* Trading Risks */}
            <section>
              <h2
                className="text-xl font-semibold mb-4"
                style={{ color: 'var(--text-primary)' }}
              >
                {t('riskDisclaimerTradingRisksTitle', language)}
              </h2>
              <div
                className="text-sm leading-relaxed space-y-3"
                style={{ color: 'var(--text-secondary)' }}
              >
                <p>{t('riskDisclaimerTradingRisks1', language)}</p>
                <ul className="list-disc list-inside ml-4 space-y-2">
                  <li>{t('riskDisclaimerTradingRisks2', language)}</li>
                  <li>{t('riskDisclaimerTradingRisks3', language)}</li>
                  <li>{t('riskDisclaimerTradingRisks4', language)}</li>
                  <li>{t('riskDisclaimerTradingRisks5', language)}</li>
                  <li>{t('riskDisclaimerTradingRisks6', language)}</li>
                  <li>{t('riskDisclaimerTradingRisks7', language)}</li>
                </ul>
              </div>
            </section>

            {/* Market Volatility */}
            <section>
              <h2
                className="text-xl font-semibold mb-4"
                style={{ color: 'var(--text-primary)' }}
              >
                {t('riskDisclaimerMarketVolatilityTitle', language)}
              </h2>
              <div
                className="text-sm leading-relaxed space-y-3"
                style={{ color: 'var(--text-secondary)' }}
              >
                <p>{t('riskDisclaimerMarketVolatility1', language)}</p>
                <p>{t('riskDisclaimerMarketVolatility2', language)}</p>
                <p>{t('riskDisclaimerMarketVolatility3', language)}</p>
              </div>
            </section>

            {/* Leverage and Margin Risks */}
            <section>
              <h2
                className="text-xl font-semibold mb-4"
                style={{ color: 'var(--text-primary)' }}
              >
                {t('riskDisclaimerLeverageTitle', language)}
              </h2>
              <div
                className="text-sm leading-relaxed space-y-3"
                style={{ color: 'var(--text-secondary)' }}
              >
                <p>{t('riskDisclaimerLeverage1', language)}</p>
                <p>{t('riskDisclaimerLeverage2', language)}</p>
                <p>{t('riskDisclaimerLeverage3', language)}</p>
              </div>
            </section>

            {/* Platform and Technology Risks */}
            <section>
              <h2
                className="text-xl font-semibold mb-4"
                style={{ color: 'var(--text-primary)' }}
              >
                {t('riskDisclaimerPlatformRisksTitle', language)}
              </h2>
              <div
                className="text-sm leading-relaxed space-y-3"
                style={{ color: 'var(--text-secondary)' }}
              >
                <p>{t('riskDisclaimerPlatformRisks1', language)}</p>
                <ul className="list-disc list-inside ml-4 space-y-2">
                  <li>{t('riskDisclaimerPlatformRisks2', language)}</li>
                  <li>{t('riskDisclaimerPlatformRisks3', language)}</li>
                  <li>{t('riskDisclaimerPlatformRisks4', language)}</li>
                  <li>{t('riskDisclaimerPlatformRisks5', language)}</li>
                </ul>
              </div>
            </section>

            {/* AI and Automated Trading Risks */}
            <section>
              <h2
                className="text-xl font-semibold mb-4"
                style={{ color: 'var(--text-primary)' }}
              >
                {t('riskDisclaimerAIRisksTitle', language)}
              </h2>
              <div
                className="text-sm leading-relaxed space-y-3"
                style={{ color: 'var(--text-secondary)' }}
              >
                <p>{t('riskDisclaimerAIRisks1', language)}</p>
                <p>{t('riskDisclaimerAIRisks2', language)}</p>
                <p>{t('riskDisclaimerAIRisks3', language)}</p>
              </div>
            </section>

            {/* No Guarantees */}
            <section>
              <h2
                className="text-xl font-semibold mb-4"
                style={{ color: 'var(--text-primary)' }}
              >
                {t('riskDisclaimerNoGuaranteesTitle', language)}
              </h2>
              <div
                className="text-sm leading-relaxed space-y-3"
                style={{ color: 'var(--text-secondary)' }}
              >
                <p>{t('riskDisclaimerNoGuarantees1', language)}</p>
                <p>{t('riskDisclaimerNoGuarantees2', language)}</p>
              </div>
            </section>

            {/* Regulatory and Legal Risks */}
            <section>
              <h2
                className="text-xl font-semibold mb-4"
                style={{ color: 'var(--text-primary)' }}
              >
                {t('riskDisclaimerRegulatoryTitle', language)}
              </h2>
              <div
                className="text-sm leading-relaxed space-y-3"
                style={{ color: 'var(--text-secondary)' }}
              >
                <p>{t('riskDisclaimerRegulatory1', language)}</p>
                <p>{t('riskDisclaimerRegulatory2', language)}</p>
              </div>
            </section>

            {/* Final Warning */}
            <section>
              <div
                className="rounded-lg p-6 border-2"
                style={{
                  background: 'var(--navy-primary)',
                  borderColor: 'var(--error)',
                }}
              >
                <h2
                  className="text-xl font-bold mb-4"
                  style={{ color: 'var(--error)' }}
                >
                  {t('riskDisclaimerFinalWarningTitle', language)}
                </h2>
                <p
                  className="text-sm leading-relaxed font-semibold mb-3"
                  style={{ color: 'var(--text-primary)' }}
                >
                  {t('riskDisclaimerFinalWarning1', language)}
                </p>
                <p
                  className="text-sm leading-relaxed"
                  style={{ color: 'var(--text-secondary)' }}
                >
                  {t('riskDisclaimerFinalWarning2', language)}
                </p>
              </div>
            </section>
          </div>
        </div>
      </Container>
    </>
  )
}
