import { useState, useEffect } from 'react'
import type { PromptTemplate, CreatePromptTemplateRequest, UpdatePromptTemplateRequest } from '../types'
import { useLanguage } from '../contexts/LanguageContext'
import { t } from '../i18n/translations'
import { toast } from 'sonner'
import { X as IconX, Save, Trash2, Copy, Eye, FileText, ArrowLeft, Search, Download, Upload } from 'lucide-react'
import { api } from '../lib/api'

interface PromptTemplateModalProps {
  isOpen: boolean
  onClose: () => void
  template?: PromptTemplate | null
  onSave?: () => void
  onDelete?: () => void
  onSelectTemplate?: (templateId: string | null) => void
}

export function PromptTemplateModal({
  isOpen,
  onClose,
  template,
  onSave,
  onDelete,
  onSelectTemplate,
}: PromptTemplateModalProps) {
  const { language } = useLanguage()
  const [name, setName] = useState('')
  const [content, setContent] = useState('')
  const [isSaving, setIsSaving] = useState(false)
  const [isDeleting, setIsDeleting] = useState(false)
  const isEditMode = !!template
  const isSystemTemplate = template?.is_system || false

  // Template selection and copy feature
  const [activeTab, setActiveTab] = useState<'create' | 'copy'>('create')
  const [availableTemplates, setAvailableTemplates] = useState<PromptTemplate[]>([])
  const [selectedTemplateForView, setSelectedTemplateForView] = useState<PromptTemplate | null>(null)
  const [viewMode, setViewMode] = useState<'browse' | 'view'>('browse')
  const [searchQuery, setSearchQuery] = useState('')
  const [isLoadingTemplates, setIsLoadingTemplates] = useState(false)

  useEffect(() => {
    if (template) {
      setName(template.name)
      setContent(template.content)
      setActiveTab('create') // When editing, always show create tab
      setViewMode('browse')
      setSelectedTemplateForView(null)
    } else {
      setName('')
      setContent('')
      // When creating new, reset to create tab if not copying
      if (!selectedTemplateForView) {
        setActiveTab('create')
      }
      setViewMode('browse')
    }
  }, [template, isOpen])

  // Reset state when modal closes
  useEffect(() => {
    if (!isOpen) {
      setActiveTab('create')
      setViewMode('browse')
      setSelectedTemplateForView(null)
      setSearchQuery('')
      setAvailableTemplates([])
    }
  }, [isOpen])

  // Fetch available templates when opening modal in copy mode
  useEffect(() => {
    if (isOpen && !isEditMode && activeTab === 'copy' && availableTemplates.length === 0) {
      fetchAvailableTemplates()
    }
  }, [isOpen, isEditMode, activeTab])

  const fetchAvailableTemplates = async () => {
    setIsLoadingTemplates(true)
    try {
      const userTemplates = await api.getUserPromptTemplates()
      setAvailableTemplates(userTemplates)
    } catch (error) {
      console.error('Failed to fetch templates:', error)
      toast.error(t('failedToFetchTemplates', language) || 'Failed to fetch templates')
    } finally {
      setIsLoadingTemplates(false)
    }
  }

  const handleCopyTemplate = (templateToCopy: PromptTemplate) => {
    // Generate unique copy name
    let copyName = `${templateToCopy.name} (Copy)`
    let counter = 1
    while (availableTemplates.some(t => t.name === copyName)) {
      counter++
      copyName = `${templateToCopy.name} (Copy ${counter})`
    }

    setName(copyName)
    setContent(templateToCopy.content)
    setActiveTab('create')
    setViewMode('browse')
    setSelectedTemplateForView(null)
    toast.success(t('templateCopied', language) || 'Template copied to create form')
  }

  const filteredTemplates = availableTemplates.filter(t => 
    t.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
    t.content.toLowerCase().includes(searchQuery.toLowerCase())
  )

  const systemTemplates = filteredTemplates.filter(t => t.is_system)
  const userTemplates = filteredTemplates.filter(t => !t.is_system)

  if (!isOpen) return null

  const handleSave = async () => {
    if (!name.trim()) {
      toast.error(t('templateNameRequired', language) || 'Template name is required')
      return
    }
    if (!content.trim()) {
      toast.error(t('templateContentRequired', language) || 'Template content is required')
      return
    }

    setIsSaving(true)
    try {
      if (isEditMode) {
        const updateRequest: UpdatePromptTemplateRequest = {
          name: name.trim(),
          content: content.trim(),
        }
        const updatedTemplate: PromptTemplate = await toast.promise(api.updatePromptTemplate(template!.id, updateRequest), {
          loading: t('savingTemplate', language) || 'Saving template...',
          success: t('templateSaved', language) || 'Template saved',
          error: t('templateSaveFailed', language) || 'Failed to save template',
        })
        // Auto-select updated template if callback provided
        if (updatedTemplate && onSelectTemplate) {
          onSelectTemplate(updatedTemplate.id)
        }
      } else {
        const createRequest: CreatePromptTemplateRequest = {
          name: name.trim(),
          content: content.trim(),
        }
        const newTemplate: PromptTemplate = await toast.promise(api.createPromptTemplate(createRequest), {
          loading: t('creatingTemplate', language) || 'Creating template...',
          success: t('templateCreated', language) || 'Template created',
          error: t('templateCreateFailed', language) || 'Failed to create template',
        })
        // Auto-select newly created template if callback provided
        if (newTemplate && onSelectTemplate) {
          onSelectTemplate(newTemplate.id)
        }
      }
      onSave?.()
      onClose()
    } catch (error) {
      console.error('Failed to save template:', error)
    } finally {
      setIsSaving(false)
    }
  }

  const handleDelete = async () => {
    if (!template || isSystemTemplate) return

    const confirmMessage = t('confirmDeleteTemplate', language) || 'Are you sure you want to delete this template?'
    if (!window.confirm(confirmMessage)) {
      return
    }

    setIsDeleting(true)
    try {
      await toast.promise(api.deletePromptTemplate(template.id), {
        loading: t('deletingTemplate', language) || 'Deleting template...',
        success: t('templateDeleted', language) || 'Template deleted',
        error: t('templateDeleteFailed', language) || 'Failed to delete template',
      })
      // Clear selection after deletion if callback provided
      if (onSelectTemplate) {
        onSelectTemplate(null)
      }
      onDelete?.()
      onClose()
    } catch (error) {
      console.error('Failed to delete template:', error)
    } finally {
      setIsDeleting(false)
    }
  }

  const handleExport = () => {
    if (!name.trim() || !content.trim()) {
      toast.error(t('templateNameAndContentRequired', language) || 'Template name and content are required for export')
      return
    }

    const data = {
      name: name.trim(),
      content: content.trim(),
      is_system: isSystemTemplate,
    }
    const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `${name.trim() || 'template'}-${Date.now()}.json`
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
    URL.revokeObjectURL(url)
    toast.success(t('templateExported', language) || 'Template exported successfully')
  }

  const handleImport = () => {
    const input = document.createElement('input')
    input.type = 'file'
    input.accept = '.json,.txt'
    input.onchange = (e) => {
      const file = (e.target as HTMLInputElement).files?.[0]
      if (!file) return

      const reader = new FileReader()
      reader.onload = (event) => {
        try {
          const text = event.target?.result as string
          if (file.name.endsWith('.json')) {
            const data = JSON.parse(text)
            if (!data.name || !data.content) {
              toast.error(t('invalidTemplateFile', language) || 'Invalid template file: name and content are required')
              return
            }
            setName(data.name)
            setContent(data.content)
          } else {
            // Plain text: use filename as name
            const nameWithoutExt = file.name.replace(/\.[^/.]+$/, '')
            setName(nameWithoutExt)
            setContent(text)
          }
          setActiveTab('create')
          toast.success(t('templateImported', language) || 'Template imported successfully')
        } catch (error) {
          console.error('Failed to import template:', error)
          toast.error(t('failedToImportTemplate', language) || 'Failed to import template. Please check the file format.')
        }
      }
      reader.readAsText(file)
    }
    input.click()
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center backdrop-blur-sm p-4 overflow-y-auto" style={{ background: 'rgba(0, 31, 63, 0.5)' }}>
      <div
        className="rounded-xl shadow-2xl max-w-4xl w-full my-8"
        style={{ background: 'var(--panel-bg)', border: '1px solid var(--panel-border)', maxHeight: 'calc(100vh - 4rem)' }}
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div className="flex items-center justify-between p-6 border-b sticky top-0 z-10 rounded-t-xl"
        style={{ borderColor: 'var(--panel-border)', background: 'var(--panel-bg)' }}>
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-lg bg-gradient-to-br from-[#00CC66] to-[#00AA55] flex items-center justify-center" style={{ color: 'var(--navy-primary)' }}>
              <Save className="w-5 h-5" />
            </div>
            <div>
              <h2 className="text-xl font-bold text-[#EAECEF]">
                {isEditMode
                  ? t('editTemplate', language) || 'Edit Template'
                  : t('createTemplate', language) || 'Create Template'}
              </h2>
              <p className="text-sm text-[#848E9C] mt-1">
                {isEditMode
                  ? t('editTemplateSubtitle', language) || 'Edit your prompt template'
                  : t('createTemplateSubtitle', language) || 'Create a new prompt template'}
              </p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="w-8 h-8 rounded-lg text-[#848E9C] hover:text-[#EAECEF] transition-colors flex items-center justify-center"
            style={{ '--hover-bg': 'var(--panel-border)' } as React.CSSProperties}
          >
            <IconX className="w-4 h-4" />
          </button>
        </div>

        {/* Tabs - Only show when creating new (not editing) */}
        {!isEditMode && (
          <div className="border-b px-6" style={{ borderColor: 'var(--panel-border)' }}>
            <div className="flex gap-1">
              <button
                onClick={() => {
                  setActiveTab('create')
                  setViewMode('browse')
                  setSelectedTemplateForView(null)
                }}
                className={`px-4 py-3 text-sm font-medium transition-colors border-b-2 ${
                  activeTab === 'create'
                    ? 'border-[#00CC66] text-[#00CC66]'
                    : 'border-transparent text-[#848E9C] hover:text-[#EAECEF]'
                }`}
              >
                {t('createNew', language) || 'Create New'}
              </button>
              <button
                onClick={() => {
                  setActiveTab('copy')
                  setViewMode('browse')
                  if (availableTemplates.length === 0) {
                    fetchAvailableTemplates()
                  }
                }}
                className={`px-4 py-3 text-sm font-medium transition-colors border-b-2 ${
                  activeTab === 'copy'
                    ? 'border-[#00CC66] text-[#00CC66]'
                    : 'border-transparent text-[#848E9C] hover:text-[#EAECEF]'
                }`}
              >
                {t('copyFromTemplate', language) || 'Copy from Template'}
              </button>
            </div>
          </div>
        )}

        {/* Content */}
        <div
          className="p-6 space-y-6 overflow-y-auto"
          style={{ maxHeight: 'calc(100vh - 16rem)' }}
        >
          {/* Copy from Template Tab Content */}
          {!isEditMode && activeTab === 'copy' && (
            <>
              {viewMode === 'browse' ? (
                <>
                  {/* Search */}
                  <div className="relative">
                    <Search className="absolute left-3 top-1/2 transform -translate-y-1/2 w-4 h-4 text-[#848E9C]" />
                    <input
                      type="text"
                      value={searchQuery}
                      onChange={(e) => setSearchQuery(e.target.value)}
                      placeholder={t('searchTemplates', language) || 'Search templates...'}
                      className="w-full pl-10 pr-3 py-2 rounded text-[#EAECEF] focus:border-[#00CC66] focus:outline-none"
                      style={{ background: 'var(--navy-primary)', border: '1px solid var(--panel-border)' }}
                    />
                  </div>

                  {isLoadingTemplates ? (
                    <div className="text-center py-8 text-[#848E9C]">
                      {t('loadingTemplates', language) || 'Loading templates...'}
                    </div>
                  ) : (
                    <>
                      {/* System Templates */}
                      {systemTemplates.length > 0 && (
                        <div>
                          <h3 className="text-sm font-semibold text-[#EAECEF] mb-3 flex items-center gap-2">
                            <FileText className="w-4 h-4" />
                            {t('systemTemplates', language) || 'System Templates'}
                          </h3>
                          <div className="grid grid-cols-1 gap-3">
                            {systemTemplates.map((tmpl) => (
                              <div
                                key={tmpl.id}
                                onClick={() => {
                                  setSelectedTemplateForView(tmpl)
                                  setViewMode('view')
                                }}
                                className="p-4 rounded-lg hover:border-[#00CC66] cursor-pointer transition-colors"
                                style={{ background: 'var(--navy-primary)', border: '1px solid var(--panel-border)' }}
                              >
                                <div className="flex items-start justify-between">
                                  <div className="flex-1">
                                    <div className="flex items-center gap-2 mb-1">
                                      <h4 className="text-sm font-medium text-[#EAECEF]">{tmpl.name}</h4>
                                      <span className="px-2 py-0.5 text-xs text-[#848E9C] rounded"
                                      style={{ background: 'var(--panel-border)' }}>
                                        {t('templateSystem', language) || 'System'}
                                      </span>
                                    </div>
                                    <p className="text-xs text-[#848E9C] line-clamp-2">
                                      {tmpl.content.substring(0, 150)}...
                                    </p>
                                  </div>
                                  <Eye className="w-4 h-4 text-[#848E9C] ml-2 flex-shrink-0" />
                                </div>
                              </div>
                            ))}
                          </div>
                        </div>
                      )}

                      {/* User Templates */}
                      {userTemplates.length > 0 && (
                        <div>
                          <h3 className="text-sm font-semibold text-[#EAECEF] mb-3 flex items-center gap-2">
                            <FileText className="w-4 h-4" />
                            {t('yourTemplates', language) || 'Your Templates'}
                          </h3>
                          <div className="grid grid-cols-1 gap-3">
                            {userTemplates.map((tmpl) => (
                              <div
                                key={tmpl.id}
                                onClick={() => {
                                  setSelectedTemplateForView(tmpl)
                                  setViewMode('view')
                                }}
                                className="p-4 rounded-lg hover:border-[#00CC66] cursor-pointer transition-colors"
                                style={{ background: 'var(--navy-primary)', border: '1px solid var(--panel-border)' }}
                              >
                                <div className="flex items-start justify-between">
                                  <div className="flex-1">
                                    <div className="flex items-center gap-2 mb-1">
                                      <h4 className="text-sm font-medium text-[#EAECEF]">{tmpl.name}</h4>
                                      <span className="px-2 py-0.5 text-xs bg-[#00CC66] bg-opacity-20 text-[#00CC66] rounded">
                                        {t('templateUser', language) || 'User'}
                                      </span>
                                    </div>
                                    <p className="text-xs text-[#848E9C] line-clamp-2">
                                      {tmpl.content.substring(0, 150)}...
                                    </p>
                                    <p className="text-xs text-[#848E9C] mt-1">
                                      {new Date(tmpl.updated_at).toLocaleDateString()}
                                    </p>
                                  </div>
                                  <Eye className="w-4 h-4 text-[#848E9C] ml-2 flex-shrink-0" />
                                </div>
                              </div>
                            ))}
                          </div>
                        </div>
                      )}

                      {filteredTemplates.length === 0 && !isLoadingTemplates && (
                        <div className="text-center py-8 text-[#848E9C]">
                          {searchQuery
                            ? t('noTemplatesFound', language) || 'No templates found'
                            : t('noTemplatesAvailable', language) || 'No templates available'}
                        </div>
                      )}
                    </>
                  )}
                </>
              ) : (
                /* Template View Mode */
                selectedTemplateForView && (
                  <div className="space-y-4">
                    <button
                      onClick={() => {
                        setViewMode('browse')
                        setSelectedTemplateForView(null)
                      }}
                      className="flex items-center gap-2 text-sm text-[#848E9C] hover:text-[#EAECEF] transition-colors"
                    >
                      <ArrowLeft className="w-4 h-4" />
                      {t('backToList', language) || 'Back to List'}
                    </button>

                    <div className="p-4 rounded-lg" style={{ background: 'var(--navy-primary)', border: '1px solid var(--panel-border)' }}>
                      <div className="flex items-center justify-between mb-3">
                        <div>
                          <h3 className="text-lg font-semibold text-[#EAECEF]">{selectedTemplateForView.name}</h3>
                          <div className="flex items-center gap-2 mt-1">
                            <span
                              className={`px-2 py-0.5 text-xs rounded ${
                                selectedTemplateForView.is_system
                                  ? 'text-[#848E9C]'
                                  : 'bg-[#00CC66] bg-opacity-20 text-[#00CC66]'
                              }`}
                            >
                              {selectedTemplateForView.is_system
                                ? t('templateSystem', language) || 'System'
                                : t('templateUser', language) || 'User'}
                            </span>
                            <span className="text-xs text-[#848E9C]">
                              {t('templateUpdated', language) || 'Updated'}:{' '}
                              {new Date(selectedTemplateForView.updated_at).toLocaleDateString()}
                            </span>
                          </div>
                        </div>
                        <button
                          onClick={() => handleCopyTemplate(selectedTemplateForView)}
                          className="px-4 py-2 bg-gradient-to-r from-[#00CC66] to-[#00AA55] rounded-lg hover:from-[#00AA55] hover:to-[#008844] transition-all duration-200 font-medium flex items-center gap-2"
                          style={{ color: 'var(--navy-primary)' }}
                        >
                          <Copy className="w-4 h-4" />
                          {t('copyTemplate', language) || 'Copy Template'}
                        </button>
                      </div>

                      <div className="mt-4 p-4 rounded-lg" style={{ background: 'var(--panel-bg)', border: '1px solid var(--panel-border)' }}>
                        <pre className="text-sm text-[#EAECEF] whitespace-pre-wrap font-mono overflow-x-auto">
                          {selectedTemplateForView.content}
                        </pre>
                      </div>
                    </div>
                  </div>
                )
              )}
            </>
          )}

          {/* Create New Tab Content */}
          {(!isEditMode && activeTab === 'create') || isEditMode ? (
            <>
              {/* Template Name */}
              <div>
                <label className="text-sm text-[#EAECEF] block mb-2">
                  {t('templateName', language) || 'Template Name'}
                </label>
                <input
                  type="text"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  disabled={isSystemTemplate}
                  className="w-full px-3 py-2 rounded text-[#EAECEF] focus:border-[#00CC66] focus:outline-none disabled:opacity-50 disabled:cursor-not-allowed"
                  style={{ background: 'var(--navy-primary)', border: '1px solid var(--panel-border)' }}
                  placeholder={t('templateNamePlaceholder', language) || 'Enter template name'}
                />
                {isSystemTemplate && (
                  <p className="text-xs text-[#848E9C] mt-1">
                    {t('systemTemplateReadOnly', language) || 'System templates are read-only'}
                  </p>
                )}
              </div>

              {/* Template Content */}
              <div>
                <div className="flex items-center justify-between mb-2">
                  <label className="text-sm text-[#EAECEF]">
                    {t('templateContent', language) || 'Template Content'}
                  </label>
                  <span className="text-xs text-[#848E9C]">
                    {content.length} {t('characters', language) || 'characters'}
                  </span>
                </div>
                <textarea
                  value={content}
                  onChange={(e) => setContent(e.target.value)}
                  disabled={isSystemTemplate}
                  className="w-full px-3 py-2 rounded text-[#EAECEF] focus:border-[#00CC66] focus:outline-none h-96 resize-y font-mono text-sm disabled:opacity-50 disabled:cursor-not-allowed"
                  style={{ background: 'var(--navy-primary)', border: '1px solid var(--panel-border)' }}
                  placeholder={t('templateContentPlaceholder', language) || 'Enter template content...'}
                />
                {isSystemTemplate && (
                  <p className="text-xs text-[#848E9C] mt-1">
                    {t('systemTemplateReadOnly', language) || 'System templates are read-only'}
                  </p>
                )}
              </div>
            </>
          ) : null}
        </div>

        {/* Footer */}
        <div className="flex justify-between items-center p-6 border-t sticky bottom-0 z-10 rounded-b-xl"
        style={{ borderColor: 'var(--panel-border)', background: 'var(--panel-bg)' }}>
          <div className="flex items-center gap-3">
            {isEditMode && !isSystemTemplate && (
              <button
                onClick={handleDelete}
                disabled={isDeleting}
                className="px-4 py-2 bg-red-600 text-white rounded-lg hover:bg-red-700 transition-colors disabled:bg-[#848E9C] disabled:cursor-not-allowed flex items-center gap-2"
              >
                <Trash2 className="w-4 h-4" />
                {t('delete', language) || 'Delete'}
              </button>
            )}
            {(!isEditMode || !isSystemTemplate) && name.trim() && content.trim() && (
              <button
                onClick={handleExport}
                className="px-4 py-2 text-[#EAECEF] rounded-lg transition-colors flex items-center gap-2"
                style={{ background: 'var(--panel-border)', border: '1px solid var(--panel-border)' }}
              >
                <Download className="w-4 h-4" />
                {t('export', language) || 'Export'}
              </button>
            )}
            {!isEditMode && (
              <button
                onClick={handleImport}
                className="px-4 py-2 text-[#EAECEF] rounded-lg transition-colors flex items-center gap-2"
                style={{ background: 'var(--panel-border)', border: '1px solid var(--panel-border)' }}
              >
                <Upload className="w-4 h-4" />
                {t('import', language) || 'Import'}
              </button>
            )}
          </div>
          <div className="flex gap-3">
            <button
              onClick={onClose}
              className="px-6 py-3 text-[#EAECEF] rounded-lg transition-all duration-200"
              style={{ background: 'var(--panel-border)', border: '1px solid var(--panel-border)' }}
            >
              {t('cancel', language) || 'Cancel'}
            </button>
            {!isSystemTemplate && (
              <button
                onClick={handleSave}
                disabled={isSaving || !name.trim() || !content.trim()}
                className="px-8 py-3 bg-gradient-to-r from-[#00CC66] to-[#00AA55] rounded-lg hover:from-[#00AA55] hover:to-[#008844] transition-all duration-200 disabled:bg-[#848E9C] disabled:cursor-not-allowed font-medium shadow-lg flex items-center gap-2"
                style={{ color: 'var(--navy-primary)' }}
              >
                <Save className="w-4 h-4" />
                {isSaving
                  ? t('saving', language) || 'Saving...'
                  : t('save', language) || 'Save'}
              </button>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}

