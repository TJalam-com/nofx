import { useParams, Link, useNavigate } from 'react-router-dom'
import useSWR from 'swr'
import { api } from '../lib/api'
import { getArticleSEOConfig } from '../config/seo'
import { useLanguage } from '../contexts/LanguageContext'
import { Helmet } from 'react-helmet-async'
import { Calendar, ArrowLeft, Share2, Twitter, Facebook, Linkedin } from 'lucide-react'
import { useState, useEffect, useRef } from 'react'
import { convertImgBBUrl } from '../utils/imgbb'

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
    
    const processImage = (img: HTMLImageElement) => {
      // Skip if already processed
      if (img.dataset.processed === 'true') {
        return
      }
      img.dataset.processed = 'true'

      // Ensure images have proper styling class
      if (!img.classList.contains('prose-img')) {
        img.classList.add('prose-img')
      }

      // Remove any wrapper containers if they exist
      if (img.parentElement?.classList.contains('article-image-wrapper')) {
        const wrapper = img.parentElement
        const parent = wrapper.parentNode
        if (parent) {
          parent.insertBefore(img, wrapper)
          wrapper.remove()
        }
      }

      // Remove width/height attributes from DOM (not just styles)
      img.removeAttribute('width')
      img.removeAttribute('height')
      
      // Set image to display at natural size - NO RESIZING
      // Only constrain max-width and center
      img.style.display = 'block'
      img.style.marginLeft = 'auto'
      img.style.marginRight = 'auto'
      // Remove all explicit sizing - let browser use natural dimensions
      img.style.removeProperty('width')
      img.style.removeProperty('height')
      img.style.maxWidth = 'min(100%, 1200px)'
      img.style.maxHeight = 'none'
      img.style.width = 'auto'
      img.style.height = 'auto'
      img.style.minWidth = '0'
      img.style.minHeight = '0'
      img.style.objectFit = 'none' // No scaling at all
      img.style.imageRendering = 'auto' // Use browser's best rendering
      
      // Force natural dimensions after image loads
      const ensureNaturalSize = () => {
        if (img.naturalWidth > 0 && img.naturalHeight > 0) {
          // Ensure we're using natural dimensions
          img.style.width = 'auto'
          img.style.height = 'auto'
          img.style.maxWidth = 'min(100%, 1200px)'
          img.style.maxHeight = 'none'
        }
      }
      
      // Ensure natural size when image loads
      if (img.complete) {
        ensureNaturalSize()
      } else {
        img.addEventListener('load', ensureNaturalSize, { once: true })
      }
      
      // Force center alignment and larger size with 3:2 aspect ratio (900x600)
      img.style.display = 'block'
      img.style.marginLeft = 'auto'
      img.style.marginRight = 'auto'
      img.style.width = '80%'
      img.style.maxWidth = 'min(95%, 1200px)'
      img.style.height = 'auto'
      img.style.aspectRatio = '3 / 2'
      img.style.objectFit = 'contain'
      
      // Add error handling
      img.onerror = () => {
        img.style.display = 'none'
      }
    }
    
    images.forEach((img) => {
      if (img.complete && img.naturalWidth > 0) {
        // Image already loaded
        processImage(img)
      } else {
        // Wait for image to load
        img.onload = () => processImage(img)
        processImage(img) // Apply wrapper immediately too
      }
    })
  }, [article])

  // Ensure featured image maintains 3:2 aspect ratio (900x600)
  useEffect(() => {
    if (!article?.featured_image_url) return
    
    // Wait for DOM to update
    const timer = setTimeout(() => {
      const featuredImg = document.querySelector('.article-featured-image') as HTMLImageElement
      if (featuredImg) {
        const handleLoad = () => {
          // Ensure 3:2 aspect ratio is maintained
          featuredImg.style.height = 'auto'
          featuredImg.style.aspectRatio = '3 / 2'
          featuredImg.style.objectFit = 'contain'
        }
        
        if (featuredImg.complete) {
          handleLoad()
        } else {
          featuredImg.addEventListener('load', handleLoad)
          return () => featuredImg.removeEventListener('load', handleLoad)
        }
      }
    }, 100)
    
    return () => clearTimeout(timer)
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

  // Helper function to convert ImgBB URLs to direct image URLs
  // Uses utility function that handles page URLs, HTML embed codes, and BBCode
  const convertImgBBUrlLocal = (url: string): string => {
    return convertImgBBUrl(url)
  }

  // Process article content to fix image URLs
  const processedContent = article.content ? (() => {
    let content = article.content
    // Find all img tags and fix their src attributes
    content = content.replace(/<img([^>]+)src=["']([^"']+)["']([^>]*)>/gi, (_match, before, src, after) => {
      const fixedSrc = convertImgBBUrlLocal(src)
      return `<img${before}src="${fixedSrc}"${after}>`
    })
    return content
  })() : article.content

  // Fix featured image URL if needed
  const processedFeaturedImageUrl = article.featured_image_url ? convertImgBBUrlLocal(article.featured_image_url) : article.featured_image_url

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
          <div className="mb-8 w-full max-w-[1200px] mx-auto">
            <img
              src={processedFeaturedImageUrl}
              alt={article.title}
              className="rounded-lg article-featured-image"
              style={{
                width: '80%',
                maxWidth: 'min(95%, 1200px)',
                height: 'auto',
                display: 'block',
                margin: '0 auto',
                aspectRatio: '3 / 2',
                objectFit: 'contain',
              }}
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
