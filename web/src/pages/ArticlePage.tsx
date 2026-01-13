import { useParams, Link, useNavigate } from 'react-router-dom'
import useSWR from 'swr'
import { api } from '../lib/api'
import { getArticleSEOConfig } from '../config/seo'
import { useLanguage } from '../contexts/LanguageContext'
import { Helmet } from 'react-helmet-async'
import { Calendar, ArrowLeft, Share2, Twitter, Facebook, Linkedin } from 'lucide-react'
import { useState, useEffect, useRef } from 'react'

export function ArticlePage() {
  const { slug } = useParams<{ slug: string }>()
  const navigate = useNavigate()
  const { language } = useLanguage()
  const [showShareMenu, setShowShareMenu] = useState(false)
  const articleContentRef = useRef<HTMLElement>(null)

  const {
    data: article,
    isLoading,
    error,
  } = useSWR(
    slug ? ['article', slug] : null,
    () => api.getArticleBySlug(slug!),
    { revalidateOnFocus: false }
  )

  // Process images after DOM is rendered to add error handling and styling
  useEffect(() => {
    if (!articleContentRef.current || !article) return
    const contentEl = articleContentRef.current
    const images = contentEl.querySelectorAll('img')
    
    images.forEach((img) => {
      // Ensure images have proper styling
      if (!img.classList.contains('prose-img')) {
        img.classList.add('prose-img')
      }
      
      // Add error handling
      img.onerror = () => {
        img.style.display = 'none'
      }
    })
  }, [article])

  const baseUrl = import.meta.env.VITE_BASE_URL || 'https://aitrading247.com'
  const articleUrl = `${baseUrl}/blog/${slug}`

  // SEO configuration
  const seoConfig = article ? getArticleSEOConfig({
    title: article.title,
    metaTitle: article.meta_title,
    metaDescription: article.meta_description || '',
    metaKeywords: article.meta_keywords,
    ogImageUrl: article.og_image_url,
    featuredImageUrl: article.featured_image_url || '',
    slug: article.slug,
    publishedAt: article.published_at,
    authorId: article.author_id,
  }, language) : null

  if (isLoading) {
    return (
      <div className="container mx-auto px-4 py-8 max-w-4xl">
        <div className="text-center py-12" style={{ color: 'var(--text-secondary, #6b7280)' }}>
          Loading article...
        </div>
      </div>
    )
  }

  if (error || !article) {
    return (
      <div className="container mx-auto px-4 py-8 max-w-4xl">
        <div className="text-center py-12">
          <h1 className="text-2xl font-bold mb-4" style={{ color: 'var(--text-primary, #111827)' }}>
            Article not found
          </h1>
          <Link
            to="/blog"
            className="text-blue-500 hover:text-blue-700"
          >
            Back to blog
          </Link>
        </div>
      </div>
    )
  }

  const handleShare = (platform: string) => {
    const shareUrls: Record<string, string> = {
      twitter: `https://twitter.com/intent/tweet?url=${encodeURIComponent(articleUrl)}&text=${encodeURIComponent(article.title)}`,
      facebook: `https://www.facebook.com/sharer/sharer.php?u=${encodeURIComponent(articleUrl)}`,
      linkedin: `https://www.linkedin.com/sharing/share-offsite/?url=${encodeURIComponent(articleUrl)}`,
    }

    if (shareUrls[platform]) {
      window.open(shareUrls[platform], '_blank', 'width=600,height=400')
    }
    setShowShareMenu(false)
  }

  // JSON-LD structured data
  const structuredData = {
    '@context': 'https://schema.org',
    '@type': 'Article',
    headline: article.meta_title || article.title,
    description: article.meta_description || article.excerpt,
    image: article.og_image_url || article.featured_image_url || `${baseUrl}/images/main.webp`,
    datePublished: article.published_at,
    dateModified: article.updated_at,
    author: article.author_email ? {
      '@type': 'Person',
      name: article.author_email,
      email: article.author_email,
    } : {
      '@type': 'Organization',
      name: 'AI Trading 24x7',
    },
    publisher: {
      '@type': 'Organization',
      name: 'AI Trading 24x7',
      logo: {
        '@type': 'ImageObject',
        url: `${baseUrl}/images/main.webp`,
      },
    },
    mainEntityOfPage: {
      '@type': 'WebPage',
      '@id': articleUrl,
    },
  }

  // Determine the date to display - prioritize created_at
  const displayDate = article.created_at || article.updated_at || (article.status === 'published' ? article.published_at : null)

  // Helper function to convert ImgBB page URLs to direct image URLs
  // Note: ImgBB page URLs (ibb.co/XXXXX) cannot be directly converted without fetching the page
  // This function attempts common patterns, but the best solution is to use direct image URLs
  const convertImgBBUrl = (url: string): string => {
    if (!url) return url
    // ImgBB page URLs: https://ibb.co/XXXXX 
    // Direct URLs: https://i.ibb.co/XXXXX/XXXXX.jpg (format varies)
    if (url.includes('ibb.co/') && !url.includes('i.ibb.co')) {
      // Try to use the embed format which sometimes works
      // ImgBB embed format: https://ibb.co/XXXXX -> can try https://i.ibb.co/XXXXX.jpg
      // But this doesn't always work as the actual path structure varies
      const match = url.match(/ibb\.co\/([a-zA-Z0-9]+)/)
      if (match && match[1]) {
        const imageId = match[1]
        // Try common pattern (may not work for all images)
        // The real solution requires fetching the ImgBB page or using their API
        return `https://i.ibb.co/${imageId}/${imageId}.jpg`
      }
    }
    return url
  }

  // Process article content to fix image URLs
  const processedContent = article.content ? (() => {
    let content = article.content
    // Find all img tags and fix their src attributes
    content = content.replace(/<img([^>]+)src=["']([^"']+)["']([^>]*)>/gi, (_match, before, src, after) => {
      const fixedSrc = convertImgBBUrl(src)
      return `<img${before}src="${fixedSrc}"${after}>`
    })
    return content
  })() : article.content

  // Fix featured image URL if needed
  const processedFeaturedImageUrl = article.featured_image_url ? convertImgBBUrl(article.featured_image_url) : article.featured_image_url

  return (
    <>
      <Helmet>
        <title>{seoConfig?.title || article.title}</title>
        <meta name="description" content={seoConfig?.description || article.meta_description} />
        {seoConfig?.keywords && <meta name="keywords" content={seoConfig.keywords} />}
        <link rel="canonical" href={seoConfig?.canonical || articleUrl} />

        {/* Open Graph */}
        <meta property="og:type" content="article" />
        <meta property="og:title" content={article.meta_title || article.title} />
        <meta property="og:description" content={article.meta_description || article.excerpt} />
        <meta property="og:image" content={article.og_image_url || article.featured_image_url || `${baseUrl}/images/main.webp`} />
        <meta property="og:url" content={articleUrl} />

        {/* Twitter Card */}
        <meta name="twitter:card" content="summary_large_image" />
        <meta name="twitter:title" content={article.meta_title || article.title} />
        <meta name="twitter:description" content={article.meta_description || article.excerpt} />
        <meta name="twitter:image" content={article.og_image_url || article.featured_image_url || `${baseUrl}/images/main.webp`} />

        {/* Structured Data */}
        <script type="application/ld+json">{JSON.stringify(structuredData)}</script>
      </Helmet>

      <div className="container mx-auto px-4 py-8 max-w-4xl">
        {/* Back Button */}
        <button
          onClick={() => navigate('/blog')}
          className="flex items-center gap-2 mb-6 text-blue-500 hover:text-blue-700"
        >
          <ArrowLeft size={20} />
          Back to Blog
        </button>

        {/* Article Header */}
        <header className="mb-8">
          <h1 className="text-4xl md:text-5xl font-bold mb-4" style={{ color: 'var(--text-primary, #111827)' }}>
            {article.title}
          </h1>
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-4 text-sm" style={{ color: 'var(--text-secondary, #6b7280)' }}>
              <div className="flex items-center gap-2">
                <Calendar size={16} />
                {displayDate ? new Date(displayDate).toLocaleDateString('en-US', {
                  year: 'numeric',
                  month: 'long',
                  day: 'numeric',
                }) : 'Not published'}
              </div>
            </div>
            <div className="relative">
              <button
                onClick={() => setShowShareMenu(!showShareMenu)}
                className="flex items-center gap-2 px-4 py-2 border rounded hover:bg-gray-100"
                style={{ borderColor: 'var(--border-color, #e5e7eb)', color: 'var(--text-primary, #111827)' }}
              >
                <Share2 size={18} />
                Share
              </button>
              {showShareMenu && (
                <div className="absolute right-0 mt-2 w-48 bg-white border rounded-lg shadow-lg z-10" style={{ borderColor: 'var(--border-color, #e5e7eb)', backgroundColor: 'var(--bg-primary, #ffffff)' }}>
                  <button
                    onClick={() => handleShare('twitter')}
                    className="w-full flex items-center gap-2 px-4 py-2 hover:bg-gray-100 text-left"
                    style={{ color: 'var(--text-primary, #111827)' }}
                  >
                    <Twitter size={18} />
                    Twitter
                  </button>
                  <button
                    onClick={() => handleShare('facebook')}
                    className="w-full flex items-center gap-2 px-4 py-2 hover:bg-gray-100 text-left"
                    style={{ color: 'var(--text-primary, #111827)' }}
                  >
                    <Facebook size={18} />
                    Facebook
                  </button>
                  <button
                    onClick={() => handleShare('linkedin')}
                    className="w-full flex items-center gap-2 px-4 py-2 hover:bg-gray-100 text-left"
                    style={{ color: 'var(--text-primary, #111827)' }}
                  >
                    <Linkedin size={18} />
                    LinkedIn
                  </button>
                </div>
              )}
            </div>
          </div>
        </header>

        {/* Featured Image */}
        {processedFeaturedImageUrl && (
          <div className="mb-8">
            <img
              src={processedFeaturedImageUrl}
              alt={article.title}
              className="w-full rounded-lg"
              onError={(e) => {
                (e.target as HTMLImageElement).style.display = 'none'
              }}
            />
          </div>
        )}

        {/* Article Content */}
        <article
          ref={articleContentRef}
          className="prose prose-lg dark:prose-invert max-w-none mb-8"
          style={{ 
            color: 'var(--text-primary, #EAECEF)',
          }}
          dangerouslySetInnerHTML={{ __html: processedContent }}
        />

        {/* Footer */}
        <div className="border-t pt-8 mt-8" style={{ borderColor: 'var(--border-color, #e5e7eb)' }}>
          <Link
            to="/blog"
            className="text-blue-500 hover:text-blue-700"
          >
            ← Back to Blog
          </Link>
        </div>
      </div>
    </>
  )
}
