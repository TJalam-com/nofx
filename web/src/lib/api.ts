import type {
  SystemStatus,
  AccountInfo,
  Position,
  DecisionRecord,
  Statistics,
  TraderInfo,
  TraderConfigData,
  AIModel,
  Exchange,
  CreateTraderRequest,
  UpdateModelConfigRequest,
  UpdateExchangeConfigRequest,
  CompetitionData,
  BacktestRunsResponse,
  BacktestStartConfig,
  BacktestStatusPayload,
  BacktestEquityPoint,
  BacktestTradeEvent,
  BacktestMetrics,
  BacktestRunMetadata,
  PromptTemplate,
  CreatePromptTemplateRequest,
  UpdatePromptTemplateRequest,
  RunningTrader,
  ReplicationStatus,
  TestSignalRequest,
  TestSignalResponse,
  UserFollowersResponse,
  TraderApplication,
  CreateTraderApplicationRequest,
  Strategy,
  CreateStrategyRequest,
  UpdateStrategyRequest,
  Article,
  CreateArticleRequest,
  UpdateArticleRequest,
  ArticlesResponse,
} from '../types'
import { CryptoService } from './crypto'
import { httpClient } from './httpClient'

const API_BASE = '/api'

// Helper function to get auth headers
function getAuthHeaders(): Record<string, string> {
  const token = localStorage.getItem('auth_token')
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
  }

  if (token) {
    headers['Authorization'] = `Bearer ${token}`
  }

  return headers
}

async function handleJSONResponse<T>(res: Response): Promise<T> {
  const text = await res.text()
  if (!res.ok) {
    let message = text || res.statusText
    try {
      const data = text ? JSON.parse(text) : null
      if (data && typeof data === 'object') {
        message = data.error || data.message || message
      }
    } catch {
      /* ignore JSON parse errors */
    }
    throw new Error(message || '请求失败')
  }
  if (!text) {
    return {} as T
  }
  return JSON.parse(text) as T
}

export const api = {
  // AI交易员管理接口
  async getTraders(): Promise<TraderInfo[]> {
    const result = await httpClient.get<TraderInfo[]>(`${API_BASE}/my-traders`)
    if (!result.success) throw new Error('获取trader列表失败')
    return result.data!
  },

  // 获取公开的交易员列表（无需认证）
  async getPublicTraders(): Promise<any[]> {
    const result = await httpClient.get<any[]>(`${API_BASE}/traders`)
    if (!result.success) throw new Error('获取公开trader列表失败')
    return result.data!
  },

  // 获取所有运行中的交易员（用于follower选择信号源）
  async getRunningTraders(): Promise<RunningTrader[]> {
    const result = await httpClient.get<RunningTrader[]>(
      `${API_BASE}/running-traders`
    )
    if (!result.success) throw new Error('获取运行中的交易员列表失败')
    return result.data!
  },

  async createTrader(request: CreateTraderRequest): Promise<TraderInfo> {
    const result = await httpClient.post<TraderInfo>(
      `${API_BASE}/traders`,
      request
    )
    if (!result.success) throw new Error('创建交易员失败')
    return result.data!
  },

  async deleteTrader(traderId: string): Promise<void> {
    const result = await httpClient.delete(`${API_BASE}/traders/${traderId}`)
    if (!result.success) throw new Error('删除交易员失败')
  },

  async startTrader(traderId: string): Promise<void> {
    const result = await httpClient.post(
      `${API_BASE}/traders/${traderId}/start`
    )
    if (!result.success) {
      throw new Error(result.message || '启动交易员失败')
    }
  },

  async stopTrader(traderId: string): Promise<void> {
    const result = await httpClient.post(`${API_BASE}/traders/${traderId}/stop`)
    if (!result.success) {
      throw new Error(result.message || '停止交易员失败')
    }
  },

  async updateTraderPrompt(
    traderId: string,
    customPrompt: string
  ): Promise<void> {
    const result = await httpClient.put(
      `${API_BASE}/traders/${traderId}/prompt`,
      { custom_prompt: customPrompt }
    )
    if (!result.success) throw new Error('更新自定义策略失败')
  },

  async toggleCompetition(
    traderId: string,
    showInCompetition: boolean
  ): Promise<void> {
    const result = await httpClient.put(
      `${API_BASE}/traders/${traderId}/competition`,
      { show_in_competition: showInCompetition }
    )
    if (!result.success) throw new Error('Failed to update competition visibility')
  },

  async getTraderConfig(traderId: string): Promise<TraderConfigData> {
    const result = await httpClient.get<TraderConfigData>(
      `${API_BASE}/traders/${traderId}/config`
    )
    if (!result.success) throw new Error('获取交易员配置失败')
    return result.data!
  },

  async updateTrader(
    traderId: string,
    request: CreateTraderRequest
  ): Promise<TraderInfo> {
    const result = await httpClient.put<TraderInfo>(
      `${API_BASE}/traders/${traderId}`,
      request
    )
    if (!result.success) throw new Error('更新交易员失败')
    return result.data!
  },

  // AI模型配置接口
  async getModelConfigs(): Promise<AIModel[]> {
    const result = await httpClient.get<AIModel[]>(`${API_BASE}/models`)
    if (!result.success) throw new Error('获取模型配置失败')
    return result.data!
  },

  // 获取系统支持的AI模型列表（无需认证）
  async getSupportedModels(): Promise<AIModel[]> {
    const result = await httpClient.get<AIModel[]>(
      `${API_BASE}/supported-models`
    )
    if (!result.success) throw new Error('获取支持的模型失败')
    return result.data!
  },

  async getPromptTemplates(): Promise<string[]> {
    const res = await fetch(`${API_BASE}/prompt-templates`)
    if (!res.ok) throw new Error('获取提示词模板失败')
    const data = await res.json()
    if (Array.isArray(data.templates)) {
      return data.templates.map((item: { name: string }) => item.name)
    }
    return []
  },

  // 提示词模板管理接口（需要认证）
  async getUserPromptTemplates(): Promise<PromptTemplate[]> {
    const result = await httpClient.get<{ templates: PromptTemplate[] }>(
      `${API_BASE}/user/prompt-templates`
    )
    if (!result.success) throw new Error('获取提示词模板列表失败')
    return result.data!.templates
  },

  async getPromptTemplate(id: string): Promise<PromptTemplate> {
    const result = await httpClient.get<PromptTemplate>(
      `${API_BASE}/user/prompt-templates/${id}`
    )
    if (!result.success) throw new Error('获取提示词模板失败')
    return result.data!
  },

  async createPromptTemplate(
    request: CreatePromptTemplateRequest
  ): Promise<PromptTemplate> {
    const result = await httpClient.post<PromptTemplate>(
      `${API_BASE}/user/prompt-templates`,
      request
    )
    if (!result.success) throw new Error('创建提示词模板失败')
    return result.data!
  },

  async updatePromptTemplate(
    id: string,
    request: UpdatePromptTemplateRequest
  ): Promise<PromptTemplate> {
    const result = await httpClient.put<PromptTemplate>(
      `${API_BASE}/user/prompt-templates/${id}`,
      request
    )
    if (!result.success) throw new Error('更新提示词模板失败')
    return result.data!
  },

  async deletePromptTemplate(id: string): Promise<void> {
    const result = await httpClient.delete(`${API_BASE}/user/prompt-templates/${id}`)
    if (!result.success) throw new Error('删除提示词模板失败')
  },

  // Get default strategy configuration based on language
  async getDefaultStrategyConfig(lang: string = 'en'): Promise<{
    prompt_template: string
    custom_prompt: string
  }> {
    const result = await httpClient.get<{
      prompt_template: string
      custom_prompt: string
    }>(`${API_BASE}/strategies/default-config?lang=${lang}`)
    if (!result.success) throw new Error('获取默认策略配置失败')
    return result.data!
  },

  // Get default URLs for data sources
  async getDefaultURLs(): Promise<{
    coin_pool_url: string
    oi_top_url: string
    quant_data_url: string
  }> {
    const result = await httpClient.get<{
      coin_pool_url: string
      oi_top_url: string
      quant_data_url: string
    }>(`${API_BASE}/config/default-urls`)
    if (!result.success) throw new Error('获取默认URL配置失败')
    return result.data!
  },

  // Strategy management
  async getStrategies(): Promise<Strategy[]> {
    const result = await httpClient.get<Strategy[]>(`${API_BASE}/strategies`)
    if (!result.success) throw new Error('获取策略列表失败')
    return result.data!
  },

  async getStrategy(id: string): Promise<Strategy> {
    const result = await httpClient.get<Strategy>(`${API_BASE}/strategies/${id}`)
    if (!result.success) throw new Error('获取策略失败')
    return result.data!
  },

  async createStrategy(data: CreateStrategyRequest): Promise<Strategy> {
    const result = await httpClient.post<Strategy>(`${API_BASE}/strategies`, data)
    if (!result.success) throw new Error('创建策略失败')
    return result.data!
  },

  async updateStrategy(id: string, data: UpdateStrategyRequest): Promise<Strategy> {
    const result = await httpClient.put<Strategy>(`${API_BASE}/strategies/${id}`, data)
    if (!result.success) throw new Error('更新策略失败')
    return result.data!
  },

  async deleteStrategy(id: string): Promise<void> {
    const result = await httpClient.delete(`${API_BASE}/strategies/${id}`)
    if (!result.success) throw new Error('删除策略失败')
  },

  async exportStrategy(id: string): Promise<Blob> {
    const res = await fetch(`${API_BASE}/strategies/${id}/export`, {
      headers: getAuthHeaders(),
    })
    if (!res.ok) throw new Error('导出策略失败')
    return await res.blob()
  },

  async importStrategy(strategyData: Record<string, any>): Promise<Strategy> {
    const result = await httpClient.post<Strategy>(`${API_BASE}/strategies/import`, {
      strategy_data: strategyData,
    })
    if (!result.success) throw new Error('导入策略失败')
    return result.data!
  },

  async getCryptoConfig(): Promise<{
    transport_encryption_enabled: boolean
    public_key_available: boolean
  }> {
    const result = await httpClient.get<{
      transport_encryption_enabled: boolean
      public_key_available: boolean
    }>(`${API_BASE}/crypto/config`)
    if (!result.success) throw new Error('Failed to get crypto config')
    return result.data!
  },

  async updateModelConfigs(request: UpdateModelConfigRequest): Promise<void> {
    // Check if transport encryption is enabled
    // If config check fails, default to plain JSON (safer fallback)
    let transportEncryptionEnabled = false
    try {
      const cryptoConfig = await this.getCryptoConfig()
      transportEncryptionEnabled = cryptoConfig.transport_encryption_enabled
    } catch (error) {
      console.warn('Failed to get crypto config, defaulting to plain JSON:', error)
      transportEncryptionEnabled = false
    }

    if (transportEncryptionEnabled) {
      // Get RSA public key
      const publicKey = await CryptoService.fetchPublicKey()

      // Initialize encryption service
      await CryptoService.initialize(publicKey)

      // Get user information
      const userId = localStorage.getItem('user_id') || ''
      const sessionId = sessionStorage.getItem('session_id') || ''

      // Encrypt sensitive data
      const encryptedPayload = await CryptoService.encryptSensitiveData(
        JSON.stringify(request),
        userId,
        sessionId
      )

      // Send encrypted data
      const result = await httpClient.put(`${API_BASE}/models`, encryptedPayload)
      if (!result.success) throw new Error('Failed to update model configuration')
    } else {
      // Transport encryption disabled, send plain JSON
      const result = await httpClient.put(`${API_BASE}/models`, request)
      if (!result.success) throw new Error('Failed to update model configuration')
    }
  },

  // Exchange configuration endpoints
  async getExchangeConfigs(): Promise<Exchange[]> {
    const result = await httpClient.get<Exchange[]>(`${API_BASE}/exchanges`)
    if (!result.success) throw new Error('Failed to get exchange configuration')
    return result.data!
  },

  // Get system supported exchanges list (no authentication required)
  async getSupportedExchanges(): Promise<Exchange[]> {
    const result = await httpClient.get<Exchange[]>(
      `${API_BASE}/supported-exchanges`
    )
    if (!result.success) throw new Error('Failed to get supported exchanges')
    return result.data!
  },

  async updateExchangeConfigs(
    request: UpdateExchangeConfigRequest
  ): Promise<void> {
    const result = await httpClient.put(`${API_BASE}/exchanges`, request)
    if (!result.success) throw new Error('Failed to update exchange configuration')
  },

  // Update exchange configuration with encrypted transport (when TRANSPORT_ENCRYPTION=true)
  async updateExchangeConfigsEncrypted(
    request: UpdateExchangeConfigRequest
  ): Promise<void> {
    // Check if transport encryption is enabled
    // If config check fails, default to plain JSON (safer fallback)
    let transportEncryptionEnabled = false
    try {
      const cryptoConfig = await this.getCryptoConfig()
      transportEncryptionEnabled = cryptoConfig.transport_encryption_enabled
    } catch (error) {
      console.warn('Failed to get crypto config, defaulting to plain JSON:', error)
      transportEncryptionEnabled = false
    }

    if (transportEncryptionEnabled) {
      // Get RSA public key
      const publicKey = await CryptoService.fetchPublicKey()

      // Initialize encryption service
      await CryptoService.initialize(publicKey)

      // Get user information
      const userId = localStorage.getItem('user_id') || ''
      const sessionId = sessionStorage.getItem('session_id') || ''

      // Encrypt sensitive data
      const encryptedPayload = await CryptoService.encryptSensitiveData(
        JSON.stringify(request),
        userId,
        sessionId
      )

      // Send encrypted data
      const result = await httpClient.put(
        `${API_BASE}/exchanges`,
        encryptedPayload
      )
      if (!result.success) throw new Error('Failed to update exchange configuration')
    } else {
      // Transport encryption disabled, send plain JSON
      const result = await httpClient.put(`${API_BASE}/exchanges`, request)
      if (!result.success) throw new Error('Failed to update exchange configuration')
    }
  },

  // 获取系统状态（支持trader_id）
  async getStatus(traderId?: string): Promise<SystemStatus> {
    const url = traderId
      ? `${API_BASE}/status?trader_id=${traderId}`
      : `${API_BASE}/status`
    const result = await httpClient.get<SystemStatus>(url)
    if (!result.success) throw new Error('获取系统状态失败')
    return result.data!
  },

  // 获取账户信息（支持trader_id）
  async getAccount(traderId?: string): Promise<AccountInfo> {
    const url = traderId
      ? `${API_BASE}/account?trader_id=${traderId}`
      : `${API_BASE}/account`
    const result = await httpClient.get<AccountInfo>(url)
    if (!result.success) throw new Error('获取账户信息失败')
    console.log('Account data fetched:', result.data)
    return result.data!
  },

  // 获取持仓列表（支持trader_id）
  async getPositions(traderId?: string): Promise<Position[]> {
    const url = traderId
      ? `${API_BASE}/positions?trader_id=${traderId}`
      : `${API_BASE}/positions`
    const result = await httpClient.get<Position[]>(url)
    if (!result.success) throw new Error('获取持仓列表失败')
    // Ensure we always return an array, never null or undefined
    return Array.isArray(result.data) ? result.data : []
  },

  // 手动平仓
  async closePosition(
    traderId: string,
    symbol: string,
    side: 'long' | 'short',
    quantity: number = 0
  ): Promise<{ success: boolean; message: string; result: any }> {
    const result = await httpClient.post<{
      success: boolean
      message: string
      result: any
    }>(`${API_BASE}/positions/close?trader_id=${traderId}`, {
      symbol,
      side,
      quantity,
    })
    if (!result.success) {
      throw new Error(result.message || '平仓失败')
    }
    return result.data!
  },

  // 获取决策日志（支持trader_id）
  async getDecisions(traderId?: string): Promise<DecisionRecord[]> {
    const url = traderId
      ? `${API_BASE}/decisions?trader_id=${traderId}`
      : `${API_BASE}/decisions`
    const result = await httpClient.get<DecisionRecord[]>(url)
    if (!result.success) throw new Error('获取决策日志失败')
    return result.data!
  },

  // 获取最新决策（支持trader_id和limit参数）
  async getLatestDecisions(
    traderId?: string,
    limit: number = 5
  ): Promise<DecisionRecord[]> {
    const params = new URLSearchParams()
    if (traderId) {
      params.append('trader_id', traderId)
    }
    params.append('limit', limit.toString())

    const result = await httpClient.get<DecisionRecord[]>(
      `${API_BASE}/decisions/latest?${params}`
    )
    if (!result.success) throw new Error('获取最新决策失败')
    // Ensure we always return an array, never null or undefined
    return Array.isArray(result.data) ? result.data : []
  },

  // 获取统计信息（支持trader_id）
  async getStatistics(traderId?: string): Promise<Statistics> {
    const url = traderId
      ? `${API_BASE}/statistics?trader_id=${traderId}`
      : `${API_BASE}/statistics`
    const result = await httpClient.get<Statistics>(url)
    if (!result.success) throw new Error('获取统计信息失败')
    return result.data!
  },

  // 获取复制交易状态
  async getReplicationStatus(traderId: string): Promise<ReplicationStatus> {
    const result = await httpClient.get<ReplicationStatus>(
      `${API_BASE}/traders/${traderId}/replication-status`
    )
    if (!result.success) throw new Error('获取复制交易状态失败')
    return result.data!
  },

  // 获取用户所有交易员的跟随者列表及其活动
  async getUserFollowers(): Promise<UserFollowersResponse> {
    const result = await httpClient.get<UserFollowersResponse>(
      `${API_BASE}/user/followers`
    )
    if (!result.success) throw new Error('获取跟随者列表失败')
    return result.data!
  },

  // 发送测试信号
  async sendTestSignal(
    traderId: string,
    signal: TestSignalRequest
  ): Promise<TestSignalResponse> {
    const result = await httpClient.post<TestSignalResponse>(
      `${API_BASE}/traders/${traderId}/test-signal`,
      signal
    )
    if (!result.success) throw new Error('发送测试信号失败')
    return result.data!
  },

  // 获取收益率历史数据（支持trader_id）
  async getEquityHistory(traderId?: string): Promise<any[]> {
    const url = traderId
      ? `${API_BASE}/equity-history?trader_id=${traderId}`
      : `${API_BASE}/equity-history`
    const result = await httpClient.get<any[]>(url)
    if (!result.success) throw new Error('获取历史数据失败')
    return result.data!
  },

  // 批量获取多个交易员的历史数据（无需认证）
  async getEquityHistoryBatch(traderIds: string[]): Promise<any> {
    const result = await httpClient.post<any>(
      `${API_BASE}/equity-history-batch`,
      { trader_ids: traderIds }
    )
    if (!result.success) throw new Error('获取批量历史数据失败')
    return result.data!
  },

  // 获取前5名交易员数据（无需认证）
  async getTopTraders(): Promise<any[]> {
    const result = await httpClient.get<any[]>(`${API_BASE}/top-traders`)
    if (!result.success) throw new Error('获取前5名交易员失败')
    return result.data!
  },

  // 获取公开交易员配置（无需认证）
  async getPublicTraderConfig(traderId: string): Promise<any> {
    const result = await httpClient.get<any>(
      `${API_BASE}/traders/${traderId}/public-config`
    )
    if (!result.success) throw new Error('获取公开交易员配置失败')
    return result.data!
  },

  // 获取AI学习表现分析（支持trader_id）
  async getPerformance(traderId?: string): Promise<any> {
    const url = traderId
      ? `${API_BASE}/performance?trader_id=${traderId}`
      : `${API_BASE}/performance`
    const result = await httpClient.get<any>(url)
    if (!result.success) throw new Error('获取AI学习数据失败')
    return result.data!
  },

  // 获取竞赛数据（无需认证）
  async getCompetition(): Promise<CompetitionData> {
    const result = await httpClient.get<CompetitionData>(
      `${API_BASE}/competition`
    )
    if (!result.success) throw new Error('获取竞赛数据失败')
    return result.data!
  },

  // 用户信号源配置接口
  async getUserSignalSource(): Promise<{
    coin_pool_url: string
    oi_top_url: string
  }> {
    const result = await httpClient.get<{
      coin_pool_url: string
      oi_top_url: string
    }>(`${API_BASE}/user/signal-sources`)
    if (!result.success) throw new Error('获取用户信号源配置失败')
    return result.data!
  },

  async saveUserSignalSource(
    coinPoolUrl: string,
    oiTopUrl: string
  ): Promise<void> {
    const result = await httpClient.post(`${API_BASE}/user/signal-sources`, {
      coin_pool_url: coinPoolUrl,
      oi_top_url: oiTopUrl,
    })
    if (!result.success) throw new Error('保存用户信号源配置失败')
  },

  // Webhook管理
  async getWebhookInfo(): Promise<{ webhook_url: string; api_key: string }> {
    const result = await httpClient.get<{
      webhook_url: string
      api_key: string
    }>(`${API_BASE}/user/webhook`)
    if (!result.success) throw new Error('获取webhook信息失败')
    return result.data!
  },

  async testWebhook(payload: object): Promise<any> {
    const result = await httpClient.post(`${API_BASE}/webhook/tradingview`, payload)
    if (!result.success) throw new Error('测试webhook失败')
    return result.data
  },

  async getRecentAlerts(traderId?: string): Promise<any[]> {
    const url = traderId
      ? `${API_BASE}/user/tradingview-alerts?trader_id=${traderId}`
      : `${API_BASE}/user/tradingview-alerts`
    const result = await httpClient.get<{ alerts: any[] }>(url)
    if (!result.success) throw new Error('获取警报列表失败')
    return result.data!.alerts || []
  },

  // 获取服务器IP（需要认证，用于白名单配置）
  async getServerIP(): Promise<{
    public_ip: string
    message: string
  }> {
    const result = await httpClient.get<{
      public_ip: string
      message: string
    }>(`${API_BASE}/server-ip`)
    if (!result.success) throw new Error('获取服务器IP失败')
    return result.data!
  },

  // Backtest APIs
  async getBacktestRuns(params?: {
    state?: string
    search?: string
    limit?: number
    offset?: number
  }): Promise<BacktestRunsResponse> {
    const query = new URLSearchParams()
    if (params?.state) query.set('state', params.state)
    if (params?.search) query.set('search', params.search)
    if (params?.limit) query.set('limit', String(params.limit))
    if (params?.offset) query.set('offset', String(params.offset))
    const res = await fetch(
      `${API_BASE}/backtest/runs${query.toString() ? `?${query}` : ''}`,
      {
        headers: getAuthHeaders(),
      }
    )
    return handleJSONResponse<BacktestRunsResponse>(res)
  },

  async startBacktest(config: BacktestStartConfig): Promise<BacktestRunMetadata> {
    const res = await fetch(`${API_BASE}/backtest/start`, {
      method: 'POST',
      headers: getAuthHeaders(),
      body: JSON.stringify({ config }),
    })
    return handleJSONResponse<BacktestRunMetadata>(res)
  },

  async pauseBacktest(runId: string): Promise<BacktestRunMetadata> {
    const res = await fetch(`${API_BASE}/backtest/pause`, {
      method: 'POST',
      headers: getAuthHeaders(),
      body: JSON.stringify({ run_id: runId }),
    })
    return handleJSONResponse<BacktestRunMetadata>(res)
  },

  async resumeBacktest(runId: string): Promise<BacktestRunMetadata> {
    const res = await fetch(`${API_BASE}/backtest/resume`, {
      method: 'POST',
      headers: getAuthHeaders(),
      body: JSON.stringify({ run_id: runId }),
    })
    return handleJSONResponse<BacktestRunMetadata>(res)
  },

  async stopBacktest(runId: string): Promise<BacktestRunMetadata> {
    const res = await fetch(`${API_BASE}/backtest/stop`, {
      method: 'POST',
      headers: getAuthHeaders(),
      body: JSON.stringify({ run_id: runId }),
    })
    return handleJSONResponse<BacktestRunMetadata>(res)
  },

  async updateBacktestLabel(
    runId: string,
    label: string
  ): Promise<BacktestRunMetadata> {
    const res = await fetch(`${API_BASE}/backtest/label`, {
      method: 'POST',
      headers: getAuthHeaders(),
      body: JSON.stringify({ run_id: runId, label }),
    })
    return handleJSONResponse<BacktestRunMetadata>(res)
  },

  async deleteBacktestRun(runId: string): Promise<void> {
    const res = await fetch(`${API_BASE}/backtest/delete`, {
      method: 'POST',
      headers: getAuthHeaders(),
      body: JSON.stringify({ run_id: runId }),
    })
    if (!res.ok) {
      throw new Error(await res.text())
    }
  },

  async getBacktestStatus(runId: string): Promise<BacktestStatusPayload> {
    const res = await fetch(`${API_BASE}/backtest/status?run_id=${runId}`, {
      headers: getAuthHeaders(),
    })
    return handleJSONResponse<BacktestStatusPayload>(res)
  },

  async getBacktestEquity(
    runId: string,
    timeframe?: string,
    limit?: number
  ): Promise<BacktestEquityPoint[]> {
    const query = new URLSearchParams({ run_id: runId })
    if (timeframe) query.set('tf', timeframe)
    if (limit) query.set('limit', String(limit))
    const res = await fetch(`${API_BASE}/backtest/equity?${query}`, {
      headers: getAuthHeaders(),
    })
    return handleJSONResponse<BacktestEquityPoint[]>(res)
  },

  async getBacktestTrades(
    runId: string,
    limit = 200
  ): Promise<BacktestTradeEvent[]> {
    const query = new URLSearchParams({
      run_id: runId,
      limit: String(limit),
    })
    const res = await fetch(`${API_BASE}/backtest/trades?${query}`, {
      headers: getAuthHeaders(),
    })
    return handleJSONResponse<BacktestTradeEvent[]>(res)
  },

  async getBacktestMetrics(runId: string): Promise<BacktestMetrics> {
    const res = await fetch(`${API_BASE}/backtest/metrics?run_id=${runId}`, {
      headers: getAuthHeaders(),
    })
    return handleJSONResponse<BacktestMetrics>(res)
  },

  async getBacktestTrace(
    runId: string,
    cycle?: number
  ): Promise<DecisionRecord> {
    const query = new URLSearchParams({ run_id: runId })
    if (cycle) query.set('cycle', String(cycle))
    const res = await fetch(`${API_BASE}/backtest/trace?${query}`, {
      headers: getAuthHeaders(),
    })
    return handleJSONResponse<DecisionRecord>(res)
  },

  async getBacktestDecisions(
    runId: string,
    limit = 20,
    offset = 0
  ): Promise<DecisionRecord[]> {
    const query = new URLSearchParams({
      run_id: runId,
      limit: String(limit),
      offset: String(offset),
    })
    const res = await fetch(`${API_BASE}/backtest/decisions?${query}`, {
      headers: getAuthHeaders(),
    })
    return handleJSONResponse<DecisionRecord[]>(res)
  },

  async exportBacktest(runId: string): Promise<Blob> {
    const res = await fetch(`${API_BASE}/backtest/export?run_id=${runId}`, {
      headers: getAuthHeaders(),
    })
    if (!res.ok) {
      const text = await res.text()
      try {
        const data = text ? JSON.parse(text) : null
        throw new Error(
          data?.error || data?.message || text || '导出失败，请稍后再试'
        )
      } catch (err) {
        if (err instanceof Error && err.message) {
          throw err
        }
        throw new Error(text || '导出失败，请稍后再试')
      }
    }
    return res.blob()
  },

  // Admin APIs
  async getAllTraders(): Promise<any[]> {
    const result = await httpClient.get<any[]>(`${API_BASE}/admin/traders`)
    if (!result.success) {
      throw new Error(result.message || '获取所有交易员失败')
    }
    return result.data!
  },

  async getAllUsers(): Promise<any[]> {
    const result = await httpClient.get<any[]>(`${API_BASE}/admin/users`)
    if (!result.success) {
      throw new Error(result.message || '获取所有用户失败')
    }
    return result.data!
  },

  async updateUserRole(userId: string, role: string): Promise<void> {
    const result = await httpClient.put(`${API_BASE}/admin/users/${userId}/role`, {
      role,
    })
    if (!result.success) {
      throw new Error(result.message || '更新用户角色失败')
    }
  },

  // 获取当前用户信息（用于刷新角色等）
  async getCurrentUser(): Promise<{ id: string; email: string; role: string }> {
    const result = await httpClient.get<{ id: string; email: string; role: string }>(
      `${API_BASE}/user/me`
    )
    if (!result.success) {
      throw new Error(result.message || '获取用户信息失败')
    }
    return result.data!
  },

  // Trader Application APIs
  async createTraderApplication(
    request: CreateTraderApplicationRequest
  ): Promise<TraderApplication> {
    const result = await httpClient.post<TraderApplication>(
      `${API_BASE}/trader-application`,
      request
    )
    if (!result.success) {
      throw new Error(result.message || '提交申请失败')
    }
    return result.data!
  },

  async getMyTraderApplication(): Promise<TraderApplication | null> {
    const result = await httpClient.get<TraderApplication | null>(
      `${API_BASE}/trader-application/my`
    )
    if (!result.success) {
      throw new Error(result.message || '获取申请信息失败')
    }
    return result.data ?? null
  },

  // Admin Trader Application APIs
  async getAllTraderApplications(): Promise<TraderApplication[]> {
    const result = await httpClient.get<TraderApplication[]>(
      `${API_BASE}/admin/trader-applications`
    )
    if (!result.success) {
      throw new Error(result.message || '获取申请列表失败')
    }
    return result.data!
  },

  async approveTraderApplication(id: string, notes?: string): Promise<void> {
    const result = await httpClient.put(
      `${API_BASE}/admin/trader-applications/${id}/approve`,
      { admin_notes: notes || '' }
    )
    if (!result.success) {
      throw new Error(result.message || '批准申请失败')
    }
  },

  async rejectTraderApplication(id: string, notes: string): Promise<void> {
    const result = await httpClient.put(
      `${API_BASE}/admin/trader-applications/${id}/reject`,
      { admin_notes: notes }
    )
    if (!result.success) {
      throw new Error(result.message || '拒绝申请失败')
    }
  },

  // Article APIs (Admin)
  async getArticles(status?: string, limit?: number, offset?: number): Promise<Article[]> {
    const params = new URLSearchParams()
    if (status) params.append('status', status)
    if (limit) params.append('limit', limit.toString())
    if (offset) params.append('offset', offset.toString())
    const query = params.toString()
    const url = query ? `${API_BASE}/admin/articles?${query}` : `${API_BASE}/admin/articles`
    const result = await httpClient.get<ArticlesResponse>(url)
    if (!result.success) {
      throw new Error(result.message || '获取文章列表失败')
    }
    return result.data!.articles
  },

  async getArticle(id: string): Promise<Article> {
    const result = await httpClient.get<Article>(`${API_BASE}/admin/articles/${id}`)
    if (!result.success) {
      throw new Error(result.message || '获取文章失败')
    }
    return result.data!
  },

  async createArticle(article: CreateArticleRequest): Promise<Article> {
    const result = await httpClient.post<Article>(`${API_BASE}/admin/articles`, article)
    if (!result.success) {
      throw new Error(result.message || '创建文章失败')
    }
    return result.data!
  },

  async updateArticle(id: string, article: UpdateArticleRequest): Promise<Article> {
    const result = await httpClient.put<Article>(`${API_BASE}/admin/articles/${id}`, article)
    if (!result.success) {
      throw new Error(result.message || '更新文章失败')
    }
    return result.data!
  },

  async deleteArticle(id: string): Promise<void> {
    const result = await httpClient.delete(`${API_BASE}/admin/articles/${id}`)
    if (!result.success) {
      throw new Error(result.message || '删除文章失败')
    }
  },

  async publishArticle(id: string): Promise<Article> {
    const result = await httpClient.post<Article>(`${API_BASE}/admin/articles/${id}/publish`, {})
    if (!result.success) {
      throw new Error(result.message || '发布文章失败')
    }
    return result.data!
  },

  async unpublishArticle(id: string): Promise<Article> {
    const result = await httpClient.post<Article>(`${API_BASE}/admin/articles/${id}/unpublish`, {})
    if (!result.success) {
      throw new Error(result.message || '取消发布文章失败')
    }
    return result.data!
  },

  // Public Article APIs
  async getPublishedArticles(limit?: number, offset?: number): Promise<Article[]> {
    const params = new URLSearchParams()
    if (limit) params.append('limit', limit.toString())
    if (offset) params.append('offset', offset.toString())
    const query = params.toString()
    const url = query ? `${API_BASE}/articles?${query}` : `${API_BASE}/articles`
    const result = await httpClient.get<ArticlesResponse>(url)
    if (!result.success) {
      throw new Error(result.message || '获取文章列表失败')
    }
    return result.data!.articles
  },

  async getArticleBySlug(slug: string): Promise<Article> {
    const result = await httpClient.get<Article>(`${API_BASE}/articles/${slug}`)
    if (!result.success) {
      throw new Error(result.message || '获取文章失败')
    }
    return result.data!
  },
}
