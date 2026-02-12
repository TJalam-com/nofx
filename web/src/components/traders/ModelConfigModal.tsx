import { useState, useEffect } from 'react'
import { Trash2, ExternalLink } from 'lucide-react'
import { t, type Language } from '../../i18n/translations'
import type { AIModel } from '../../types'
import { getModelIcon } from '../ModelIcons'
import { getShortName } from './utils'

// Get provider-specific API application links and hints
function getProviderHints(
  provider: string,
  language: Language
): { url: string; hint: string } | null {
  const providerLower = provider?.toLowerCase() || ''

  switch (providerLower) {
    case 'grok':
      return {
        url: 'https://console.x.ai',
        hint:
          t('providerHintGrok', language) ||
          'Get your API key from xAI Console',
      }
    case 'openai':
      return {
        url: 'https://platform.openai.com',
        hint:
          t('providerHintOpenAI', language) ||
          'Get your API key from OpenAI Platform',
      }
    case 'claude':
      return {
        url: 'https://console.anthropic.com',
        hint:
          t('providerHintClaude', language) ||
          'Get your API key from Anthropic Console',
      }
    case 'gemini':
      return {
        url: 'https://aistudio.google.com',
        hint:
          t('providerHintGemini', language) ||
          'Get your API key from Google AI Studio',
      }
    case 'kimi':
      return {
        url: 'https://platform.moonshot.cn',
        hint:
          t('providerHintKimi', language) ||
          'Get your API key from Moonshot Platform',
      }
    case 'deepseek':
      return {
        url: 'https://platform.deepseek.com',
        hint:
          t('providerHintDeepSeek', language) ||
          'Get your API key from DeepSeek Platform',
      }
    case 'qwen':
      return {
        url: 'https://dashscope.console.aliyun.com',
        hint:
          t('providerHintQwen', language) ||
          'Get your API key from Alibaba Cloud DashScope',
      }
    default:
      return null
  }
}

interface ModelConfigModalProps {
  allModels: AIModel[]
  configuredModels: AIModel[]
  editingModelId: string | null
  onSave: (
    modelId: string,
    apiKey: string,
    baseUrl?: string,
    modelName?: string
  ) => Promise<void>
  onDelete: (modelId: string) => void
  onClose: () => void
  language: Language
}

export function ModelConfigModal({
  allModels,
  configuredModels,
  editingModelId,
  onSave,
  onDelete,
  onClose,
  language,
}: ModelConfigModalProps) {
  const [selectedModelId, setSelectedModelId] = useState(editingModelId || '')
  const [apiKey, setApiKey] = useState('')
  const [baseUrl, setBaseUrl] = useState('')
  const [modelName, setModelName] = useState('')
  const [isLoading, setIsLoading] = useState(false)

  // Get current editing model information - when editing, find from configured models, when creating new, find from all supported models
  const selectedModel = editingModelId
    ? configuredModels?.find((m) => m.id === selectedModelId)
    : allModels?.find((m) => m.id === selectedModelId)

  // If editing existing model, initialize API Key, Base URL and Model Name
  useEffect(() => {
    if (editingModelId && selectedModel) {
      setApiKey(selectedModel.apiKey || '')
      setBaseUrl(selectedModel.customApiUrl || '')
      setModelName(selectedModel.customModelName || '')
    }
  }, [editingModelId, selectedModel])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!selectedModelId || !apiKey.trim() || isLoading) return

    setIsLoading(true)
    try {
      await onSave(
        selectedModelId,
        apiKey.trim(),
        baseUrl.trim() || undefined,
        modelName.trim() || undefined
      )
    } catch (error) {
      // Error handling is done in parent component
    } finally {
      setIsLoading(false)
    }
  }

  // Filter out prompt template names from AI models (they're not real AI models)
  const isPromptTemplateName = (name: string): boolean => {
    if (!name) return false
    const promptTemplateNames = [
      'risk_management',
      'risk-management',
      'riskmanagement',
      'default',
      'adaptive',
      'aggressive',
      'conservative',
      'scalping',
    ]
    const nameLower = name.toLowerCase().trim()
    return promptTemplateNames.some(
      (templateName) => nameLower === templateName.toLowerCase()
    )
  }

  // Available model list (all supported models, excluding prompt templates)
  const availableModels = (allModels || []).filter((model) => {
    // Exclude models that are actually prompt template names
    if (
      isPromptTemplateName(model.id) ||
      isPromptTemplateName(model.name || '')
    ) {
      return false
    }
    return true
  })

  return (
    <div
      className="fixed inset-0 flex items-center justify-center z-50 p-4 overflow-y-auto"
      style={{ background: 'rgba(0, 31, 63, 0.5)' }}
    >
      <div
        className="bg-gray-800 rounded-lg w-full max-w-lg relative my-8"
        style={{
          background: 'var(--navy-dark)',
          maxHeight: 'calc(100vh - 4rem)',
        }}
      >
        <div
          className="flex items-center justify-between p-6 pb-4 sticky top-0 z-10"
          style={{ background: 'var(--navy-dark)' }}
        >
          <h3 className="text-xl font-bold" style={{ color: '#EAECEF' }}>
            {editingModelId
              ? t('editAIModel', language)
              : t('addAIModel', language)}
          </h3>
          {editingModelId && (
            <button
              type="button"
              onClick={() => onDelete(editingModelId)}
              className="p-2 rounded hover:bg-red-100 transition-colors"
              style={{ background: 'rgba(246, 70, 93, 0.1)', color: '#F6465D' }}
              title={t('delete', language)}
            >
              <Trash2 className="w-4 h-4" />
            </button>
          )}
        </div>

        <form onSubmit={handleSubmit} className="px-6 pb-6">
          <div
            className="space-y-4 overflow-y-auto"
            style={{ maxHeight: 'calc(100vh - 16rem)' }}
          >
            {!editingModelId && (
              <div>
                <label
                  className="block text-sm font-semibold mb-2"
                  style={{ color: '#EAECEF' }}
                >
                  {t('selectModel', language)}
                </label>
                <select
                  value={selectedModelId}
                  onChange={(e) => setSelectedModelId(e.target.value)}
                  className="w-full px-3 py-2 rounded"
                  style={{
                    background: 'var(--navy-primary)',
                    border: '1px solid var(--panel-border)',
                    color: '#EAECEF',
                  }}
                  required
                >
                  <option value="">{t('pleaseSelectModel', language)}</option>
                  {availableModels.map((model) => (
                    <option key={model.id} value={model.id}>
                      {getShortName(model.name)} ({model.provider})
                    </option>
                  ))}
                </select>
              </div>
            )}

            {selectedModel && (
              <div
                className="p-4 rounded"
                style={{
                  background: 'var(--navy-primary)',
                  border: '1px solid var(--panel-border)',
                }}
              >
                <div className="flex items-center gap-3 mb-3">
                  <div className="w-8 h-8 flex items-center justify-center">
                    {getModelIcon(selectedModel.provider || selectedModel.id, {
                      width: 32,
                      height: 32,
                    }) || (
                      <div
                        className="w-8 h-8 rounded-full flex items-center justify-center text-sm font-bold"
                        style={{
                          background:
                            selectedModel.id === 'deepseek'
                              ? '#60a5fa'
                              : '#c084fc',
                          color: '#fff',
                        }}
                      >
                        {selectedModel.name[0]}
                      </div>
                    )}
                  </div>
                  <div>
                    <div className="font-semibold" style={{ color: '#EAECEF' }}>
                      {getShortName(selectedModel.name)}
                    </div>
                    <div className="text-xs" style={{ color: '#848E9C' }}>
                      {selectedModel.provider} • {selectedModel.id}
                    </div>
                  </div>
                </div>
              </div>
            )}

            {selectedModel && (
              <>
                <div>
                  <label
                    className="block text-sm font-semibold mb-2"
                    style={{ color: '#EAECEF' }}
                  >
                    API Key
                  </label>
                  <input
                    type="password"
                    value={apiKey}
                    onChange={(e) => setApiKey(e.target.value)}
                    placeholder={t('enterAPIKey', language)}
                    className="w-full px-3 py-2 rounded"
                    style={{
                      background: 'var(--navy-primary)',
                      border: '1px solid var(--panel-border)',
                      color: '#EAECEF',
                    }}
                    required
                  />
                </div>

                <div>
                  <label
                    className="block text-sm font-semibold mb-2"
                    style={{ color: '#EAECEF' }}
                  >
                    {t('customBaseURL', language)}
                  </label>
                  <input
                    type="url"
                    value={baseUrl}
                    onChange={(e) => setBaseUrl(e.target.value)}
                    placeholder={t('customBaseURLPlaceholder', language)}
                    className="w-full px-3 py-2 rounded"
                    style={{
                      background: 'var(--navy-primary)',
                      border: '1px solid var(--panel-border)',
                      color: '#EAECEF',
                    }}
                  />
                  <div className="text-xs mt-1" style={{ color: '#848E9C' }}>
                    {t('leaveBlankForDefault', language)}
                  </div>
                </div>

                <div>
                  <label
                    className="block text-sm font-semibold mb-2"
                    style={{ color: '#EAECEF' }}
                  >
                    {t('modelName', language) || 'Model Name'} (
                    {t('optional', language) || 'Optional'})
                  </label>
                  <input
                    type="text"
                    value={modelName}
                    onChange={(e) => setModelName(e.target.value)}
                    placeholder={
                      t('modelNamePlaceholder', language) ||
                      'e.g., deepseek-chat, qwen3-max, gpt-5'
                    }
                    className="w-full px-3 py-2 rounded"
                    style={{
                      background: 'var(--navy-primary)',
                      border: '1px solid var(--panel-border)',
                      color: '#EAECEF',
                    }}
                  />
                  <div className="text-xs mt-1" style={{ color: '#848E9C' }}>
                    {t('leaveBlankForDefaultModelName', language) ||
                      'Leave blank to use default model name'}
                  </div>
                </div>

                {selectedModel &&
                  getProviderHints(
                    selectedModel.provider || selectedModel.id,
                    language
                  ) && (
                    <div
                      className="p-4 rounded mb-4"
                      style={{
                        background: 'rgba(59, 130, 246, 0.1)',
                        border: '1px solid rgba(59, 130, 246, 0.2)',
                      }}
                    >
                      <div
                        className="text-sm font-semibold mb-2 flex items-center gap-2"
                        style={{ color: '#3B82F6' }}
                      >
                        🔗 {t('getAPIKey', language) || 'Get API Key'}
                      </div>
                      <div
                        className="text-xs space-y-2"
                        style={{ color: '#848E9C' }}
                      >
                        <div>
                          {
                            getProviderHints(
                              selectedModel.provider || selectedModel.id,
                              language
                            )?.hint
                          }
                        </div>
                        <a
                          href={
                            getProviderHints(
                              selectedModel.provider || selectedModel.id,
                              language
                            )?.url
                          }
                          target="_blank"
                          rel="noopener noreferrer"
                          className="inline-flex items-center gap-1 text-blue-400 hover:text-blue-300 transition-colors"
                        >
                          {
                            getProviderHints(
                              selectedModel.provider || selectedModel.id,
                              language
                            )?.url
                          }
                          <ExternalLink className="w-3 h-3" />
                        </a>
                      </div>
                    </div>
                  )}

                <div
                  className="p-4 rounded"
                  style={{
                    background: 'rgba(0, 255, 127, 0.1)',
                    border: '1px solid rgba(0, 255, 127, 0.2)',
                  }}
                >
                  <div
                    className="text-sm font-semibold mb-2"
                    style={{ color: 'var(--green-primary)' }}
                  >
                    ℹ️ {t('information', language)}
                  </div>
                  <div
                    className="text-xs space-y-1"
                    style={{ color: '#848E9C' }}
                  >
                    <div>{t('modelConfigInfo1', language)}</div>
                    <div>{t('modelConfigInfo2', language)}</div>
                    <div>{t('modelConfigInfo3', language)}</div>
                  </div>
                </div>
              </>
            )}
          </div>

          <div
            className="flex gap-3 mt-6 pt-4 sticky bottom-0"
            style={{ background: 'var(--navy-dark)' }}
          >
            <button
              type="button"
              onClick={onClose}
              className="flex-1 px-4 py-2 rounded text-sm font-semibold"
              style={{ background: 'var(--navy-light)', color: '#848E9C' }}
            >
              {t('cancel', language)}
            </button>
            <button
              type="submit"
              disabled={!selectedModel || !apiKey.trim() || isLoading}
              className="flex-1 px-4 py-2 rounded text-sm font-semibold disabled:opacity-50"
              style={{
                background: 'var(--green-primary)',
                color: 'var(--navy-primary)',
              }}
            >
              {isLoading
                ? t('saving', language) || 'Saving...'
                : t('saveConfig', language)}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
