import { useLanguage } from '../contexts/LanguageContext'
import { t } from '../i18n/translations'
import { useSEO } from '../hooks/useSEO'
import { Container } from '../components/Container'

/**
 * Terms and Conditions Page
 * 
 * Displays comprehensive terms and conditions for trading platform usage,
 * including risk warnings, disclaimers, and AGPL-3.0 license reference.
 * 
 * Accessibility: Full keyboard navigation support, semantic HTML structure
 * Responsive: Mobile-first design with proper breakpoints
 * Dark/Light Mode: Uses CSS variables for theme compatibility
 */
export function TermsAndConditionsPage() {
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
              {t('termsTitle', language)}
            </h1>
            <p
              className="text-sm"
              style={{ color: 'var(--text-secondary)' }}
            >
              {t('termsLastUpdated', language)}
            </p>
          </div>

          {/* Terms Content */}
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
                {t('termsIntroductionTitle', language)}
              </h2>
              <div
                className="text-sm leading-relaxed space-y-3"
                style={{ color: 'var(--text-secondary)' }}
              >
                <p>{t('termsIntroduction1', language)}</p>
                <p>{t('termsIntroduction2', language)}</p>
              </div>
            </section>

            {/* Testing Phase */}
            <section>
              <h2
                className="text-xl font-semibold mb-4"
                style={{ color: 'var(--text-primary)' }}
              >
                {t('termsTestingPhaseTitle', language)}
              </h2>
              <div
                className="text-sm leading-relaxed space-y-3"
                style={{ color: 'var(--text-secondary)' }}
              >
                <p>{t('termsTestingPhase1', language)}</p>
                <p>{t('termsTestingPhase2', language)}</p>
                <p className="font-semibold">{t('termsTestingPhase3', language)}</p>
                <p>{t('termsTestingPhase4', language)}</p>
                <p>{t('termsTestingPhase5', language)}</p>
              </div>
            </section>

            {/* Risk Warnings */}
            <section>
              <h2
                className="text-xl font-semibold mb-4"
                style={{ color: 'var(--text-primary)' }}
              >
                {t('termsRiskWarningsTitle', language)}
              </h2>
              <div
                className="text-sm leading-relaxed space-y-3"
                style={{ color: 'var(--text-secondary)' }}
              >
                <p>{t('termsRiskWarnings1', language)}</p>
                <p>{t('termsRiskWarnings2', language)}</p>
                <p>{t('termsRiskWarnings3', language)}</p>
                <ul className="list-disc list-inside ml-4 space-y-2">
                  <li>{t('termsRiskWarnings4', language)}</li>
                  <li>{t('termsRiskWarnings5', language)}</li>
                  <li>{t('termsRiskWarnings6', language)}</li>
                  <li>{t('termsRiskWarnings7', language)}</li>
                  <li>{t('termsRiskWarnings8', language)}</li>
                </ul>
              </div>
            </section>

            {/* Platform Disclaimers */}
            <section>
              <h2
                className="text-xl font-semibold mb-4"
                style={{ color: 'var(--text-primary)' }}
              >
                {t('termsPlatformDisclaimersTitle', language)}
              </h2>
              <div
                className="text-sm leading-relaxed space-y-3"
                style={{ color: 'var(--text-secondary)' }}
              >
                <p>{t('termsPlatformDisclaimers1', language)}</p>
                <p>{t('termsPlatformDisclaimers2', language)}</p>
                <p>{t('termsPlatformDisclaimers3', language)}</p>
                <p>{t('termsPlatformDisclaimers4', language)}</p>
              </div>
            </section>

            {/* User Responsibilities */}
            <section>
              <h2
                className="text-xl font-semibold mb-4"
                style={{ color: 'var(--text-primary)' }}
              >
                {t('termsUserResponsibilitiesTitle', language)}
              </h2>
              <div
                className="text-sm leading-relaxed space-y-3"
                style={{ color: 'var(--text-secondary)' }}
              >
                <p>{t('termsUserResponsibilities1', language)}</p>
                <ul className="list-disc list-inside ml-4 space-y-2">
                  <li>{t('termsUserResponsibilities2', language)}</li>
                  <li>{t('termsUserResponsibilities3', language)}</li>
                  <li>{t('termsUserResponsibilities4', language)}</li>
                  <li>{t('termsUserResponsibilities5', language)}</li>
                  <li>{t('termsUserResponsibilities6', language)}</li>
                </ul>
              </div>
            </section>

            {/* Limitation of Liability */}
            <section>
              <h2
                className="text-xl font-semibold mb-4"
                style={{ color: 'var(--text-primary)' }}
              >
                {t('termsLiabilityTitle', language)}
              </h2>
              <div
                className="text-sm leading-relaxed space-y-3"
                style={{ color: 'var(--text-secondary)' }}
              >
                <p>{t('termsLiability1', language)}</p>
                <p>{t('termsLiability2', language)}</p>
                <p>{t('termsLiability3', language)}</p>
              </div>
            </section>

            {/* License Information */}
            <section>
              <h2
                className="text-xl font-semibold mb-4"
                style={{ color: 'var(--text-primary)' }}
              >
                {t('termsLicenseTitle', language)}
              </h2>
              <div
                className="text-sm leading-relaxed space-y-3"
                style={{ color: 'var(--text-secondary)' }}
              >
                <p>{t('termsLicense1', language)}</p>
                <p>{t('termsLicense2', language)}</p>
                <p>
                  {t('termsLicense3', language)}{' '}
                  <a
                    href="https://www.gnu.org/licenses/agpl-3.0.html"
                    target="_blank"
                    rel="noopener noreferrer"
                    className="underline hover:opacity-80 transition-opacity"
                    style={{ color: 'var(--green-primary)' }}
                  >
                    {t('termsLicenseLink', language)}
                  </a>
                </p>
              </div>
            </section>

            {/* Acceptance */}
            <section>
              <h2
                className="text-xl font-semibold mb-4"
                style={{ color: 'var(--text-primary)' }}
              >
                {t('termsAcceptanceTitle', language)}
              </h2>
              <div
                className="text-sm leading-relaxed space-y-3"
                style={{ color: 'var(--text-secondary)' }}
              >
                <p>{t('termsAcceptance1', language)}</p>
                <p>{t('termsAcceptance2', language)}</p>
              </div>
            </section>

            {/* Contact */}
            <section>
              <h2
                className="text-xl font-semibold mb-4"
                style={{ color: 'var(--text-primary)' }}
              >
                {t('termsContactTitle', language)}
              </h2>
              <div
                className="text-sm leading-relaxed"
                style={{ color: 'var(--text-secondary)' }}
              >
                <p>{t('termsContact1', language)}</p>
              </div>
            </section>
          </div>
        </div>
      </Container>
    </>
  )
}

