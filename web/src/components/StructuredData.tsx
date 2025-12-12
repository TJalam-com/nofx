import { Helmet } from 'react-helmet-async'

interface StructuredDataProps {
  data: Record<string, any>
}

export function StructuredData({ data }: StructuredDataProps) {
  return (
    <Helmet>
      <script type="application/ld+json">{JSON.stringify(data)}</script>
    </Helmet>
  )
}

// Organization Schema for homepage
export function OrganizationSchema() {
  const baseUrl = import.meta.env.VITE_BASE_URL || 'https://nofx.ai'
  
  const schema = {
    '@context': 'https://schema.org',
    '@type': 'Organization',
    name: 'AI Trading 24x7',
    url: baseUrl,
    logo: `${baseUrl}/icons/nofx.svg`,
    description: 'Multi-AI Model Trading Platform for automated cryptocurrency trading',
    sameAs: [
      'https://github.com/NoFxAiOS/nofx',
      // Add other social media links as they become available
    ],
  }

  return <StructuredData data={schema} />
}

// WebSite Schema with SearchAction
export function WebSiteSchema() {
  const baseUrl = import.meta.env.VITE_BASE_URL || 'https://nofx.ai'
  
  const schema = {
    '@context': 'https://schema.org',
    '@type': 'WebSite',
    name: 'AI Trading 24x7',
    url: baseUrl,
    potentialAction: {
      '@type': 'SearchAction',
      target: {
        '@type': 'EntryPoint',
        urlTemplate: `${baseUrl}/faq?q={search_term_string}`,
      },
      'query-input': 'required name=search_term_string',
    },
  }

  return <StructuredData data={schema} />
}

// FAQPage Schema
export function FAQPageSchema({ faqs }: { faqs: Array<{ question: string; answer: string }> }) {
  const schema = {
    '@context': 'https://schema.org',
    '@type': 'FAQPage',
    mainEntity: faqs.map((faq) => ({
      '@type': 'Question',
      name: faq.question,
      acceptedAnswer: {
        '@type': 'Answer',
        text: faq.answer,
      },
    })),
  }

  return <StructuredData data={schema} />
}

// BreadcrumbList Schema
export function BreadcrumbListSchema({ items }: { items: Array<{ name: string; url: string }> }) {
  const baseUrl = import.meta.env.VITE_BASE_URL || 'https://nofx.ai'
  
  const schema = {
    '@context': 'https://schema.org',
    '@type': 'BreadcrumbList',
    itemListElement: items.map((item, index) => ({
      '@type': 'ListItem',
      position: index + 1,
      name: item.name,
      item: item.url.startsWith('http') ? item.url : `${baseUrl}${item.url}`,
    })),
  }

  return <StructuredData data={schema} />
}

// Offer Schema for pricing information
export interface OfferData {
  name: string
  price: string
  priceCurrency: string
  availability?: string
  url?: string
}

export function OfferSchema({ offer }: { offer: OfferData }) {
  const baseUrl = import.meta.env.VITE_BASE_URL || 'https://nofx.ai'
  
  const schema = {
    '@context': 'https://schema.org',
    '@type': 'Offer',
    name: offer.name,
    price: offer.price,
    priceCurrency: offer.priceCurrency,
    availability: offer.availability || 'https://schema.org/InStock',
    url: offer.url || baseUrl,
  }

  return <StructuredData data={schema} />
}

// AggregateRating Schema for reviews/ratings
export interface AggregateRatingData {
  ratingValue: number
  ratingCount: number
  bestRating?: number
  worstRating?: number
}

export function AggregateRatingSchema({ rating }: { rating: AggregateRatingData }) {
  const schema = {
    '@context': 'https://schema.org',
    '@type': 'AggregateRating',
    ratingValue: rating.ratingValue,
    ratingCount: rating.ratingCount,
    bestRating: rating.bestRating || 5,
    worstRating: rating.worstRating || 1,
  }

  return <StructuredData data={schema} />
}

// SoftwareApplication Schema for trading platform
export interface SoftwareApplicationData {
  name?: string
  description?: string
  applicationCategory?: string
  operatingSystem?: string
  offers?: OfferData[]
  aggregateRating?: AggregateRatingData
  url?: string
}

export function SoftwareApplicationSchema({
  name = 'AI Trading 24x7',
  description = 'AI-powered copy trading platform supporting multiple exchanges and AI models',
  applicationCategory = 'FinanceApplication',
  operatingSystem = 'Web',
  offers = [],
  aggregateRating,
  url,
}: SoftwareApplicationData = {}) {
  const baseUrl = import.meta.env.VITE_BASE_URL || 'https://nofx.ai'
  
  const schema: Record<string, any> = {
    '@context': 'https://schema.org',
    '@type': 'SoftwareApplication',
    name,
    description,
    applicationCategory,
    operatingSystem,
    applicationSubCategory: 'Trading Platform',
    url: url || baseUrl,
    offers: offers.map((offer) => ({
      '@type': 'Offer',
      name: offer.name,
      price: offer.price,
      priceCurrency: offer.priceCurrency,
      availability: offer.availability || 'https://schema.org/InStock',
      url: offer.url || baseUrl,
    })),
  }

  // Add aggregateRating if provided
  if (aggregateRating) {
    schema.aggregateRating = {
      '@type': 'AggregateRating',
      ratingValue: aggregateRating.ratingValue,
      ratingCount: aggregateRating.ratingCount,
      bestRating: aggregateRating.bestRating || 5,
      worstRating: aggregateRating.worstRating || 1,
    }
  }

  return <StructuredData data={schema} />
}

