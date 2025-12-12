import { useState } from 'react'
import { toast } from 'sonner'
import { Plus, Edit, Trash2, Download, Upload, Copy } from 'lucide-react'
import useSWR from 'swr'
import { api } from '../lib/api'
import { useLanguage } from '../contexts/LanguageContext'
import type { Strategy, CreateStrategyRequest } from '../types'
import { StrategyEditorModal } from '../components/StrategyEditorModal'

export function StrategyStudioPage() {
  const { language } = useLanguage()
  const [showCreateModal, setShowCreateModal] = useState(false)
  const [showEditModal, setShowEditModal] = useState(false)
  const [editingStrategy, setEditingStrategy] = useState<Strategy | null>(null)
  const [showImportModal, setShowImportModal] = useState(false)

  // Format date for display
  const formatDate = (dateString: string | undefined | null) => {
    if (!dateString || dateString.trim() === '') {
      return language === 'zh' ? '未知日期' : 'Unknown date'
    }
    
    try {
      // Handle SQLite datetime format "2006-01-02 15:04:05" or ISO format
      let date: Date
      
      // Try parsing SQLite format first (YYYY-MM-DD HH:MM:SS)
      const sqliteMatch = dateString.match(/^(\d{4})-(\d{2})-(\d{2})(?:\s+(\d{2}):(\d{2}):(\d{2}))?/)
      if (sqliteMatch) {
        const [, year, month, day, hour = '0', minute = '0', second = '0'] = sqliteMatch
        date = new Date(parseInt(year), parseInt(month) - 1, parseInt(day), parseInt(hour), parseInt(minute), parseInt(second))
      } else {
        // Try standard Date parsing
        date = new Date(dateString)
      }
      
      // Check if date is invalid or is the zero date (0001-01-01)
      if (isNaN(date.getTime()) || date.getFullYear() < 1900) {
        console.warn('Invalid date string:', dateString)
        return language === 'zh' ? '无效日期' : 'Invalid date'
      }
      
      return date.toLocaleDateString(language === 'zh' ? 'zh-CN' : 'en-US', {
        year: 'numeric',
        month: 'numeric',
        day: 'numeric',
      })
    } catch (error) {
      console.error('Date parse error:', error, 'for string:', dateString)
      return language === 'zh' ? '日期解析错误' : 'Date parse error'
    }
  }

  const { data: strategies, mutate: mutateStrategies } = useSWR(
    'strategies',
    api.getStrategies,
    { refreshInterval: 30000 }
  )

  const handleCreate = () => {
    setEditingStrategy(null)
    setShowCreateModal(true)
  }

  const handleEdit = (strategy: Strategy) => {
    setEditingStrategy(strategy)
    setShowEditModal(true)
  }

  const handleDelete = async (strategy: Strategy) => {
    if (!confirm(language === 'zh' 
      ? `确定要删除策略 "${strategy.name}" 吗？` 
      : `Are you sure you want to delete strategy "${strategy.name}"?`)) {
      return
    }

    try {
      await api.deleteStrategy(strategy.id)
      toast.success(language === 'zh' ? '策略删除成功' : 'Strategy deleted successfully')
      mutateStrategies()
    } catch (error: any) {
      toast.error(error.message || (language === 'zh' ? '删除策略失败' : 'Failed to delete strategy'))
    }
  }

  const handleExport = async (strategy: Strategy) => {
    try {
      const blob = await api.exportStrategy(strategy.id)
      const url = window.URL.createObjectURL(blob)
      const a = document.createElement('a')
      a.href = url
      a.download = `${strategy.name.replace(/\s+/g, '_')}_strategy.json`
      document.body.appendChild(a)
      a.click()
      document.body.removeChild(a)
      window.URL.revokeObjectURL(url)
      toast.success(language === 'zh' ? '策略导出成功' : 'Strategy exported successfully')
    } catch (error: any) {
      toast.error(error.message || (language === 'zh' ? '导出策略失败' : 'Failed to export strategy'))
    }
  }

  const handleImport = () => {
    setShowImportModal(true)
  }

  const handleImportFile = async (file: File) => {
    try {
      const text = await file.text()
      const strategyData = JSON.parse(text)
      await api.importStrategy(strategyData)
      toast.success(language === 'zh' ? '策略导入成功' : 'Strategy imported successfully')
      mutateStrategies()
      setShowImportModal(false)
    } catch (error: any) {
      toast.error(error.message || (language === 'zh' ? '导入策略失败' : 'Failed to import strategy'))
    }
  }

  const handleDuplicate = async (strategy: Strategy) => {
    try {
      const duplicateData: CreateStrategyRequest = {
        name: `${strategy.name} (Copy)`,
        description: strategy.description,
        system_prompt_template: strategy.system_prompt_template,
        custom_prompt: strategy.custom_prompt,
        override_base_prompt: strategy.override_base_prompt,
        btc_eth_leverage: strategy.btc_eth_leverage,
        altcoin_leverage: strategy.altcoin_leverage,
        trading_symbols: strategy.trading_symbols,
        is_cross_margin: strategy.is_cross_margin,
        use_coin_pool: strategy.use_coin_pool,
        use_oi_top: strategy.use_oi_top,
        use_tradingview: strategy.use_tradingview,
        enable_raw_klines: strategy.enable_raw_klines,
        enable_ema: strategy.enable_ema,
        enable_macd: strategy.enable_macd,
        enable_rsi: strategy.enable_rsi,
        enable_atr: strategy.enable_atr,
        enable_volume: strategy.enable_volume,
        enable_oi: strategy.enable_oi,
        enable_funding: strategy.enable_funding,
        indicator_timeframe: strategy.indicator_timeframe,
        quant_data_url: strategy.quant_data_url,
      }
      await api.createStrategy(duplicateData)
      toast.success(language === 'zh' ? '策略复制成功' : 'Strategy duplicated successfully')
      mutateStrategies()
    } catch (error: any) {
      toast.error(error.message || (language === 'zh' ? '复制策略失败' : 'Failed to duplicate strategy'))
    }
  }

  return (
    <div className="min-h-screen" style={{ background: 'var(--navy-background)' }}>
      <div className="container mx-auto px-4 py-8">
        {/* Header */}
        <div className="flex items-center justify-between mb-8">
          <div>
            <h1 className="text-3xl font-bold text-[#EAECEF] mb-2">
              {language === 'zh' ? '策略' : 'Strategy'}
            </h1>
            <p className="text-[#848E9C]">
              {language === 'zh' 
                ? '创建、管理和分享您的交易策略' 
                : 'Create, manage, and share your trading strategies'}
            </p>
          </div>
          <div className="flex gap-3">
            <button
              onClick={handleImport}
              className="px-4 py-2 rounded-lg flex items-center gap-2 transition-colors"
              style={{ 
                background: 'var(--navy-primary)', 
                border: '1px solid var(--panel-border)',
                color: '#EAECEF'
              }}
            >
              <Upload className="w-4 h-4" />
              {language === 'zh' ? '导入' : 'Import'}
            </button>
            <button
              onClick={handleCreate}
              className="px-4 py-2 rounded-lg flex items-center gap-2 transition-colors"
              style={{ 
                background: 'var(--green-primary)', 
                color: 'var(--navy-primary)'
              }}
            >
              <Plus className="w-4 h-4" />
              {language === 'zh' ? '创建策略' : 'Create Strategy'}
            </button>
          </div>
        </div>

        {/* Strategies Grid */}
        {!strategies || strategies.length === 0 ? (
          <div className="text-center py-16 rounded-lg" style={{ background: 'var(--navy-primary)', border: '1px solid var(--panel-border)' }}>
            <p className="text-[#848E9C] mb-4">
              {language === 'zh' ? '还没有策略，创建第一个吧！' : 'No strategies yet. Create your first one!'}
            </p>
            <button
              onClick={handleCreate}
              className="px-6 py-2 rounded-lg flex items-center gap-2 mx-auto transition-colors"
              style={{ 
                background: 'var(--green-primary)', 
                color: 'var(--navy-primary)'
              }}
            >
              <Plus className="w-4 h-4" />
              {language === 'zh' ? '创建策略' : 'Create Strategy'}
            </button>
          </div>
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            {strategies.map((strategy) => (
              <div
                key={strategy.id}
                className="rounded-lg p-5 transition-all hover:scale-[1.02]"
                style={{ background: 'var(--navy-dark)', border: '1px solid var(--panel-border)' }}
              >
                <div className="flex items-start justify-between mb-3">
                  <div className="flex-1">
                    <h3 className="text-lg font-semibold text-[#EAECEF] mb-1">
                      {strategy.name}
                    </h3>
                    {strategy.description && (
                      <p className="text-sm text-[#848E9C] line-clamp-2">
                        {strategy.description}
                      </p>
                    )}
                  </div>
                </div>

                <div className="flex flex-wrap gap-2 mb-4">
                  {strategy.system_prompt_template && (
                    <span className="text-xs px-2 py-1 rounded" style={{ background: 'var(--navy-background)', color: '#848E9C' }}>
                      {strategy.system_prompt_template}
                    </span>
                  )}
                  {strategy.use_coin_pool && (
                    <span className="text-xs px-2 py-1 rounded" style={{ background: 'var(--navy-background)', color: '#848E9C' }}>
                      Coin Pool
                    </span>
                  )}
                  {strategy.use_oi_top && (
                    <span className="text-xs px-2 py-1 rounded" style={{ background: 'var(--navy-background)', color: '#848E9C' }}>
                      OI Top
                    </span>
                  )}
                </div>

                <div className="text-xs text-[#848E9C] mb-4">
                  {language === 'zh' ? '更新于' : 'Updated'} {strategy.updated_at ? formatDate(strategy.updated_at) : (language === 'zh' ? '未知日期' : 'Unknown date')}
                </div>

                <div className="flex gap-2">
                  <button
                    onClick={() => handleEdit(strategy)}
                    className="flex-1 px-3 py-2 rounded text-sm flex items-center justify-center gap-1 transition-colors"
                    style={{ 
                      background: 'var(--navy-background)', 
                      border: '1px solid var(--panel-border)',
                      color: '#EAECEF'
                    }}
                  >
                    <Edit className="w-3 h-3" />
                    {language === 'zh' ? '编辑' : 'Edit'}
                  </button>
                  <button
                    onClick={() => handleDuplicate(strategy)}
                    className="px-3 py-2 rounded text-sm flex items-center justify-center gap-1 transition-colors"
                    style={{ 
                      background: 'var(--navy-background)', 
                      border: '1px solid var(--panel-border)',
                      color: '#EAECEF'
                    }}
                    title={language === 'zh' ? '复制' : 'Duplicate'}
                  >
                    <Copy className="w-3 h-3" />
                  </button>
                  <button
                    onClick={() => handleExport(strategy)}
                    className="px-3 py-2 rounded text-sm flex items-center justify-center gap-1 transition-colors"
                    style={{ 
                      background: 'var(--navy-background)', 
                      border: '1px solid var(--panel-border)',
                      color: '#EAECEF'
                    }}
                    title={language === 'zh' ? '导出' : 'Export'}
                  >
                    <Download className="w-3 h-3" />
                  </button>
                  <button
                    onClick={() => handleDelete(strategy)}
                    className="px-3 py-2 rounded text-sm flex items-center justify-center gap-1 transition-colors hover:bg-red-500/20"
                    style={{ 
                      background: 'var(--navy-background)', 
                      border: '1px solid var(--panel-border)',
                      color: '#EAECEF'
                    }}
                    title={language === 'zh' ? '删除' : 'Delete'}
                  >
                    <Trash2 className="w-3 h-3" />
                  </button>
                </div>
              </div>
            ))}
          </div>
        )}

        {/* Create/Edit Modal */}
        {(showCreateModal || showEditModal) && (
          <StrategyEditorModal
            isOpen={showCreateModal || showEditModal}
            onClose={() => {
              setShowCreateModal(false)
              setShowEditModal(false)
              setEditingStrategy(null)
            }}
            strategy={editingStrategy}
            onSave={async () => {
              mutateStrategies()
              setShowCreateModal(false)
              setShowEditModal(false)
              setEditingStrategy(null)
            }}
          />
        )}

        {/* Import Modal */}
        {showImportModal && (
          <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
            <div className="rounded-lg p-6 max-w-md w-full mx-4" style={{ background: 'var(--navy-primary)', border: '1px solid var(--panel-border)' }}>
              <h2 className="text-xl font-semibold text-[#EAECEF] mb-4">
                {language === 'zh' ? '导入策略' : 'Import Strategy'}
              </h2>
              <input
                type="file"
                accept=".json"
                onChange={(e) => {
                  const file = e.target.files?.[0]
                  if (file) {
                    handleImportFile(file)
                  }
                }}
                className="w-full mb-4"
              />
              <div className="flex gap-3 justify-end">
                <button
                  onClick={() => setShowImportModal(false)}
                  className="px-4 py-2 rounded transition-colors"
                  style={{ 
                    background: 'var(--navy-background)', 
                    border: '1px solid var(--panel-border)',
                    color: '#EAECEF'
                  }}
                >
                  {language === 'zh' ? '取消' : 'Cancel'}
                </button>
              </div>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}

