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
  const baseUrl = import.meta.env.VITE_BASE_URL || 'https://aitrading247.com'
  const fullCanonical = canonical || baseUrl
  
  // Prefer WebP for OG images, fallback to PNG
  const getOgImage = (imagePath: string): string => {
    if (imagePath.startsWith('http')) return imagePath
    // Try WebP first, fallback to PNG
    const webpPath = imagePath.replace(/\.png$/, '.webp')
    return `${baseUrl}${webpPath}`
  }
  const fullOgImage = ogImage 
    ? (ogImage.startsWith('http') ? ogImage : getOgImage(ogImage))
    : `${baseUrl}/images/main.webp`
  const ogLocale = lang === 'zh' ? 'zh_CN' : 'en_US'

  return (
    <Helmet>
      {/* Primary Meta Tags */}
      <title>{title}</title>
      <meta name="title" content={title} />
      <meta name="description" content={description} />
      {keywords && <meta name="keywords" content={keywords} />}
      {noindex && <meta name="robots" content="noindex, nofollow" />}
      {!noindex && <meta name="robots" content="index, follow" />}
      <meta name="author" content="AI Trading 24x7" />
      <meta name="theme-color" content="#0a0e27" />

      {/* Canonical URL */}
      <link rel="canonical" href={fullCanonical} />
      
      {/* Alternate language versions */}
      <link rel="alternate" hrefLang="en" href={fullCanonical} />
      <link rel="alternate" hrefLang="zh-CN" href={`${fullCanonical}?lang=zh`} />
      <link rel="alternate" hrefLang="x-default" href={fullCanonical} />

      {/* Open Graph / Facebook */}
      <meta property="og:type" content={ogType} />
      <meta property="og:url" content={fullCanonical} />
      <meta property="og:title" content={title} />
      <meta property="og:description" content={description} />
      <meta property="og:image" content={fullOgImage} />
      <meta property="og:image:width" content="1200" />
      <meta property="og:image:height" content="630" />
      <meta property="og:image:alt" content={title} />
      <meta property="og:site_name" content="AI Trading 24x7" />
      <meta property="og:locale" content={ogLocale} />
      <meta property="og:locale:alternate" content={lang === 'zh' ? 'en_US' : 'zh_CN'} />

      {/* Twitter */}
      <meta name="twitter:card" content="summary_large_image" />
      <meta name="twitter:url" content={fullCanonical} />
      <meta name="twitter:title" content={title} />
      <meta name="twitter:description" content={description} />
      <meta name="twitter:image" content={fullOgImage} />
      <meta name="twitter:image:alt" content={title} />
      <meta name="twitter:site" content="@AITrading24x7" />

      {/* Language */}
      <html lang={lang === 'zh' ? 'zh-CN' : 'en'} />
    </Helmet>
  )
}

