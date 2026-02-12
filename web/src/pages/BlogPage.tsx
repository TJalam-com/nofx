import { useState, useMemo } from 'react'
import { Link } from 'react-router-dom'
import useSWR from 'swr'
import { api } from '../lib/api'
import {
  Search,
  ChevronLeft,
  ChevronRight,
  Calendar,
  FileText,
} from 'lucide-react'
import type { Article } from '../types'
import { useSEO } from '../hooks/useSEO'
import { Helmet } from 'react-helmet-async'
import { convertImgBBUrl } from '../utils/imgbb'

export function BlogPage() {
  const { SEOComponent } = useSEO()
  const [searchQuery, setSearchQuery] = useState('')
  const [currentPage, setCurrentPage] = useState(1)
  const articlesPerPage = 12

  const { data: articles, isLoading: articlesLoading } = useSWR<Article[]>(
    ['published-articles', currentPage],
    () =>
      api.getPublishedArticles(
        articlesPerPage,
        (currentPage - 1) * articlesPerPage
      ),
    { revalidateOnFocus: false }
  )

  // Filter articles by search query
  const filteredArticles = useMemo(() => {
    if (!articles) return []

    if (!searchQuery.trim()) return articles

    const query = searchQuery.toLowerCase()
    return articles.filter(
      (article) =>
        article.title.toLowerCase().includes(query) ||
        article.excerpt.toLowerCase().includes(query) ||
        article.meta_description.toLowerCase().includes(query)
    )
  }, [articles, searchQuery])

  const baseUrl = import.meta.env.VITE_BASE_URL || 'https://aitrading247.com'

  // Helper function to convert ImgBB URLs to direct image URLs
  // Uses utility function that handles page URLs, HTML embed codes, and BBCode
  const convertImgBBUrlLocal = (url: string): string => {
    return convertImgBBUrl(url)
  }

  // Structured data for BlogCollection
  const structuredData = {
    '@context': 'https://schema.org',
    '@type': 'CollectionPage',
    name: 'AI Trading 24x7 Blog',
    description:
      'Latest articles about AI trading, cryptocurrency, and trading strategies',
    url: `${baseUrl}/blog`,
    mainEntity: {
      '@type': 'ItemList',
      numberOfItems: articles?.length || 0,
      itemListElement:
        articles?.map((article, index) => ({
          '@type': 'ListItem',
          position: index + 1,
          item: {
            '@type': 'Article',
            headline: article.title,
            description: article.excerpt || article.meta_description,
            image: article.featured_image_url
              ? convertImgBBUrlLocal(article.featured_image_url)
              : `${baseUrl}/images/main.webp`,
            url: `${baseUrl}/blog/${article.slug}`,
            datePublished: article.published_at,
            dateModified: article.updated_at,
          },
        })) || [],
    },
  }

  return (
    <>
      <SEOComponent />
      <Helmet>
        {/* Structured Data */}
        <script type="application/ld+json">
          {JSON.stringify(structuredData)}
        </script>
      </Helmet>
      <div className="container mx-auto px-4 py-8 max-w-7xl">
        <div className="mb-8">
          <h1
            className="text-4xl font-bold mb-4"
            style={{ color: 'var(--text-primary, #111827)' }}
          >
            Blog
          </h1>
          <p
            className="text-lg"
            style={{ color: 'var(--text-secondary, #6b7280)' }}
          >
            Latest articles about AI trading, cryptocurrency, and trading
            strategies
          </p>
        </div>

        {/* Search */}
        <div className="mb-8">
          <div className="relative max-w-md">
            <Search
              className="absolute left-3 top-1/2 transform -translate-y-1/2 text-gray-400"
              size={20}
            />
            <input
              type="text"
              placeholder="Search articles..."
              value={searchQuery}
              onChange={(e) => {
                setSearchQuery(e.target.value)
                setCurrentPage(1)
              }}
              className="w-full pl-10 pr-4 py-2 border rounded placeholder-gray-500"
              style={{
                borderColor: 'var(--border-color, #e5e7eb)',
                color: '#000000',
                backgroundColor: 'var(--bg-primary, #ffffff)',
              }}
            />
          </div>
        </div>

        {/* Articles Grid */}
        {articlesLoading ? (
          <div
            className="text-center py-12"
            style={{ color: 'var(--text-secondary, #6b7280)' }}
          >
            Loading articles...
          </div>
        ) : filteredArticles.length === 0 ? (
          <div
            className="text-center py-12"
            style={{ color: 'var(--text-secondary, #6b7280)' }}
          >
            <FileText size={48} className="mx-auto mb-4 opacity-50" />
            <p>No articles found</p>
          </div>
        ) : (
          <>
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6 mb-8">
              {filteredArticles.map((article) => {
                const processedImageUrl = article.featured_image_url
                  ? convertImgBBUrlLocal(article.featured_image_url)
                  : null
                return (
                  <Link
                    key={article.id}
                    to={`/blog/${article.slug}`}
                    className="border rounded-lg overflow-hidden hover:shadow-lg transition-shadow group"
                    style={{
                      borderColor: 'var(--border-color, #e5e7eb)',
                      backgroundColor: 'var(--bg-primary, #ffffff)',
                    }}
                  >
                    {processedImageUrl && (
                      <div className="aspect-[4/3] overflow-hidden bg-gray-100">
                        <img
                          src={processedImageUrl}
                          alt={article.title}
                          className="w-full h-full object-cover group-hover:scale-105 transition-transform"
                          onError={(e) => {
                            ;(e.target as HTMLImageElement).style.display =
                              'none'
                          }}
                        />
                      </div>
                    )}
                    <div className="p-6">
                      <h2
                        className="text-xl font-semibold mb-2 line-clamp-2 group-hover:text-blue-600 transition-colors"
                        style={{ color: '#000000' }}
                      >
                        {article.title}
                      </h2>
                      <p
                        className="text-sm mb-4 line-clamp-3"
                        style={{ color: '#000000' }}
                      >
                        {article.excerpt || article.meta_description}
                      </p>
                      <div
                        className="flex items-center gap-2 text-xs"
                        style={{ color: '#000000' }}
                      >
                        <Calendar size={14} />
                        {article.published_at
                          ? new Date(article.published_at).toLocaleDateString()
                          : new Date(article.created_at).toLocaleDateString()}
                      </div>
                    </div>
                  </Link>
                )
              })}
            </div>

            {/* Pagination */}
            {articles && articles.length === articlesPerPage && (
              <div className="flex justify-center items-center gap-2">
                <button
                  onClick={() => setCurrentPage((p) => Math.max(1, p - 1))}
                  disabled={currentPage === 1}
                  className="p-2 border rounded disabled:opacity-50"
                  style={{
                    borderColor: 'var(--border-color, #e5e7eb)',
                    color: 'var(--text-primary, #111827)',
                  }}
                >
                  <ChevronLeft size={20} />
                </button>
                <span
                  className="px-4"
                  style={{ color: 'var(--text-primary, #111827)' }}
                >
                  Page {currentPage}
                </span>
                <button
                  onClick={() => setCurrentPage((p) => p + 1)}
                  disabled={articles.length < articlesPerPage}
                  className="p-2 border rounded disabled:opacity-50"
                  style={{
                    borderColor: 'var(--border-color, #e5e7eb)',
                    color: 'var(--text-primary, #111827)',
                  }}
                >
                  <ChevronRight size={20} />
                </button>
              </div>
            )}
          </>
        )}
      </div>
    </>
  )
}
