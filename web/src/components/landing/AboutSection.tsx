import { motion } from 'framer-motion'
import { Shield, Target, Zap, Globe, Lock, TrendingUp } from 'lucide-react'
import AnimatedSection from './AnimatedSection'
import { t, Language } from '../../i18n/translations'

interface AboutSectionProps {
  language: Language
}

const getFeatureCards = (language: Language) => [
  {
    icon: Zap,
    title: language === 'en' ? 'AI-Powered' : 'AI 驱动',
    description: language === 'en' 
      ? 'Multi-model support (DeepSeek, Qwen) with custom prompts and intelligent decision-making'
      : '多模型支持（DeepSeek、Qwen），自定义提示词，智能决策',
  },
  {
    icon: Globe,
    title: language === 'en' ? 'Cloud-Based' : '云端服务',
    description: language === 'en'
      ? 'Accessible anywhere, anytime. No local setup required. Fully managed SaaS platform'
      : '随时随地访问，无需本地设置。完全托管的 SaaS 平台',
  },
  {
    icon: Lock,
    title: language === 'en' ? 'Secure' : '安全可靠',
    description: language === 'en'
      ? 'Non-custodial platform with fine-grained API controls and real-time risk monitoring'
      : '非托管平台，API 权限精细控制，实时风险监控',
  },
  {
    icon: TrendingUp,
    title: language === 'en' ? 'Advanced Features' : '高级功能',
    description: language === 'en'
      ? 'Copy trading, TradingView webhook integration, and automated strategy execution'
      : '跟单交易、TradingView Webhook 集成和自动化策略执行',
  },
]

export default function AboutSection({ language }: AboutSectionProps) {
  const featureCards = getFeatureCards(language)
  
  return (
    <AnimatedSection id="about" backgroundColor="var(--navy-dark)">
      <div className="max-w-7xl mx-auto">
        <div className="grid lg:grid-cols-2 gap-12 items-start">
          {/* Left Content */}
          <motion.div
            className="space-y-6"
            initial={{ opacity: 0, x: -50 }}
            whileInView={{ opacity: 1, x: 0 }}
            viewport={{ once: true }}
            transition={{ duration: 0.6 }}
          >
            <motion.div
              className="inline-flex items-center gap-2 px-4 py-2 rounded-full"
              style={{
                background: 'rgba(0, 255, 127, 0.1)',
                border: '1px solid rgba(0, 255, 127, 0.2)',
              }}
              whileHover={{ scale: 1.05 }}
            >
              <Target
                className="w-4 h-4"
                style={{ color: 'var(--green-primary)' }}
              />
              <span
                className="text-sm font-semibold"
                style={{ color: 'var(--green-primary)' }}
              >
                {t('aboutNofx', language)}
              </span>
            </motion.div>

            <h2
              className="text-4xl lg:text-5xl font-bold leading-tight"
              style={{ color: 'var(--text-primary)' }}
            >
              {t('whatIsNofx', language)}
            </h2>
            
            <div className="space-y-4">
              <p
                className="text-lg leading-relaxed"
                style={{ color: 'var(--text-secondary)' }}
              >
                {t('nofxNotAnotherBot', language)}{' '}
                {t('nofxDescription1', language)}{' '}
                {t('nofxDescription2', language)}
              </p>
              <p
                className="text-lg leading-relaxed"
                style={{ color: 'var(--text-secondary)' }}
              >
                {t('nofxDescription3', language)}{' '}
                {t('nofxDescription4', language)}{' '}
                {t('nofxDescription5', language)}
              </p>
            </div>

            <motion.div
              className="flex items-start gap-4 p-6 rounded-xl border-2 transition-all"
              style={{
                background: 'var(--navy-dark)',
                borderColor: 'var(--panel-border)',
              }}
              whileHover={{
                borderColor: 'var(--green-primary)',
                boxShadow: '0 0 20px var(--green-glow)',
              }}
              initial={{ opacity: 0, y: 20 }}
              whileInView={{ opacity: 1, y: 0 }}
              viewport={{ once: true }}
              transition={{ duration: 0.5, delay: 0.2 }}
            >
              <div
                className="w-14 h-14 rounded-xl flex items-center justify-center flex-shrink-0"
                style={{
                  background: 'linear-gradient(135deg, rgba(0, 255, 127, 0.2) 0%, rgba(0, 255, 127, 0.05) 100%)',
                  border: '1px solid rgba(0, 255, 127, 0.3)',
                }}
              >
                <Shield
                  className="w-7 h-7"
                  style={{ color: 'var(--green-primary)' }}
                />
              </div>
              <div>
                <div
                  className="font-bold text-lg mb-1"
                  style={{ color: 'var(--text-primary)' }}
                >
                  {t('youFullControl', language)}
                </div>
                <div
                  className="text-sm leading-relaxed"
                  style={{ color: 'var(--text-secondary)' }}
                >
                  {t('fullControlDesc', language)}
                </div>
              </div>
            </motion.div>
          </motion.div>

          {/* Right Content - Feature Grid */}
          <motion.div
            className="grid grid-cols-1 sm:grid-cols-2 gap-4 lg:gap-6"
            initial={{ opacity: 0, x: 50 }}
            whileInView={{ opacity: 1, x: 0 }}
            viewport={{ once: true }}
            transition={{ duration: 0.6, delay: 0.2 }}
          >
            {featureCards.map((feature, index) => {
              const Icon = feature.icon
              return (
                <motion.div
                  key={index}
                  className="p-6 lg:p-7 rounded-xl border-2 transition-all relative overflow-hidden h-full"
                  style={{
                    background: 'var(--navy-dark)',
                    borderColor: 'var(--panel-border)',
                  }}
                  initial={{ opacity: 0, y: 20 }}
                  whileInView={{ opacity: 1, y: 0 }}
                  viewport={{ once: true }}
                  transition={{ duration: 0.5, delay: 0.3 + index * 0.1 }}
                  whileHover={{
                    borderColor: 'var(--green-primary)',
                    boxShadow: '0 0 25px var(--green-glow)',
                    y: -8,
                    scale: 1.02,
                  }}
                >
                  {/* Background glow effect */}
                  <motion.div
                    className="absolute inset-0 pointer-events-none"
                    style={{
                      background: 'radial-gradient(circle at center, var(--green-glow) 0%, transparent 70%)',
                    }}
                    initial={{ opacity: 0 }}
                    whileHover={{ opacity: 1 }}
                    transition={{ duration: 0.3 }}
                  />
                  
                  {/* Subtle pattern overlay */}
                  <div
                    className="absolute inset-0 opacity-5 pointer-events-none"
                    style={{
                      backgroundImage: `radial-gradient(circle at 2px 2px, var(--green-primary) 1px, transparent 0)`,
                      backgroundSize: '24px 24px',
                    }}
                  />
                  
                  <div className="relative z-10 flex flex-col h-full">
                    <div
                      className="w-14 h-14 lg:w-16 lg:h-16 rounded-xl flex items-center justify-center mb-4 flex-shrink-0"
                      style={{
                        background: 'linear-gradient(135deg, rgba(0, 255, 127, 0.2) 0%, rgba(0, 255, 127, 0.05) 100%)',
                        border: '1px solid rgba(0, 255, 127, 0.3)',
                        boxShadow: '0 4px 12px rgba(0, 255, 127, 0.1)',
                      }}
                    >
                      <Icon
                        className="w-7 h-7 lg:w-8 lg:h-8"
                        style={{ color: 'var(--green-primary)' }}
                      />
                    </div>
                    <h3
                      className="font-bold text-lg lg:text-xl mb-2"
                      style={{ color: 'var(--text-primary)' }}
                    >
                      {feature.title}
                    </h3>
                    <p
                      className="text-sm lg:text-base leading-relaxed flex-grow"
                      style={{ color: 'var(--text-secondary)' }}
                    >
                      {feature.description}
                    </p>
                  </div>
                </motion.div>
              )
            })}
          </motion.div>
        </div>
      </div>
    </AnimatedSection>
  )
}
