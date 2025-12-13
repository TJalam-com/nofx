import { Language } from '../i18n/translations'

export interface SEOConfig {
  title: string
  description: string
  keywords?: string
  ogImage?: string
  ogType?: string
  noindex?: boolean
  canonical?: string
}

const BASE_URL = import.meta.env.VITE_BASE_URL || 'https://aitrading247.com'
// Prefer WebP for better performance, modern platforms support it
const DEFAULT_OG_IMAGE = `${BASE_URL}/images/main.webp`

export const defaultSEO: SEOConfig = {
  title: 'AI Trading 24x7 - Multi-AI Model Trading Platform',
  description:
    'AI Trading 24x7 platform with multiple AI models for automated cryptocurrency trading. Follow top traders, backtest strategies, and manage your crypto portfolio.',
  keywords: 'AI trading, cryptocurrency, automated trading, crypto trading, AI models, trading platform',
  ogImage: DEFAULT_OG_IMAGE,
  ogType: 'website',
}

export const seoConfig: Record<string, (lang: Language) => SEOConfig> = {
  '/': (lang: Language) => ({
    title: lang === 'en' 
      ? 'AI Trading 24x7 | Best AI Copy Trading Platform 2025'
      : 'AI Trading 24x7 | 最佳AI复制交易平台 2025',
    description: lang === 'en'
      ? 'AI Trading 24x7 - The #1 AI copy trading platform. Automate trades with AI decision engine, copy top traders, multi-exchange support. Free trial, open source (AGPL-3.0).'
      : 'AI Trading 24x7 - 排名第一的AI复制交易平台。使用AI决策引擎自动化交易，复制顶级交易员，支持多交易所。免费试用，开源（AGPL-3.0）。',
    keywords: lang === 'en'
      ? 'AI trading, AI copy trading, AI trading bot, best AI trading platform, automated trading, copy trading platform, AI trader, crypto trading bot, AI trading signals, multi-exchange trading, DeepSeek, Qwen, GPT-4, Claude'
      : 'AI交易, AI复制交易, AI交易机器人, 最佳AI交易平台, 自动化交易, 复制交易平台, AI交易员, 加密交易机器人, AI交易信号, 多交易所交易, DeepSeek, Qwen',
    ogImage: DEFAULT_OG_IMAGE,
    ogType: 'website',
    canonical: BASE_URL,
  }),

  '/competition': (lang: Language) => ({
    title: lang === 'en'
      ? 'Trading Competition - AI Trading 24x7 Platform'
      : '交易竞赛 - AI Trading 24x7 平台',
    description: lang === 'en'
      ? 'View live trading competition leaderboard. See top AI traders ranked by performance, equity, and trading metrics.'
      : '查看实时交易竞赛排行榜。查看按表现、权益和交易指标排名的顶级AI交易员。',
    keywords: lang === 'en'
      ? 'trading competition, leaderboard, AI traders, trading performance'
      : '交易竞赛, 排行榜, AI交易员, 交易表现',
    ogImage: DEFAULT_OG_IMAGE,
    ogType: 'website',
    canonical: `${BASE_URL}/competition`,
  }),

  '/traders': (lang: Language) => ({
    title: lang === 'en'
      ? 'AI Traders - Browse and Follow Top Traders'
      : 'AI交易员 - 浏览并跟随顶级交易员',
    description: lang === 'en'
      ? 'Browse available AI traders. Configure and follow top-performing traders to replicate their trading strategies.'
      : '浏览可用的AI交易员。配置并跟随表现优异的交易员，复制他们的交易策略。',
    keywords: lang === 'en'
      ? 'AI traders, follow traders, trading strategies, copy trading'
      : 'AI交易员, 跟随交易员, 交易策略, 复制交易',
    ogImage: DEFAULT_OG_IMAGE,
    ogType: 'website',
    canonical: `${BASE_URL}/traders`,
  }),

  '/faq': (lang: Language) => ({
    title: lang === 'en'
      ? 'Frequently Asked Questions - AI Trading 24x7'
      : '常见问题 - AI Trading 24x7',
    description: lang === 'en'
      ? 'Find answers to common questions about AI Trading 24x7 platform, features, trading strategies, and more.'
      : '查找关于AI Trading 24x7平台、功能、交易策略等的常见问题解答。',
    keywords: lang === 'en'
      ? 'FAQ, AI trading help, trading questions, platform guide'
      : '常见问题, AI交易帮助, 交易问题, 平台指南',
    ogImage: DEFAULT_OG_IMAGE,
    ogType: 'website',
    canonical: `${BASE_URL}/faq`,
  }),

  '/dashboard': (lang: Language) => ({
    title: lang === 'en'
      ? 'Trading Dashboard - AI Trading 24x7'
      : '交易仪表板 - AI Trading 24x7',
    description: lang === 'en'
      ? 'Your personal trading dashboard. Monitor positions, account balance, and trading performance.'
      : '您的个人交易仪表板。监控持仓、账户余额和交易表现。',
    keywords: lang === 'en'
      ? 'trading dashboard, portfolio, trading performance'
      : '交易仪表板, 投资组合, 交易表现',
    ogImage: DEFAULT_OG_IMAGE,
    ogType: 'website',
    noindex: true, // Private page
    canonical: `${BASE_URL}/dashboard`,
  }),

  '/followers': (lang: Language) => ({
    title: lang === 'en'
      ? 'My Followers - AI Trading 24x7'
      : '我的跟随者 - AI Trading 24x7',
    description: lang === 'en'
      ? 'View all followers of your traders and their activities.'
      : '查看您交易员的所有跟随者及其活动。',
    keywords: lang === 'en'
      ? 'followers, trader followers, trading community'
      : '跟随者, 交易员跟随者, 交易社区',
    ogImage: DEFAULT_OG_IMAGE,
    ogType: 'website',
    noindex: true, // Private page
    canonical: `${BASE_URL}/followers`,
  }),

  '/backtest': (lang: Language) => ({
    title: lang === 'en'
      ? 'Backtest Lab - AI Trading 24x7'
      : '回测实验室 - AI Trading 24x7',
    description: lang === 'en'
      ? 'Backtest your trading strategies with historical data. Test AI models and analyze performance.'
      : '使用历史数据回测您的交易策略。测试AI模型并分析表现。',
    keywords: lang === 'en'
      ? 'backtest, trading strategies, historical data, AI testing'
      : '回测, 交易策略, 历史数据, AI测试',
    ogImage: DEFAULT_OG_IMAGE,
    ogType: 'website',
    noindex: true, // Private page
    canonical: `${BASE_URL}/backtest`,
  }),

  '/webhook': (lang: Language) => ({
    title: lang === 'en'
      ? 'Webhook Configuration - AI Trading 24x7'
      : 'Webhook配置 - AI Trading 24x7',
    description: lang === 'en'
      ? 'Configure TradingView webhooks for automated trading signals.'
      : '配置TradingView webhooks以实现自动化交易信号。',
    keywords: lang === 'en'
      ? 'webhook, TradingView, trading signals, automation'
      : 'webhook, TradingView, 交易信号, 自动化',
    ogImage: DEFAULT_OG_IMAGE,
    ogType: 'website',
    noindex: true, // Private page
    canonical: `${BASE_URL}/webhook`,
  }),

  '/stats': (lang: Language) => ({
    title: lang === 'en'
      ? 'Admin Statistics - AI Trading 24x7'
      : '管理员统计 - AI Trading 24x7',
    description: lang === 'en'
      ? 'Administrative statistics and system metrics.'
      : '管理统计和系统指标。',
    keywords: lang === 'en'
      ? 'admin stats, system metrics, platform statistics'
      : '管理统计, 系统指标, 平台统计',
    ogImage: DEFAULT_OG_IMAGE,
    ogType: 'website',
    noindex: true, // Admin page - should not be indexed
    canonical: `${BASE_URL}/stats`,
  }),

  '/login': (lang: Language) => ({
    title: lang === 'en'
      ? 'Login - AI Trading 24x7'
      : '登录 - AI Trading 24x7',
    description: lang === 'en'
      ? 'Sign in to your AI Trading 24x7 account to access your trading dashboard and manage your portfolio.'
      : '登录您的AI Trading 24x7账户，访问交易仪表板并管理您的投资组合。',
    keywords: lang === 'en'
      ? 'login, sign in, AI trading account'
      : '登录, 登入, AI交易账户',
    ogImage: DEFAULT_OG_IMAGE,
    ogType: 'website',
    canonical: `${BASE_URL}/login`,
  }),

  '/register': (lang: Language) => ({
    title: lang === 'en'
      ? 'Register - AI Trading 24x7'
      : '注册 - AI Trading 24x7',
    description: lang === 'en'
      ? 'Create a new AI Trading 24x7 account to start automated cryptocurrency trading with multiple AI models.'
      : '创建新的AI Trading 24x7账户，开始使用多个AI模型进行自动化加密货币交易。',
    keywords: lang === 'en'
      ? 'register, sign up, create account, AI trading'
      : '注册, 注册账户, 创建账户, AI交易',
    ogImage: DEFAULT_OG_IMAGE,
    ogType: 'website',
    canonical: `${BASE_URL}/register`,
  }),

  '/become-trader': (lang: Language) => ({
    title: lang === 'en'
      ? 'Become a Trader - AI Trading 24x7'
      : '成为交易员 - AI Trading 24x7',
    description: lang === 'en'
      ? 'Apply to become a trader and let others follow your trading strategies. Share your expertise and build your trading community.'
      : '申请成为交易员，让其他用户跟随您的交易策略。分享您的专业知识，建立您的交易社区。',
    keywords: lang === 'en'
      ? 'become trader, trader application, trading community'
      : '成为交易员, 交易员申请, 交易社区',
    ogImage: DEFAULT_OG_IMAGE,
    ogType: 'website',
    canonical: `${BASE_URL}/become-trader`,
  }),

  '/admin/trader-applications': (lang: Language) => ({
    title: lang === 'en'
      ? 'Admin - Trader Applications - AI Trading 24x7'
      : '管理员 - 交易员申请 - AI Trading 24x7',
    description: lang === 'en'
      ? 'Administrative panel for managing trader applications.'
      : '管理交易员申请的管理面板。',
    keywords: lang === 'en'
      ? 'admin, trader applications, administration'
      : '管理员, 交易员申请, 管理',
    ogImage: DEFAULT_OG_IMAGE,
    ogType: 'website',
    noindex: true, // Admin page - should not be indexed
    canonical: `${BASE_URL}/admin/trader-applications`,
  }),

  '/terms': (lang: Language) => ({
    title: lang === 'en'
      ? 'Terms of Service - AI Trading 24x7'
      : '服务条款 - AI Trading 24x7',
    description: lang === 'en'
      ? 'Terms of Service and legal agreement for AI Trading 24x7 platform. Read our terms, conditions, and liability disclaimers.'
      : 'AI Trading 24x7平台的服务条款和法律协议。阅读我们的条款、条件和责任免责声明。',
    keywords: lang === 'en'
      ? 'terms of service, legal agreement, trading platform terms, liability disclaimer'
      : '服务条款, 法律协议, 交易平台条款, 责任免责声明',
    ogImage: DEFAULT_OG_IMAGE,
    ogType: 'website',
    canonical: `${BASE_URL}/terms`,
  }),

  '/risk-disclaimer': (lang: Language) => ({
    title: lang === 'en'
      ? 'Risk Disclaimer - AI Trading 24x7 | Critical Trading Warnings'
      : '风险免责声明 - AI Trading 24x7 | 重要交易警告',
    description: lang === 'en'
      ? 'Critical risk disclaimer for cryptocurrency trading. Understand the substantial risks involved in trading, including potential loss of capital, market volatility, and leverage risks.'
      : '加密货币交易的重要风险免责声明。了解交易涉及的重大风险，包括潜在的资本损失、市场波动和杠杆风险。',
    keywords: lang === 'en'
      ? 'risk disclaimer, trading risks, cryptocurrency risks, trading warnings, financial risk, investment risk'
      : '风险免责声明, 交易风险, 加密货币风险, 交易警告, 金融风险, 投资风险',
    ogImage: DEFAULT_OG_IMAGE,
    ogType: 'website',
    canonical: `${BASE_URL}/risk-disclaimer`,
  }),

  '/license': (lang: Language) => ({
    title: lang === 'en'
      ? 'License - AGPL-3.0 | AI Trading 24x7 Open Source'
      : '许可证 - AGPL-3.0 | AI Trading 24x7 开源',
    description: lang === 'en'
      ? 'AI Trading 24x7 is licensed under AGPL-3.0. View source code, license file, and understand your rights to use, modify, and distribute the software.'
      : 'AI Trading 24x7采用AGPL-3.0许可证。查看源代码、许可证文件，了解您使用、修改和分发软件的权利。',
    keywords: lang === 'en'
      ? 'AGPL-3.0, open source license, source code, GitHub, copyleft, free software'
      : 'AGPL-3.0, 开源许可证, 源代码, GitHub, 版权左派, 自由软件',
    ogImage: DEFAULT_OG_IMAGE,
    ogType: 'website',
    canonical: `${BASE_URL}/license`,
  }),

  '/pricing': (lang: Language) => ({
    title: lang === 'en'
      ? 'Pricing - AI Trading 24x7 | Free Trial Available'
      : '定价 - AI Trading 24x7 | 免费试用',
    description: lang === 'en'
      ? 'AI Trading 24x7 pricing plans. Currently free during testing phase. View features, limitations, and future pricing tiers.'
      : 'AI Trading 24x7定价计划。测试阶段目前免费。查看功能、限制和未来定价层级。',
    keywords: lang === 'en'
      ? 'pricing, subscription, free trial, trading platform pricing, AI trading cost'
      : '定价, 订阅, 免费试用, 交易平台定价, AI交易成本',
    ogImage: DEFAULT_OG_IMAGE,
    ogType: 'website',
    canonical: `${BASE_URL}/pricing`,
  }),

  '/about': (lang: Language) => ({
    title: lang === 'en'
      ? 'About Us - AI Trading 24x7 | Our Story & Mission'
      : '关于我们 - AI Trading 24x7 | 我们的故事和使命',
    description: lang === 'en'
      ? 'Learn about AI Trading 24x7, our mission to create a universal AI trading OS, team, backing by Amber.ac, and commitment to open source.'
      : '了解AI Trading 24x7，我们创建通用AI交易操作系统的使命，团队，Amber.ac的支持，以及对开源的承诺。',
    keywords: lang === 'en'
      ? 'about us, AI trading platform, trading OS, team, mission, vision, open source'
      : '关于我们, AI交易平台, 交易操作系统, 团队, 使命, 愿景, 开源',
    ogImage: DEFAULT_OG_IMAGE,
    ogType: 'website',
    canonical: `${BASE_URL}/about`,
  }),

  '/features': (lang: Language) => ({
    title: lang === 'en'
      ? 'Features - AI Trading 24x7 | Platform Capabilities'
      : '功能 - AI Trading 24x7 | 平台能力',
    description: lang === 'en'
      ? 'Explore all features of AI Trading 24x7: AI models, multi-agent competition, copy trading, webhook integration, security, and technical highlights.'
      : '探索AI Trading 24x7的所有功能：AI模型、多代理竞争、复制交易、Webhook集成、安全和技术亮点。',
    keywords: lang === 'en'
      ? 'features, AI trading features, copy trading, multi-agent, webhook, trading capabilities'
      : '功能, AI交易功能, 复制交易, 多代理, webhook, 交易能力',
    ogImage: DEFAULT_OG_IMAGE,
    ogType: 'website',
    canonical: `${BASE_URL}/features`,
  }),

  '/security': (lang: Language) => ({
    title: lang === 'en'
      ? 'Security - AI Trading 24x7 | Trust & Safety'
      : '安全 - AI Trading 24x7 | 信任与安全',
    description: lang === 'en'
      ? 'Learn about AI Trading 24x7 security measures: encryption, credential management, authentication, monitoring, and best practices for safe trading.'
      : '了解AI Trading 24x7的安全措施：加密、凭证管理、身份验证、监控和安全交易的最佳实践。',
    keywords: lang === 'en'
      ? 'security, encryption, trading security, API security, authentication, secure trading platform'
      : '安全, 加密, 交易安全, API安全, 身份验证, 安全交易平台',
    ogImage: DEFAULT_OG_IMAGE,
    ogType: 'website',
    canonical: `${BASE_URL}/security`,
  }),

  '/contact': (lang: Language) => ({
    title: lang === 'en'
      ? 'Contact - AI Trading 24x7 | Support & Help'
      : '联系我们 - AI Trading 24x7 | 支持与帮助',
    description: lang === 'en'
      ? 'Get in touch with AI Trading 24x7. Email us at contact@aitrading247.com for support, bug reports, security issues, or general inquiries.'
      : '联系AI Trading 24x7。发送邮件至contact@aitrading247.com获取支持、报告错误、安全问题或一般询问。',
    keywords: lang === 'en'
      ? 'contact, support, help, bug report, security report, trading platform support, email'
      : '联系, 支持, 帮助, 错误报告, 安全报告, 交易平台支持, 电子邮件',
    ogImage: DEFAULT_OG_IMAGE,
    ogType: 'website',
    canonical: `${BASE_URL}/contact`,
  }),
}

export function getSEOConfig(pathname: string, lang: Language): SEOConfig {
  const configFn = seoConfig[pathname]
  if (configFn) {
    return configFn(lang)
  }
  return defaultSEO
}

