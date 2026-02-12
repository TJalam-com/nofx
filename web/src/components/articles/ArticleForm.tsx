import { useState, useEffect } from 'react'
import { ArticleEditor } from './ArticleEditor'
import type {
  Article,
  CreateArticleRequest,
  UpdateArticleRequest,
} from '../../types'
import { toast } from 'sonner'
import { extractDirectImageUrl, convertImgBBUrl } from '../../utils/imgbb'

interface ArticleFormProps {
  article?: Article
  onSubmit: (data: CreateArticleRequest | UpdateArticleRequest) => Promise<void>
  onCancel: () => void
  isLoading?: boolean
  onFormChange?: (formData: {
    title: string
    content: string
    excerpt: string
    slug: string
  }) => void
}

// Helper function to normalize slug (convert to URL-friendly format)
function normalizeSlug(input: string): string {
  // Convert to lowercase
  let slug = input.toLowerCase()

  // Replace spaces and underscores with hyphens
  slug = slug.replace(/\s+/g, '-')
  slug = slug.replace(/_/g, '-')

  // Remove special characters, keep only alphanumeric and hyphens
  slug = slug.replace(/[^a-z0-9-]/g, '')

  // Remove multiple consecutive hyphens
  slug = slug.replace(/-+/g, '-')

  // Remove leading and trailing hyphens
  slug = slug.trim().replace(/^-+|-+$/g, '')

  return slug
}

export function ArticleForm({
  article,
  onSubmit,
  onCancel,
  isLoading = false,
  onFormChange,
}: ArticleFormProps) {
  const [title, setTitle] = useState(article?.title || '')
  const [slug, setSlug] = useState(article?.slug || '')
  const [content, setContent] = useState(article?.content || '')
  const [excerpt, setExcerpt] = useState(article?.excerpt || '')
  const [featuredImageUrl, setFeaturedImageUrl] = useState(
    article?.featured_image_url || ''
  )
  const [status, setStatus] = useState<'draft' | 'published'>(
    article?.status || 'draft'
  )
  const [metaTitle, setMetaTitle] = useState(article?.meta_title || '')
  const [metaDescription, setMetaDescription] = useState(
    article?.meta_description || ''
  )
  const [metaKeywords, setMetaKeywords] = useState(article?.meta_keywords || '')
  const [ogImageUrl, setOgImageUrl] = useState(article?.og_image_url || '')
  const [isSlugManuallyEdited, setIsSlugManuallyEdited] = useState(false)

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
      setIsSlugManuallyEdited(true) // Slug from existing article is considered manually set
    } else {
      // Reset slug and manual edit flag when creating new article
      setSlug('')
      setIsSlugManuallyEdited(false)
    }
  }, [article])

  // Auto-generate slug from title when title changes (only if slug is empty or was auto-generated)
  useEffect(() => {
    // Only auto-generate for new articles (not when editing)
    if (!article && title && (!slug || !isSlugManuallyEdited)) {
      const generatedSlug = normalizeSlug(title)
      if (generatedSlug) {
        setSlug(generatedSlug)
        setIsSlugManuallyEdited(false)
      }
    }
  }, [title, article, slug, isSlugManuallyEdited])

  // Notify parent of form changes for auto-save tracking
  useEffect(() => {
    if (onFormChange) {
      onFormChange({ title, content, excerpt, slug })
    }
  }, [title, content, excerpt, slug, onFormChange])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()

    // Validate required fields if trying to publish
    if (status === 'published') {
      const missingFields: string[] = []
      if (!title || !title.trim()) {
        missingFields.push('Title')
      }
      if (!content || !content.trim()) {
        missingFields.push('Content')
      }
      if (!metaDescription || !metaDescription.trim()) {
        missingFields.push('Meta Description')
      }

      if (missingFields.length > 0) {
        toast.error(
          `Cannot publish: Missing required fields: ${missingFields.join(', ')}. Please fill in all required fields before publishing.`
        )
        return
      }
    }

    if (article) {
      // Update case - all fields are optional, but slug should not be changed
      const updateData: UpdateArticleRequest = {
        title,
        content,
        excerpt,
        featured_image_url: convertImgBBUrl(featuredImageUrl),
        status,
        meta_title: metaTitle || undefined,
        meta_description: metaDescription,
        meta_keywords: metaKeywords || undefined,
        og_image_url: ogImageUrl ? convertImgBBUrl(ogImageUrl) : undefined,
        // Do not include slug when updating - it should remain unchanged
      }
      await onSubmit(updateData)
    } else {
      // Create case - title is required
      const createData: CreateArticleRequest = {
        title,
        content,
        excerpt,
        featured_image_url: convertImgBBUrl(featuredImageUrl),
        status,
        meta_title: metaTitle || undefined,
        meta_description: metaDescription,
        meta_keywords: metaKeywords || undefined,
        og_image_url: ogImageUrl || undefined,
        slug: slug || undefined,
      }
      await onSubmit(createData)
    }
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-6">
      {/* Author Info (read-only) */}
      {article?.author_email && (
        <div
          className="p-3 rounded-md"
          style={{ backgroundColor: 'var(--bg-secondary, #f3f4f6)' }}
        >
          <label
            className="block text-xs font-medium mb-1"
            style={{ color: '#000000' }}
          >
            Author
          </label>
          <p className="text-sm font-medium" style={{ color: '#000000' }}>
            {article.author_email}
          </p>
        </div>
      )}

      {/* Title */}
      <div>
        <label
          htmlFor="title"
          className="block text-sm font-medium mb-2"
          style={{ color: '#000000' }}
        >
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
            backgroundColor: 'var(--bg-primary, #ffffff)',
          }}
        />
      </div>

      {/* Slug (editable only when creating, read-only when editing) */}
      <div>
        <label
          htmlFor="slug"
          className="block text-sm font-medium mb-2"
          style={{ color: '#000000' }}
        >
          Slug (URL-friendly identifier)
          {article && (
            <span className="ml-2 text-xs text-gray-500">
              (Cannot be changed after creation)
            </span>
          )}
        </label>
        <input
          type="text"
          id="slug"
          value={slug}
          onChange={(e) => {
            const normalized = normalizeSlug(e.target.value)
            setSlug(normalized)
            // If user clears the slug, allow auto-generation again
            if (normalized === '') {
              setIsSlugManuallyEdited(false)
            } else {
              setIsSlugManuallyEdited(true)
            }
          }}
          placeholder="Auto-generated from title if left empty (spaces will be converted to hyphens)"
          disabled={!!article}
          className="w-full px-3 py-2 border rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent transition-colors placeholder-gray-500 disabled:opacity-50 disabled:cursor-not-allowed disabled:bg-gray-100"
          style={{
            borderColor: 'var(--border-color, #d1d5db)',
            color: '#000000',
            backgroundColor: article
              ? 'var(--bg-secondary, #f3f4f6)'
              : 'var(--bg-primary, #ffffff)',
          }}
        />
      </div>

      {/* Content */}
      <div>
        <label
          className="block text-sm font-medium mb-2"
          style={{ color: '#000000' }}
        >
          Content *
        </label>
        <div
          className="border rounded-md"
          style={{ borderColor: 'var(--border-color, #d1d5db)' }}
        >
          <ArticleEditor content={content} onChange={setContent} />
        </div>
      </div>

      {/* Excerpt */}
      <div>
        <label
          htmlFor="excerpt"
          className="block text-sm font-medium mb-2"
          style={{ color: '#000000' }}
        >
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
            backgroundColor: 'var(--bg-primary, #ffffff)',
          }}
        />
      </div>

      {/* Featured Image URL */}
      <div>
        <label
          htmlFor="featuredImageUrl"
          className="block text-sm font-medium mb-2"
          style={{ color: '#000000' }}
        >
          Featured Image URL
        </label>
        <p
          className="text-xs mb-2"
          style={{ color: 'var(--text-secondary, #6b7280)' }}
        >
          Recommended dimensions: 1200 x 900 px (4:3 aspect ratio) for optimal
          display quality
        </p>
        <p
          className="text-xs mb-2"
          style={{ color: 'var(--text-secondary, #6b7280)' }}
        >
          <strong>For ImgBB images:</strong> Paste the direct image URL from
          ImgBB's embed codes (HTML or BBCode) for best results. You can also
          paste the page URL (https://ibb.co/...) and it will be converted
          automatically.
        </p>
        <input
          type="url"
          id="featuredImageUrl"
          value={featuredImageUrl}
          onChange={(e) => {
            const url = e.target.value
            // Convert ImgBB URLs automatically
            const convertedUrl = convertImgBBUrl(url)
            setFeaturedImageUrl(convertedUrl)
          }}
          onPaste={(e) => {
            // Extract direct URL from pasted embed code
            const pastedText = e.clipboardData.getData('text/plain')
            if (pastedText) {
              const directUrl =
                extractDirectImageUrl(pastedText) || convertImgBBUrl(pastedText)
              if (directUrl !== pastedText || directUrl.includes('i.ibb.co')) {
                e.preventDefault()
                setFeaturedImageUrl(directUrl)
              }
            }
          }}
          placeholder="https://example.com/image.jpg or paste ImgBB embed code"
          className="w-full px-3 py-2 border rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent transition-colors placeholder-gray-500"
          style={{
            borderColor: 'var(--border-color, #d1d5db)',
            color: '#000000',
            backgroundColor: 'var(--bg-primary, #ffffff)',
          }}
        />
        {featuredImageUrl && (
          <img
            src={convertImgBBUrl(featuredImageUrl)}
            alt="Featured"
            className="mt-3 max-w-xs rounded-lg border shadow-sm"
            style={{ borderColor: 'var(--border-color, #d1d5db)' }}
            onError={(e) => {
              ;(e.target as HTMLImageElement).style.display = 'none'
            }}
          />
        )}
      </div>

      {/* Status */}
      <div>
        <label
          htmlFor="status"
          className="block text-sm font-medium mb-2"
          style={{ color: '#000000' }}
        >
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
            backgroundColor: 'var(--bg-primary, #ffffff)',
          }}
        >
          <option value="draft">Draft</option>
          <option value="published">Published</option>
        </select>
      </div>

      {/* SEO Section */}
      <div
        className="border-t pt-6"
        style={{ borderColor: 'var(--border-color, #e5e7eb)' }}
      >
        <h3 className="text-lg font-semibold mb-4" style={{ color: '#000000' }}>
          SEO Settings
        </h3>

        {/* Meta Title */}
        <div className="mb-4">
          <label
            htmlFor="metaTitle"
            className="block text-sm font-medium mb-2"
            style={{ color: '#000000' }}
          >
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
              backgroundColor: 'var(--bg-primary, #ffffff)',
            }}
          />
        </div>

        {/* Meta Description */}
        <div className="mb-4">
          <label
            htmlFor="metaDescription"
            className="block text-sm font-medium mb-2"
            style={{ color: '#000000' }}
          >
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
              backgroundColor: 'var(--bg-primary, #ffffff)',
            }}
          />
        </div>

        {/* Meta Keywords */}
        <div className="mb-4">
          <label
            htmlFor="metaKeywords"
            className="block text-sm font-medium mb-2"
            style={{ color: '#000000' }}
          >
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
              backgroundColor: 'var(--bg-primary, #ffffff)',
            }}
          />
        </div>

        {/* OG Image URL */}
        <div>
          <label
            htmlFor="ogImageUrl"
            className="block text-sm font-medium mb-2"
            style={{ color: '#000000' }}
          >
            Open Graph Image URL (optional, defaults to featured image)
          </label>
          <input
            type="url"
            id="ogImageUrl"
            value={ogImageUrl}
            onChange={(e) => {
              const url = e.target.value
              // Convert ImgBB URLs automatically
              const convertedUrl = convertImgBBUrl(url)
              setOgImageUrl(convertedUrl)
            }}
            onPaste={(e) => {
              // Extract direct URL from pasted embed code
              const pastedText = e.clipboardData.getData('text/plain')
              if (pastedText) {
                const directUrl =
                  extractDirectImageUrl(pastedText) ||
                  convertImgBBUrl(pastedText)
                if (
                  directUrl !== pastedText ||
                  directUrl.includes('i.ibb.co')
                ) {
                  e.preventDefault()
                  setOgImageUrl(directUrl)
                }
              }
            }}
            placeholder="https://example.com/og-image.jpg or paste ImgBB embed code"
            className="w-full px-3 py-2 border rounded-md focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent transition-colors placeholder-gray-500"
            style={{
              borderColor: 'var(--border-color, #d1d5db)',
              color: '#000000',
              backgroundColor: 'var(--bg-primary, #ffffff)',
            }}
          />
        </div>
      </div>

      {/* Actions */}
      <div
        className="flex gap-4 justify-end pt-4 border-t"
        style={{ borderColor: 'var(--border-color, #e5e7eb)' }}
      >
        <button
          type="button"
          onClick={onCancel}
          className="px-6 py-2 border rounded-md hover:bg-gray-50 transition-colors font-medium"
          style={{
            borderColor: 'var(--border-color, #d1d5db)',
            color: '#000000',
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
          {isLoading
            ? 'Saving...'
            : article
              ? 'Update Article'
              : 'Create Article'}
        </button>
      </div>
    </form>
  )
}
