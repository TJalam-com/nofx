import { Helmet } from 'react-helmet-async'
import { SEOConfig } from '../config/seo'

interface SEOProps extends SEOConfig {
  lang?: string
}

export function SEO({
  title,
  description,
  keywords,
  ogImage,
  ogType = 'website',
  noindex = false,
  canonical,
  lang = 'en',
}: SEOProps) {
  const baseUrl = import.meta.env.VITE_BASE_URL || 'https://nofx.ai'
  const fullCanonical = canonical || baseUrl
  const fullOgImage = ogImage ? (ogImage.startsWith('http') ? ogImage : `${baseUrl}${ogImage}`) : `${baseUrl}/images/main.png`

  return (
    <Helmet>
      {/* Primary Meta Tags */}
      <title>{title}</title>
      <meta name="title" content={title} />
      <meta name="description" content={description} />
      {keywords && <meta name="keywords" content={keywords} />}
      {noindex && <meta name="robots" content="noindex, nofollow" />}

      {/* Canonical URL */}
      <link rel="canonical" href={fullCanonical} />

      {/* Open Graph / Facebook */}
      <meta property="og:type" content={ogType} />
      <meta property="og:url" content={fullCanonical} />
      <meta property="og:title" content={title} />
      <meta property="og:description" content={description} />
      <meta property="og:image" content={fullOgImage} />
      <meta property="og:site_name" content="AI Trading 24x7" />

      {/* Twitter */}
      <meta name="twitter:card" content="summary_large_image" />
      <meta name="twitter:url" content={fullCanonical} />
      <meta name="twitter:title" content={title} />
      <meta name="twitter:description" content={description} />
      <meta name="twitter:image" content={fullOgImage} />

      {/* Language */}
      <html lang={lang === 'zh' ? 'zh-CN' : 'en'} />
    </Helmet>
  )
}

