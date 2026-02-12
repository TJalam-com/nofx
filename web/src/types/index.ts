// Page type for navigation
export type Page =
  | 'competition'
  | 'traders'
  | 'trader'
  | 'followers'
  | 'backtest'
  | 'webhook'
  | 'faq'
  | 'stats'
  | 'applications'
  | 'strategy-studio'
  | 'login'
  | 'register'

// 系统状态
export interface SystemStatus {
  is_running: boolean
  start_time: string
  runtime_minutes: number
  call_count: number
  initial_balance: number
  scan_interval: string
  stop_until: string
  last_reset_time: string
  ai_provider: string
}

// 账户信息
export interface AccountInfo {
  total_equity: number
  available_balance: number
  total_pnl: number
  total_pnl_pct: number
  total_unrealized_pnl: number
  margin_used: number
  margin_used_pct: number
  position_count: number
  initial_balance: number
  daily_pnl: number
}

// 持仓信息
export interface Position {
  symbol: string
  side: string
  entry_price: number
  mark_price: number
  quantity: number
  leverage: number
  unrealized_pnl: number
  unrealized_pnl_pct: number
  liquidation_price: number
  margin_used: number
}

// 决策动作
export interface DecisionAction {
  action: string
  symbol: string
  quantity: number
  leverage: number
  price: number
  order_id: number
  timestamp: string
  success: boolean
  error: string
  stop_loss?: number
  take_profit?: number
  confidence?: number
}

// 决策记录
export interface DecisionRecord {
  timestamp: string
  cycle_number: number
  input_prompt: string
  cot_trace: string
  decision_json: string
  account_state: {
    total_balance: number
    available_balance: number
    total_unrealized_profit: number
    position_count: number
    margin_used_pct: number
  }
  positions: Array<{
    symbol: string
    side: string
    position_amt: number
    entry_price: number
    mark_price: number
    unrealized_profit: number
    leverage: number
    liquidation_price: number
  }>
  candidate_coins: string[]
  decisions: DecisionAction[]
  execution_log: string[]
  success: boolean
  error_message: string
  raw_response?: string // Raw AI response for debugging parse failures
}

// 统计信息
export interface Statistics {
  total_cycles: number
  successful_cycles: number
  failed_cycles: number
  total_open_positions: number
  total_close_positions: number
}

// Streaming configuration types
export interface StreamingConfig {
  id: string
  admin_id: string
  is_streaming: boolean
  stream_mode: 'full' | 'minimal' | 'charts_only'
  show_decisions: boolean
  show_positions: boolean
  animation_speed: number // 0.5-2x
  theme: 'dark' | 'light' | 'cyberpunk'
  auto_switch_charts: boolean // Auto-switch between equity and market charts
  auto_switch_interval: number // Seconds between switches (5-30)
  auto_scroll_ai_analysis: boolean // Auto-scroll AI analysis text as it types
  auto_scroll_page: boolean // Auto-scroll the entire page
  auto_scroll_page_speed: number // 0.5x - 2x speed multiplier
  sound_enabled: boolean // Enable sound effects
  watermark_text: string
  widget_visibility: {
    equity: boolean
    positions: boolean
    decisions: boolean
    metrics: boolean
    ai_process: boolean
  }
  layout_type: 'dashboard' | 'chart_focus' | 'position_focus'
}

// WebSocket message types
export interface WebSocketMessage {
  type:
    | 'equity_update'
    | 'position_update'
    | 'decision_update'
    | 'account_update'
  data: any
  timestamp: string
}
