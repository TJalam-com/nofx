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
  | 'articles'
  | 'strategy-studio'
  | 'login'
  | 'register'

export interface SystemStatus {
  trader_id: string
  trader_name: string
  ai_model: string
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

export interface AccountInfo {
  total_equity: number
  wallet_balance: number
  unrealized_profit: number // 未实现盈亏（交易所API官方值）
  available_balance: number
  total_pnl: number
  total_pnl_pct: number
  initial_balance: number
  daily_pnl: number
  position_count: number
  margin_used: number
  margin_used_pct: number
}

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

export interface PendingOrder {
  id: string
  trader_id: string
  symbol: string
  side: string
  order_type: string // "stop_loss", "take_profit", "limit"
  trigger_price: number
  quantity: number
  filled_quantity: number
  status: string // "pending", "partially_filled", "cancelled"
  parent_position_id: string
  exchange_order_id: string
  created_at: string
}

export interface DecisionAction {
  action: string
  symbol: string
  quantity: number
  leverage: number
  price: number
  order_id: number
  timestamp: string
  success: boolean
  error?: string
  reasoning?: string
  parent_signal_id?: string
  signal_decision?: string // "accept", "reject", "modify"
  stop_loss?: number
  take_profit?: number
  confidence?: number
}

export interface AccountSnapshot {
  total_balance: number
  available_balance: number
  total_unrealized_profit: number
  position_count: number
  margin_used_pct: number
}

export interface DecisionRecord {
  timestamp: string
  cycle_number: number
  input_prompt: string
  cot_trace: string
  decision_json: string
  account_state: AccountSnapshot
  positions: any[]
  candidate_coins: string[]
  decisions: DecisionAction[]
  execution_log: string[]
  success: boolean
  error_message?: string
  raw_response?: string // Raw AI response for debugging parse failures
}

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

// AI Trading 24x7相关类型
export interface TraderInfo {
  trader_id: string
  trader_name: string
  ai_model: string
  exchange_id?: string
  is_running?: boolean
  show_in_competition?: boolean
  custom_prompt?: string
  use_coin_pool?: boolean
  use_oi_top?: boolean
  use_tradingview?: boolean
  system_prompt_template?: string
  followed_trader_id?: string // 跟随的交易员ID（用于follower角色）
  strategy_id?: string // Strategy ID associated with this trader
}

// Running Trader interface for follower signal source selection
export interface RunningTrader {
  trader_id: string
  trader_name: string
  user_id: string
  user_email: string
}

export interface AIModel {
  id: string
  name: string
  provider: string
  enabled: boolean
  apiKey?: string
  customApiUrl?: string
  customModelName?: string
}

export interface Exchange {
  id: string
  name: string
  type: 'cex' | 'dex'
  enabled: boolean
  apiKey?: string
  secretKey?: string
  testnet?: boolean
  // Hyperliquid 特定字段
  hyperliquidWalletAddr?: string
  // Aster 特定字段
  asterUser?: string
  asterSigner?: string
  asterPrivateKey?: string
  // OKX specific fields
  okxPassphrase?: string
  // Lighter specific fields
  lighterWalletAddr?: string
  lighterAPIKeyPrivateKey?: string
  lighterAPIKeyIndex?: number
}

export interface CreateTraderRequest {
  name: string
  ai_model_id: string
  exchange_id: string
  strategy_id?: string // Strategy ID (new version)
  initial_balance?: number // Optional: auto-fetched on create, manually updatable on edit
  scan_interval_minutes?: number
  btc_eth_leverage?: number
  altcoin_leverage?: number
  trading_symbols?: string
  custom_prompt?: string
  override_base_prompt?: boolean
  system_prompt_template?: string
  is_cross_margin?: boolean
  use_coin_pool?: boolean
  use_oi_top?: boolean
  use_tradingview?: boolean
  followed_trader_id?: string // Followed trader ID (for follower role)
  // Indicator configuration
  enable_raw_klines?: boolean // Raw OHLCV klines (always true, required)
  enable_ema?: boolean // Enable EMA indicator
  enable_macd?: boolean // Enable MACD indicator
  enable_rsi?: boolean // Enable RSI indicator
  enable_atr?: boolean // Enable ATR indicator
  enable_volume?: boolean // Enable volume data
  enable_oi?: boolean // Enable open interest data
  enable_funding?: boolean // Enable funding rate data
  indicator_timeframe?: string // Timeframe for indicators (e.g., "3m", "15m", "1h", "4h")
  quant_data_url?: string // External quant data API URL with {symbol} placeholder
}

export interface UpdateModelConfigRequest {
  models: {
    [key: string]: {
      enabled: boolean
      api_key: string
      custom_api_url?: string
      custom_model_name?: string
    }
  }
}

export interface UpdateExchangeConfigRequest {
  exchanges: {
    [key: string]: {
      enabled: boolean
      api_key: string
      secret_key: string
      testnet?: boolean
      // Hyperliquid 特定字段
      hyperliquid_wallet_addr?: string
      // Aster 特定字段
      aster_user?: string
      aster_signer?: string
      aster_private_key?: string
      okx_passphrase?: string
      // Lighter 特定字段
      lighter_wallet_addr?: string
      lighter_api_key_private_key?: string
      lighter_api_key_index?: number
    }
  }
}

// Competition related types
export interface CompetitionTraderData {
  trader_id: string
  trader_name: string
  ai_model: string
  exchange: string
  total_equity: number
  total_pnl: number
  total_pnl_pct: number
  position_count: number
  margin_used_pct: number
  is_running: boolean
  followed_trader_id?: string // 跟随的交易员ID（用于follower角色）
  followers_count?: number // Number of followers for this trader
}

export interface CompetitionData {
  traders: CompetitionTraderData[]
  count: number
  follower_total_count?: number // Total count of all follower traders
  follower_running_count?: number // Count of running follower traders
}

// Trader Configuration Data for View Modal
export interface TraderConfigData {
  trader_id?: string
  trader_name: string
  ai_model: string
  exchange_id: string
  strategy_id?: string // Strategy ID (new version)
  btc_eth_leverage: number
  altcoin_leverage: number
  trading_symbols: string
  custom_prompt: string
  override_base_prompt: boolean
  system_prompt_template: string
  is_cross_margin: boolean
  use_coin_pool: boolean
  use_oi_top: boolean
  use_tradingview: boolean
  followed_trader_id?: string // Followed trader ID (for follower role)
  initial_balance: number
  scan_interval_minutes: number
  is_running: boolean
  // Indicator configuration
  enable_raw_klines?: boolean // Raw OHLCV klines (always true, required)
  enable_ema?: boolean // Enable EMA indicator
  enable_macd?: boolean // Enable MACD indicator
  enable_rsi?: boolean // Enable RSI indicator
  enable_atr?: boolean // Enable ATR indicator
  enable_volume?: boolean // Enable volume data
  enable_oi?: boolean // Enable open interest data
  enable_funding?: boolean // Enable funding rate data
  indicator_timeframe?: string // Timeframe for indicators (e.g., "3m", "15m", "1h", "4h")
  quant_data_url?: string // External quant data API URL with {symbol} placeholder
}

// Backtest types
export interface BacktestRunSummary {
  symbol_count: number;
  decision_tf: string;
  processed_bars: number;
  progress_pct: number;
  equity_last: number;
  max_drawdown_pct: number;
  liquidated: boolean;
  liquidation_note?: string;
}

export interface BacktestRunMetadata {
  run_id: string;
  label?: string;
  user_id?: string;
  last_error?: string;
  version: number;
  state: string;
  created_at: string;
  updated_at: string;
  summary: BacktestRunSummary;
}

export interface BacktestRunsResponse {
  total: number;
  items: BacktestRunMetadata[];
}

// Replication Status Types
export interface ReplicationStatus {
  trader_id: string
  trader_name: string
  is_child: boolean
  is_parent: boolean
  parent?: {
    trader_id: string
    trader_name: string
    is_running: boolean
    error?: string
  } | null
  followers?: Array<{
    trader_id: string
    trader_name: string
    is_running: boolean
    user_id: string
    error?: string
  }>
  error?: string
}

export interface TestSignalRequest {
  symbol: string
  action: string
  leverage?: number
  position_size_usd?: number
  stop_loss?: number
  take_profit?: number
  reasoning?: string
}

export interface TestSignalResponse {
  message: string
  signal: {
    symbol: string
    action: string
    leverage?: number
    position_size_usd?: number
    stop_loss?: number
    take_profit?: number
    reasoning?: string
  }
  followers_count: number
}

// User Followers Types
export interface FollowerWithActivities {
  trader_id: string
  trader_name: string
  user_id: string
  user_email?: string
  user_name?: string
  is_running: boolean
  account: AccountInfo | { error?: string }
  latest_decisions: Array<{
    timestamp: string
    cycle_number: number
    success: boolean
    error_message?: string
    decisions: DecisionRecord['decisions']
  }>
  positions: Position[]
  error?: string
}

export interface ParentTraderWithFollowers {
  trader_id: string
  trader_name: string
  followers: FollowerWithActivities[]
}

export interface SocialLinks {
  twitter?: string
  telegram?: string
  discord?: string
  website?: string
  [key: string]: string | undefined
}

export interface TraderApplication {
  id: string
  user_id: string
  user_email?: string
  name: string
  email: string
  description: string
  trading_experience: string
  strategy_overview: string
  social_links?: SocialLinks
  status: 'pending' | 'approved' | 'rejected'
  admin_notes?: string
  created_at: string
  updated_at: string
}

export interface CreateTraderApplicationRequest {
  name: string
  email: string
  description: string
  trading_experience: string
  strategy_overview: string
  social_links?: SocialLinks
}

export interface UserFollowersResponse {
  parent_traders: ParentTraderWithFollowers[]
}

export interface BacktestStatusPayload {
  run_id: string;
  state: string;
  progress_pct: number;
  processed_bars: number;
  current_time: number;
  decision_cycle: number;
  equity: number;
  unrealized_pnl: number;
  realized_pnl: number;
  note?: string;
  last_error?: string;
  last_updated_iso: string;
}

export interface BacktestEquityPoint {
  ts: number;
  equity: number;
  available: number;
  pnl: number;
  pnl_pct: number;
  dd_pct: number;
  cycle: number;
}

export interface BacktestTradeEvent {
  ts: number;
  symbol: string;
  action: string;
  side?: string;
  qty: number;
  price: number;
  fee: number;
  slippage: number;
  order_value: number;
  realized_pnl: number;
  leverage?: number;
  cycle: number;
  position_after: number;
  liquidation: boolean;
  note?: string;
}

export interface BacktestMetrics {
  total_return_pct: number;
  max_drawdown_pct: number;
  sharpe_ratio: number;
  profit_factor: number;
  win_rate: number;
  trades: number;
  avg_win: number;
  avg_loss: number;
  best_symbol: string;
  worst_symbol: string;
  liquidated: boolean;
  symbol_stats?: Record<
    string,
    {
      total_trades: number;
      winning_trades: number;
      losing_trades: number;
      total_pnl: number;
      avg_pnl: number;
      win_rate: number;
    }
  >;
}

export interface BacktestStartConfig {
  run_id?: string;
  ai_model_id?: string;
  symbols: string[];
  timeframes: string[];
  decision_timeframe: string;
  decision_cadence_nbars: number;
  start_ts: number;
  end_ts: number;
  initial_balance: number;
  fee_bps: number;
  slippage_bps: number;
  fill_policy: string;
  prompt_variant?: string;
  prompt_template?: string;
  custom_prompt?: string;
  override_prompt?: boolean;
  cache_ai?: boolean;
  replay_only?: boolean;
  checkpoint_interval_bars?: number;
  checkpoint_interval_seconds?: number;
  replay_decision_dir?: string;
  shared_ai_cache_path?: string;
  ai?: {
    provider?: string;
    model?: string;
    key?: string;
    secret_key?: string;
    base_url?: string;
  };
  leverage?: {
    btc_eth_leverage?: number;
    altcoin_leverage?: number;
  };
}

// Prompt Template types
export interface PromptTemplate {
  id: string;
  name: string;
  content: string;
  is_system: boolean;
  created_at: string;
  updated_at: string;
}

export interface CreatePromptTemplateRequest {
  name: string;
  content: string;
}

export interface UpdatePromptTemplateRequest {
  name?: string;
  content?: string;
}

// Strategy types
export interface Strategy {
  id: string
  user_id: string
  name: string
  description: string
  system_prompt_template: string
  custom_prompt: string
  override_base_prompt: boolean
  btc_eth_leverage: number
  altcoin_leverage: number
  trading_symbols: string
  is_cross_margin: boolean
  use_coin_pool: boolean
  use_oi_top: boolean
  use_tradingview: boolean
  enable_raw_klines: boolean
  enable_ema: boolean
  enable_macd: boolean
  enable_rsi: boolean
  enable_atr: boolean
  enable_volume: boolean
  enable_oi: boolean
  enable_funding: boolean
  indicator_timeframe: string
  quant_data_url: string
  // Risk Management Configuration
  min_risk_reward_ratio?: number
  max_positions?: number
  margin_usage_limit?: number
  min_opening_amount?: number
  min_opening_amount_btc_eth?: number
  // Position Sizing Configuration
  altcoin_position_min?: number
  altcoin_position_max?: number
  btc_eth_position_min?: number
  btc_eth_position_max?: number
  available_margin_multiplier?: number
  // Trading Rules Configuration
  min_confidence_for_entry?: number
  min_holding_time_minutes?: number
  // Sharpe Ratio Configuration (JSON string)
  sharpe_ratio_config?: string
  created_at: string
  updated_at: string
}

export interface CreateStrategyRequest {
  name: string
  description?: string
  system_prompt_template?: string
  custom_prompt?: string
  override_base_prompt?: boolean
  btc_eth_leverage?: number
  altcoin_leverage?: number
  trading_symbols?: string
  is_cross_margin?: boolean
  use_coin_pool?: boolean
  use_oi_top?: boolean
  use_tradingview?: boolean
  enable_raw_klines?: boolean
  enable_ema?: boolean
  enable_macd?: boolean
  enable_rsi?: boolean
  enable_atr?: boolean
  enable_volume?: boolean
  enable_oi?: boolean
  enable_funding?: boolean
  indicator_timeframe?: string
  quant_data_url?: string
  // Risk Management Configuration
  min_risk_reward_ratio?: number
  max_positions?: number
  margin_usage_limit?: number
  min_opening_amount?: number
  min_opening_amount_btc_eth?: number
  // Position Sizing Configuration
  altcoin_position_min?: number
  altcoin_position_max?: number
  btc_eth_position_min?: number
  btc_eth_position_max?: number
  available_margin_multiplier?: number
  // Trading Rules Configuration
  min_confidence_for_entry?: number
  min_holding_time_minutes?: number
  // Sharpe Ratio Configuration (JSON string)
  sharpe_ratio_config?: string
}

export interface UpdateStrategyRequest {
  name: string
  description?: string
  system_prompt_template?: string
  custom_prompt?: string
  override_base_prompt?: boolean
  btc_eth_leverage?: number
  altcoin_leverage?: number
  trading_symbols?: string
  is_cross_margin?: boolean
  use_coin_pool?: boolean
  use_oi_top?: boolean
  use_tradingview?: boolean
  enable_raw_klines?: boolean
  enable_ema?: boolean
  enable_macd?: boolean
  enable_rsi?: boolean
  enable_atr?: boolean
  enable_volume?: boolean
  enable_oi?: boolean
  enable_funding?: boolean
  indicator_timeframe?: string
  quant_data_url?: string
  // Risk Management Configuration
  min_risk_reward_ratio?: number
  max_positions?: number
  margin_usage_limit?: number
  min_opening_amount?: number
  min_opening_amount_btc_eth?: number
  // Position Sizing Configuration
  altcoin_position_min?: number
  altcoin_position_max?: number
  btc_eth_position_min?: number
  btc_eth_position_max?: number
  available_margin_multiplier?: number
  // Trading Rules Configuration
  min_confidence_for_entry?: number
  min_holding_time_minutes?: number
  // Sharpe Ratio Configuration (JSON string)
  sharpe_ratio_config?: string
}

// Article types
export interface Article {
  id: string
  slug: string
  title: string
  content: string
  excerpt: string
  featured_image_url: string
  author_id: string
  author_email?: string
  status: 'draft' | 'published'
  meta_title?: string
  meta_description: string
  meta_keywords?: string
  og_image_url?: string
  published_at?: string
  created_at: string
  updated_at: string
}

export interface CreateArticleRequest {
  title: string
  content: string
  excerpt?: string
  featured_image_url?: string
  status?: 'draft' | 'published'
  meta_title?: string
  meta_description?: string
  meta_keywords?: string
  og_image_url?: string
  slug?: string
}

export interface UpdateArticleRequest {
  title?: string
  content?: string
  excerpt?: string
  featured_image_url?: string
  status?: 'draft' | 'published'
  meta_title?: string
  meta_description?: string
  meta_keywords?: string
  og_image_url?: string
  slug?: string
}

export interface ArticlesResponse {
  articles: Article[]
}