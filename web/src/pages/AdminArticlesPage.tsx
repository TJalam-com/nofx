import { useState, useMemo, useRef, useEffect, useCallback } from 'react'
import { useNavigate } from 'react-router-dom'
import useSWR from 'swr'
import { api } from '../lib/api'
import { useAuth, isAdmin } from '../contexts/AuthContext'
import { toast } from 'sonner'
import { ArticleForm } from '../components/articles/ArticleForm'
import type { Article, CreateArticleRequest, UpdateArticleRequest } from '../types'
import {
  Plus,
  Edit,
  Trash2,
  Eye,
  EyeOff,
  Search,
  ChevronLeft,
  ChevronRight,
  FileText,
  X,
} from 'lucide-react'

export default function AdminArticlesPage() {
  const { user } = useAuth()
  const navigate = useNavigate()
  const [filterStatus, setFilterStatus] = useState<'all' | 'draft' | 'published'>('all')
  const [searchQuery, setSearchQuery] = useState('')
  const [currentPage, setCurrentPage] = useState(1)
  const [editingArticle, setEditingArticle] = useState<Article | null>(null)
  const [showCreateForm, setShowCreateForm] = useState(false)
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [currentFormData, setCurrentFormData] = useState<{ title: string; content: string; excerpt: string; slug: string } | null>(null)
  const [isAutoSaving, setIsAutoSaving] = useState(false)
  const modalContentRef = useRef<HTMLDivElement>(null)
  const articlesPerPage = 10

  // Redirect if not admin
  if (!isAdmin(user)) {
    navigate('/traders')
    return null
  }

  // Fetch articles
  const {
    data: articles,
    mutate: mutateArticles,
    isLoading: articlesLoading,
  } = useSWR<Article[]>(
    user ? ['admin-articles', filterStatus] : null,
    () => api.getArticles(filterStatus === 'all' ? undefined : filterStatus),
    { refreshInterval: 30000 }
  )

  // Filter articles by search query
  const filteredArticles = useMemo(() => {
    if (!articles) return []
    
    if (!searchQuery.trim()) return articles
    
    const query = searchQuery.toLowerCase()
    return articles.filter(
      (article) =>
        article.title.toLowerCase().includes(query) ||
        article.slug.toLowerCase().includes(query) ||
        article.excerpt.toLowerCase().includes(query) ||
        article.meta_description.toLowerCase().includes(query)
    )
  }, [articles, searchQuery])

  // Paginate filtered articles
  const paginatedArticles = useMemo(() => {
    const startIndex = (currentPage - 1) * articlesPerPage
    return filteredArticles.slice(startIndex, startIndex + articlesPerPage)
  }, [filteredArticles, currentPage])

  const totalPages = Math.ceil(filteredArticles.length / articlesPerPage)

  const handleCreate = async (data: CreateArticleRequest) => {
    setIsSubmitting(true)
    try {
      await api.createArticle(data)
      toast.success('Article created successfully')
      setShowCreateForm(false)
      mutateArticles()
    } catch (error: any) {
      toast.error(error.message || 'Failed to create article')
    } finally {
      setIsSubmitting(false)
    }
  }

  const handleUpdate = async (data: UpdateArticleRequest) => {
    if (!editingArticle) return
    
    setIsSubmitting(true)
    try {
      await api.updateArticle(editingArticle.id, data)
      toast.success('Article updated successfully')
      setEditingArticle(null)
      mutateArticles()
    } catch (error: any) {
      toast.error(error.message || 'Failed to update article')
    } finally {
      setIsSubmitting(false)
    }
  }

  const handleDelete = async (id: string) => {
    if (!confirm('Are you sure you want to delete this article?')) return
    
    try {
      await api.deleteArticle(id)
      toast.success('Article deleted successfully')
      mutateArticles()
    } catch (error: any) {
      toast.error(error.message || 'Failed to delete article')
    }
  }

  const handlePublish = async (id: string) => {
    const article = articles?.find(a => a.id === id)
    if (!article) return

    // Validate required fields before publishing
    const missingFields: string[] = []
    if (!article.title || !article.title.trim()) {
      missingFields.push('Title')
    }
    if (!article.content || !article.content.trim()) {
      missingFields.push('Content')
    }
    if (!article.meta_description || !article.meta_description.trim()) {
      missingFields.push('Meta Description')
    }

    if (missingFields.length > 0) {
      toast.error(`Cannot publish: Missing required fields: ${missingFields.join(', ')}. Please edit the article to add these fields.`)
      return
    }

    try {
      await api.publishArticle(id)
      toast.success('Article published successfully')
      // Force revalidation to get updated data
      await mutateArticles()
      // Also update the specific article in the cache if it exists
      if (articles) {
        const updatedArticles = articles.map(a => 
          a.id === id ? { ...a, status: 'published' as const, published_at: new Date().toISOString() } : a
        )
        mutateArticles(updatedArticles, false)
      }
    } catch (error: any) {
      toast.error(error.message || 'Failed to publish article')
    }
  }

  // Check if article can be published (has all required fields)
  const canPublish = (article: Article): boolean => {
    return !!(
      article.title?.trim() &&
      article.content?.trim() &&
      article.meta_description?.trim()
    )
  }

  const handleUnpublish = async (id: string) => {
    try {
      await api.unpublishArticle(id)
      toast.success('Article unpublished successfully')
      // Force revalidation to get updated data
      await mutateArticles()
      // Also update the specific article in the cache if it exists
      if (articles) {
        const updatedArticles = articles.map(a => 
          a.id === id ? { ...a, status: 'draft' as const, published_at: undefined } : a
        )
        mutateArticles(updatedArticles, false)
      }
    } catch (error: any) {
      toast.error(error.message || 'Failed to unpublish article')
    }
  }

  // Auto-save as draft when closing
  const handleClose = useCallback(async () => {
    // Only auto-save if there's meaningful content (title or content)
    if (currentFormData && (currentFormData.title?.trim() || currentFormData.content?.trim())) {
      const hasChanges = editingArticle
        ? (currentFormData.title !== editingArticle.title ||
           currentFormData.content !== editingArticle.content ||
           currentFormData.excerpt !== (editingArticle.excerpt || '') ||
           currentFormData.slug !== editingArticle.slug)
        : true // New article always has changes if there's content

      if (hasChanges) {
        setIsAutoSaving(true)
        try {
          if (editingArticle) {
            // Update existing article as draft
            await api.updateArticle(editingArticle.id, {
              title: currentFormData.title?.trim() || editingArticle.title,
              content: currentFormData.content || '',
              excerpt: currentFormData.excerpt || '',
              status: 'draft',
            })
            toast.success('Article saved as draft')
          } else {
            // Create new article as draft only if there's a title or content
            if (currentFormData.title?.trim() || currentFormData.content?.trim()) {
              await api.createArticle({
                title: currentFormData.title?.trim() || 'Untitled',
                content: currentFormData.content || '',
                excerpt: currentFormData.excerpt || '',
                slug: currentFormData.slug || undefined,
                status: 'draft',
              })
              toast.success('Article saved as draft')
            }
          }
          mutateArticles()
        } catch (error: any) {
          toast.error(error.message || 'Failed to auto-save article')
        } finally {
          setIsAutoSaving(false)
        }
      }
    }
    
    setShowCreateForm(false)
    setEditingArticle(null)
    setCurrentFormData(null)
  }, [currentFormData, editingArticle, mutateArticles])

  // Handle click outside modal
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (modalContentRef.current && !modalContentRef.current.contains(event.target as Node)) {
        handleClose()
      }
    }

    if (showCreateForm || editingArticle) {
      document.addEventListener('mousedown', handleClickOutside)
      return () => {
        document.removeEventListener('mousedown', handleClickOutside)
      }
    }
  }, [showCreateForm, editingArticle, handleClose])

  return (
    <div className="container mx-auto px-4 py-8 max-w-7xl">
      <div className="flex flex-col sm:flex-row sm:justify-between sm:items-center gap-4 mb-6">
        <h1 className="text-2xl sm:text-3xl font-bold" style={{ color: 'var(--text-primary, #111827)' }}>
          Article Management
        </h1>
        <button
          onClick={() => {
            setShowCreateForm(true)
            setEditingArticle(null)
          }}
          className="flex items-center justify-center gap-2 px-4 py-2 bg-blue-500 text-white rounded hover:bg-blue-600 whitespace-nowrap text-sm sm:text-base"
        >
          <Plus size={20} />
          Create Article
        </button>
      </div>

      {/* Filters and Search */}
      <div className="mb-6 flex flex-col sm:flex-row gap-4">
        <div className="flex-1 relative">
          <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 text-gray-400" size={20} />
          <input
            type="text"
            placeholder="Search articles..."
            value={searchQuery}
            onChange={(e) => {
              setSearchQuery(e.target.value)
              setCurrentPage(1)
            }}
            className="w-full pl-10 pr-4 py-2 border rounded placeholder-gray-500"
            style={{ borderColor: 'var(--border-color, #e5e7eb)', color: '#000000', backgroundColor: 'var(--bg-primary, #ffffff)' }}
          />
        </div>
        <div className="flex gap-2">
          <button
            onClick={() => {
              setFilterStatus('all')
              setCurrentPage(1)
            }}
            className={`px-4 py-2 rounded ${filterStatus === 'all' ? 'bg-blue-500 text-white' : 'border'}`}
            style={filterStatus !== 'all' ? { borderColor: 'var(--border-color, #e5e7eb)', color: 'var(--text-primary, #111827)' } : {}}
          >
            All
          </button>
          <button
            onClick={() => {
              setFilterStatus('draft')
              setCurrentPage(1)
            }}
            className={`px-4 py-2 rounded ${filterStatus === 'draft' ? 'bg-blue-500 text-white' : 'border'}`}
            style={filterStatus !== 'draft' ? { borderColor: 'var(--border-color, #e5e7eb)', color: 'var(--text-primary, #111827)' } : {}}
          >
            Draft
          </button>
          <button
            onClick={() => {
              setFilterStatus('published')
              setCurrentPage(1)
            }}
            className={`px-4 py-2 rounded ${filterStatus === 'published' ? 'bg-blue-500 text-white' : 'border'}`}
            style={filterStatus !== 'published' ? { borderColor: 'var(--border-color, #e5e7eb)', color: 'var(--text-primary, #111827)' } : {}}
          >
            Published
          </button>
        </div>
      </div>

      {/* Create/Edit Form Modal */}
      {(showCreateForm || editingArticle) && (
        <div className="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center z-50 p-4 overflow-y-auto">
          <div 
            ref={modalContentRef}
            className="bg-white rounded-lg p-6 max-w-4xl w-full max-h-[90vh] overflow-y-auto relative" 
            style={{ backgroundColor: 'var(--bg-primary, #ffffff)' }}
          >
            {/* Close Button */}
            <button
              onClick={handleClose}
              className="absolute top-4 right-4 p-2 rounded-md hover:bg-gray-100 transition-colors z-20"
              style={{ color: '#000000' }}
              disabled={isAutoSaving || isSubmitting}
              title="Close (auto-saves as draft)"
            >
              <X size={20} />
            </button>
            
            {/* Header */}
            <div className="pr-8 mb-4">
              <h2 className="text-2xl font-bold" style={{ color: '#000000' }}>
                {editingArticle ? 'Edit Article' : 'Create Article'}
              </h2>
              {isAutoSaving && (
                <p className="text-sm mt-1" style={{ color: 'var(--text-secondary, #6b7280)' }}>
                  Auto-saving as draft...
                </p>
              )}
            </div>
            
            <ArticleForm
              article={editingArticle || undefined}
              onSubmit={editingArticle ? (data) => handleUpdate(data as UpdateArticleRequest) : (data) => handleCreate(data as CreateArticleRequest)}
              onCancel={handleClose}
              isLoading={isSubmitting || isAutoSaving}
              onFormChange={setCurrentFormData}
            />
          </div>
        </div>
      )}

      {/* Articles List */}
      {articlesLoading ? (
        <div className="text-center py-12" style={{ color: 'var(--text-secondary, #6b7280)' }}>
          Loading articles...
        </div>
      ) : paginatedArticles.length === 0 ? (
        <div className="text-center py-12" style={{ color: 'var(--text-secondary, #6b7280)' }}>
          <FileText size={48} className="mx-auto mb-4 opacity-50" />
          <p>No articles found</p>
        </div>
      ) : (
        <>
          <div className="space-y-4">
            {paginatedArticles.map((article) => (
              <div
                key={article.id}
                className="border rounded-lg p-4 hover:shadow-md transition-shadow"
                style={{ borderColor: 'var(--border-color, #e5e7eb)', backgroundColor: 'var(--bg-primary, #ffffff)' }}
              >
                <div className="flex flex-col sm:flex-row sm:justify-between sm:items-start gap-4">
                  <div className="flex-1 min-w-0">
                    <div className="flex flex-wrap items-center gap-2 mb-2">
                      <h3 className="text-lg sm:text-xl font-semibold break-words" style={{ color: '#000000' }}>
                        {article.title}
                      </h3>
                      <span
                        className={`px-2 py-1 text-xs rounded shrink-0 ${
                          article.status === 'published' ? 'bg-green-100 text-green-800' : 'bg-gray-100 text-gray-800'
                        }`}
                      >
                        {article.status}
                      </span>
                    </div>
                    <p className="text-xs sm:text-sm mb-2 break-words" style={{ color: '#000000' }}>
                      {article.excerpt || 'No excerpt'}
                    </p>
                    <p className="text-xs break-words" style={{ color: '#000000' }}>
                      Slug: <code className="bg-gray-100 px-1 rounded break-all" style={{ color: '#000000' }}>{article.slug}</code> • 
                      Created: {new Date(article.created_at).toLocaleDateString()}
                      {article.status === 'published' ? (
                        article.published_at ? (
                          ` • Published: ${new Date(article.published_at).toLocaleDateString()}`
                        ) : (
                          ' • Published: Just now'
                        )
                      ) : (
                        ' • Not published'
                      )}
                    </p>
                  </div>
                  <div className="flex flex-wrap gap-2 shrink-0 sm:ml-4">
                    {article.status === 'published' && (
                      <a
                        href={`/blog/${article.slug}`}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="p-2 border rounded hover:bg-gray-100"
                        style={{ borderColor: 'var(--border-color, #e5e7eb)', color: '#000000' }}
                        title="View article"
                      >
                        <Eye size={18} />
                      </a>
                    )}
                    <button
                      onClick={() => setEditingArticle(article)}
                      className="p-2 border rounded hover:bg-gray-100"
                      style={{ borderColor: 'var(--border-color, #e5e7eb)', color: '#000000' }}
                      title="Edit article"
                    >
                      <Edit size={18} />
                    </button>
                    {article.status === 'draft' ? (
                      <button
                        onClick={() => handlePublish(article.id)}
                        disabled={!canPublish(article)}
                        className="p-2 border rounded hover:bg-gray-100 disabled:opacity-50 disabled:cursor-not-allowed"
                        style={{ borderColor: 'var(--border-color, #e5e7eb)', color: '#000000' }}
                        title={
                          canPublish(article)
                            ? 'Publish article'
                            : 'Cannot publish: Missing required fields (Title, Content, or Meta Description)'
                        }
                      >
                        <Eye size={18} />
                      </button>
                    ) : (
                      <button
                        onClick={() => handleUnpublish(article.id)}
                        className="p-2 border rounded hover:bg-gray-100"
                        style={{ borderColor: 'var(--border-color, #e5e7eb)', color: '#000000' }}
                        title="Unpublish article"
                      >
                        <EyeOff size={18} />
                      </button>
                    )}
                    <button
                      onClick={() => handleDelete(article.id)}
                      className="p-2 border rounded hover:bg-red-100"
                      style={{ borderColor: 'var(--border-color, #e5e7eb)', color: '#000000' }}
                      title="Delete article"
                    >
                      <Trash2 size={18} />
                    </button>
                  </div>
                </div>
              </div>
            ))}
          </div>

          {/* Pagination */}
          {totalPages > 1 && (
            <div className="flex justify-center items-center gap-2 mt-6">
              <button
                onClick={() => setCurrentPage((p) => Math.max(1, p - 1))}
                disabled={currentPage === 1}
                className="p-2 border rounded disabled:opacity-50"
                style={{ borderColor: 'var(--border-color, #e5e7eb)', color: 'var(--text-primary, #111827)' }}
              >
                <ChevronLeft size={20} />
              </button>
              <span className="px-4" style={{ color: 'var(--text-primary, #111827)' }}>
                Page {currentPage} of {totalPages}
              </span>
              <button
                onClick={() => setCurrentPage((p) => Math.min(totalPages, p + 1))}
                disabled={currentPage === totalPages}
                className="p-2 border rounded disabled:opacity-50"
                style={{ borderColor: 'var(--border-color, #e5e7eb)', color: 'var(--text-primary, #111827)' }}
              >
                <ChevronRight size={20} />
              </button>
            </div>
          )}
        </>
      )}
    </div>
  )
}
