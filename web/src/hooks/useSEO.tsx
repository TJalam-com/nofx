import { useLocation } from 'react-router-dom'
import { useLanguage } from '../contexts/LanguageContext'
import { getSEOConfig } from '../config/seo'
import { SEO } from '../components/SEO'

export function useSEO() {
  const location = useLocation()
  const { language } = useLanguage()
  const seoConfig = getSEOConfig(location.pathname, language)

  return {
    SEOComponent: () => <SEO {...seoConfig} lang={language} />,
    seoConfig,
  }
}

