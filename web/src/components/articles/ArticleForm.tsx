import { useState, useEffect } from 'react'
import { ArticleEditor } from './ArticleEditor'
import type { Article, CreateArticleRequest, UpdateArticleRequest } from '../../types'

interface ArticleFormProps {
  article?: Article
  onSubmit: (data: CreateArticleRequest | UpdateArticleRequest) => Promise<void>
  onCancel: () => void
  isLoading?: boolean
}

export function ArticleForm({ article, onSubmit, onCancel, isLoading = false }: ArticleFormProps) {
  const [title, setTitle] = useState(article?.title || '')
  const [slug, setSlug] = useState(article?.slug || '')
  const [content, setContent] = useState(article?.content || '')
  const [excerpt, setExcerpt] = useState(article?.excerpt || '')
  const [featuredImageUrl, setFeaturedImageUrl] = useState(article?.featured_image_url || '')
  const [status, setStatus] = useState<'draft' | 'published'>(article?.status || 'draft')
  const [metaTitle, setMetaTitle] = useState(article?.meta_title || '')
  const [metaDescription, setMetaDescription] = useState(article?.meta_description || '')
  const [metaKeywords, setMetaKeywords] = useState(article?.meta_keywords || '')
  const [ogImageUrl, setOgImageUrl] = useState(article?.og_image_url || '')

  // Sync form state when article prop changes
  useEffect(() => {
    if (article) {
      setTitle(article.title || '')
      setSlug(article.slug || '')
      setContent(article.content || '')
      setExcerpt(article.excerpt || '')
      setFeaturedImageUrl(article.featured_image_url || '')
      setStatus(article.status || 'draft')
      setMetaTitle(article.meta_title || '')
      setMetaDescription(article.meta_description || '')
      setMetaKeywords(article.meta_keywords || '')
      setOgImageUrl(article.og_image_url || '')
    }
  }, [article])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    
    const formData: CreateArticleRequest | UpdateArticleRequest = {
      title,
      content,
      excerpt,
      featured_image_url: featuredImageUrl,
      status,
      meta_title: metaTitle || undefined,
      meta_description: metaDescription,
      meta_keywords: metaKeywords || undefined,
      og_image_url: ogImageUrl || undefined,
    }

    if (slug) {
      (formData as UpdateArticleRequest).slug = slug
    }

    await onSubmit(formData)
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-6">
      {/* Author Info (read-only) */}
      {article?.author_email && (
        <div className="p-3 rounded-md" style={{ backgroundColor: 'var(--bg-secondary, #f3f4f6)' }}>
          <label className="block text-xs font-medium mb-1" style={{ color: '#000000' }}>
            Author
          </label>
          <p className="text-sm font-medium" style={{ color: '#000000' }}>
            {article.author_email}
          </p>
        </div>
      )}

      {/* Title */}
      <div>
        <label htmlFor="title" className="block text-sm font-medium mb-2" style={{ color: '#000000' }}>
          Title *
        </label>
        <input
          type="text"
          id="title"
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          required
          className="w-full px-3 py-2 border rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent transition-colors"
          style={{ 
            borderColor: 'var(--border-color, #d1d5db)', 
            color: '#000000', 
            backgroundColor: 'var(--bg-primary, #ffffff)' 
          }}
        />
      </div>

      {/* Slug (editable) */}
      <div>
        <label htmlFor="slug" className="block text-sm font-medium mb-2" style={{ color: '#000000' }}>
          Slug (URL-friendly identifier)
        </label>
        <input
          type="text"
          id="slug"
          value={slug}
          onChange={(e) => setSlug(e.target.value)}
          placeholder="Auto-generated from title if left empty"
          className="w-full px-3 py-2 border rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent transition-colors placeholder-gray-500"
          style={{ 
            borderColor: 'var(--border-color, #d1d5db)', 
            color: '#000000', 
            backgroundColor: 'var(--bg-primary, #ffffff)' 
          }}
        />
      </div>

      {/* Content */}
      <div>
        <label className="block text-sm font-medium mb-2" style={{ color: '#000000' }}>
          Content *
        </label>
        <div className="border rounded-md overflow-hidden" style={{ borderColor: 'var(--border-color, #d1d5db)' }}>
          <ArticleEditor content={content} onChange={setContent} />
        </div>
      </div>

      {/* Excerpt */}
      <div>
        <label htmlFor="excerpt" className="block text-sm font-medium mb-2" style={{ color: '#000000' }}>
          Excerpt (Short description for previews)
        </label>
        <textarea
          id="excerpt"
          value={excerpt}
          onChange={(e) => setExcerpt(e.target.value)}
          rows={3}
          className="w-full px-3 py-2 border rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent transition-colors resize-y"
          style={{ 
            borderColor: 'var(--border-color, #d1d5db)', 
            color: '#000000', 
            backgroundColor: 'var(--bg-primary, #ffffff)' 
          }}
        />
      </div>

      {/* Featured Image URL */}
      <div>
        <label htmlFor="featuredImageUrl" className="block text-sm font-medium mb-2" style={{ color: '#000000' }}>
          Featured Image URL
        </label>
        <input
          type="url"
          id="featuredImageUrl"
          value={featuredImageUrl}
          onChange={(e) => setFeaturedImageUrl(e.target.value)}
          placeholder="https://example.com/image.jpg"
          className="w-full px-3 py-2 border rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent transition-colors placeholder-gray-500"
          style={{ 
            borderColor: 'var(--border-color, #d1d5db)', 
            color: '#000000', 
            backgroundColor: 'var(--bg-primary, #ffffff)' 
          }}
        />
        {featuredImageUrl && (
          <img 
            src={featuredImageUrl} 
            alt="Featured" 
            className="mt-3 max-w-xs rounded-lg border shadow-sm" 
            style={{ borderColor: 'var(--border-color, #d1d5db)' }}
            onError={(e) => { (e.target as HTMLImageElement).style.display = 'none' }} 
          />
        )}
      </div>

      {/* Status */}
      <div>
        <label htmlFor="status" className="block text-sm font-medium mb-2" style={{ color: '#000000' }}>
          Status
        </label>
        <select
          id="status"
          value={status}
          onChange={(e) => setStatus(e.target.value as 'draft' | 'published')}
          className="w-full px-3 py-2 border rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent transition-colors"
          style={{ 
            borderColor: 'var(--border-color, #d1d5db)', 
            color: '#000000', 
            backgroundColor: 'var(--bg-primary, #ffffff)' 
          }}
        >
          <option value="draft">Draft</option>
          <option value="published">Published</option>
        </select>
      </div>

      {/* SEO Section */}
      <div className="border-t pt-6" style={{ borderColor: 'var(--border-color, #e5e7eb)' }}>
        <h3 className="text-lg font-semibold mb-4" style={{ color: '#000000' }}>SEO Settings</h3>

        {/* Meta Title */}
        <div className="mb-4">
          <label htmlFor="metaTitle" className="block text-sm font-medium mb-2" style={{ color: '#000000' }}>
            Meta Title (optional, defaults to title)
          </label>
          <input
            type="text"
            id="metaTitle"
            value={metaTitle}
            onChange={(e) => setMetaTitle(e.target.value)}
            className="w-full px-3 py-2 border rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent transition-colors"
            style={{ 
              borderColor: 'var(--border-color, #d1d5db)', 
              color: '#000000', 
              backgroundColor: 'var(--bg-primary, #ffffff)' 
            }}
          />
        </div>

        {/* Meta Description */}
        <div className="mb-4">
          <label htmlFor="metaDescription" className="block text-sm font-medium mb-2" style={{ color: '#000000' }}>
            Meta Description *
          </label>
          <textarea
            id="metaDescription"
            value={metaDescription}
            onChange={(e) => setMetaDescription(e.target.value)}
            rows={3}
            required
            className="w-full px-3 py-2 border rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent transition-colors resize-y"
            style={{ 
              borderColor: 'var(--border-color, #d1d5db)', 
              color: '#000000', 
              backgroundColor: 'var(--bg-primary, #ffffff)' 
            }}
          />
        </div>

        {/* Meta Keywords */}
        <div className="mb-4">
          <label htmlFor="metaKeywords" className="block text-sm font-medium mb-2" style={{ color: '#000000' }}>
            Meta Keywords (comma-separated)
          </label>
          <input
            type="text"
            id="metaKeywords"
            value={metaKeywords}
            onChange={(e) => setMetaKeywords(e.target.value)}
            placeholder="AI trading, cryptocurrency, trading strategies"
            className="w-full px-3 py-2 border rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent transition-colors placeholder-gray-500"
            style={{ 
              borderColor: 'var(--border-color, #d1d5db)', 
              color: '#000000', 
              backgroundColor: 'var(--bg-primary, #ffffff)' 
            }}
          />
        </div>

        {/* OG Image URL */}
        <div>
          <label htmlFor="ogImageUrl" className="block text-sm font-medium mb-2" style={{ color: '#000000' }}>
            Open Graph Image URL (optional, defaults to featured image)
          </label>
          <input
            type="url"
            id="ogImageUrl"
            value={ogImageUrl}
            onChange={(e) => setOgImageUrl(e.target.value)}
            placeholder="https://example.com/og-image.jpg"
            className="w-full px-3 py-2 border rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent transition-colors placeholder-gray-500"
            style={{ 
              borderColor: 'var(--border-color, #d1d5db)', 
              color: '#000000', 
              backgroundColor: 'var(--bg-primary, #ffffff)' 
            }}
          />
        </div>
      </div>

      {/* Actions */}
      <div className="flex gap-4 justify-end pt-4 border-t" style={{ borderColor: 'var(--border-color, #e5e7eb)' }}>
        <button
          type="button"
          onClick={onCancel}
          className="px-6 py-2 border rounded-md hover:bg-gray-50 transition-colors font-medium"
          style={{ 
            borderColor: 'var(--border-color, #d1d5db)', 
            color: '#000000' 
          }}
          disabled={isLoading}
        >
          Cancel
        </button>
        <button
          type="submit"
          className="px-6 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700 disabled:opacity-50 disabled:cursor-not-allowed transition-colors font-medium shadow-sm"
          disabled={isLoading}
        >
          {isLoading ? 'Saving...' : article ? 'Update Article' : 'Create Article'}
        </button>
      </div>
    </form>
  )
}
