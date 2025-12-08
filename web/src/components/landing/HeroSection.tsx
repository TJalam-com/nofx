import { motion, useScroll, useTransform } from 'framer-motion'
import { ArrowRight, Sparkles } from 'lucide-react'
import { t, Language } from '../../i18n/translations'
import { useNavigate } from 'react-router-dom'
import { useAuth } from '../../contexts/AuthContext'

interface HeroSectionProps {
  language: Language
  onGetStarted?: () => void
}

export default function HeroSection({ language, onGetStarted }: HeroSectionProps) {
  const { scrollYProgress } = useScroll()
  const opacity = useTransform(scrollYProgress, [0, 0.2], [1, 0])
  const scale = useTransform(scrollYProgress, [0, 0.2], [1, 0.8])
  const navigate = useNavigate()
  const { user } = useAuth()
  const isLoggedIn = !!user

  const fadeInUp = {
    initial: { opacity: 0, y: 60 },
    animate: { opacity: 1, y: 0 },
    transition: { duration: 0.7, ease: [0.6, -0.05, 0.01, 0.99] },
  }
  
  const staggerContainer = {
    animate: { transition: { staggerChildren: 0.15 } },
  }

  const handleGetStarted = () => {
    if (onGetStarted) {
      onGetStarted()
    } else if (isLoggedIn) {
      navigate('/competition')
    } else {
      navigate('/login')
    }
  }

  return (
    <section className="relative pt-24 pb-32 px-4 overflow-hidden">
      {/* Background Effects */}
      <div className="absolute inset-0 pointer-events-none">
        {/* Gradient Orbs */}
        <motion.div
          className="absolute top-1/4 -left-1/4 w-96 h-96 rounded-full blur-3xl opacity-20"
          style={{
            background: 'radial-gradient(circle, var(--green-primary) 0%, transparent 70%)',
          }}
          animate={{
            scale: [1, 1.2, 1],
            x: [0, 50, 0],
            y: [0, 30, 0],
          }}
          transition={{
            duration: 8,
            repeat: Infinity,
            ease: 'easeInOut',
          }}
        />
        <motion.div
          className="absolute top-1/3 -right-1/4 w-96 h-96 rounded-full blur-3xl opacity-10"
          style={{
            background: 'radial-gradient(circle, var(--green-primary) 0%, transparent 70%)',
          }}
          animate={{
            scale: [1, 1.3, 1],
            x: [0, -30, 0],
            y: [0, 50, 0],
          }}
          transition={{
            duration: 10,
            repeat: Infinity,
            ease: 'easeInOut',
          }}
        />
        
        {/* Grid Pattern */}
        <div
          className="absolute inset-0 opacity-5"
          style={{
            backgroundImage: `linear-gradient(var(--green-primary) 1px, transparent 1px), linear-gradient(90deg, var(--green-primary) 1px, transparent 1px)`,
            backgroundSize: '50px 50px',
          }}
        />
      </div>

      <div className="max-w-7xl mx-auto relative z-10">
        <div className="max-w-5xl mx-auto">
          <motion.div
            className="space-y-8 lg:space-y-10"
            style={{ opacity, scale }}
            initial="initial"
            animate="animate"
            variants={staggerContainer}
          >
            {/* Badge */}
            <motion.div
              className="inline-flex items-center gap-2 px-4 py-2 rounded-full"
              style={{
                background: 'rgba(0, 255, 127, 0.1)',
                border: '1px solid rgba(0, 255, 127, 0.2)',
              }}
              initial={{ opacity: 0, y: 20 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.6 }}
              whileHover={{ scale: 1.05 }}
            >
              <Sparkles className="w-4 h-4" style={{ color: 'var(--green-primary)' }} />
              <span
                className="text-sm font-semibold"
                style={{ color: 'var(--green-primary)' }}
              >
                {language === 'en' ? 'The Future of AI Trading' : 'AI 交易的未来'}
              </span>
            </motion.div>

            {/* Main Heading */}
            <motion.h1
              className="text-5xl sm:text-6xl lg:text-7xl xl:text-8xl font-bold leading-[1.1] tracking-tight"
              variants={fadeInUp}
            >
              <span style={{ color: 'var(--text-primary)' }}>
                {t('heroTitle1', language)}
              </span>
              <br />
              <span
                style={{ 
                  color: 'var(--green-primary)',
                  background: 'linear-gradient(135deg, var(--green-primary) 0%, var(--green-light) 100%)',
                  WebkitBackgroundClip: 'text',
                  WebkitTextFillColor: 'transparent',
                  backgroundClip: 'text',
                }}
              >
                {t('heroTitle2', language)}
              </span>
            </motion.h1>

            {/* Description */}
            <motion.p
              className="text-lg sm:text-xl lg:text-2xl leading-relaxed max-w-3xl"
              style={{ color: 'var(--text-secondary)' }}
              variants={fadeInUp}
            >
              {t('heroDescription', language)}
            </motion.p>

            {/* CTA Buttons */}
            <motion.div
              className="flex flex-col sm:flex-row gap-4 pt-4"
              variants={fadeInUp}
            >
              <motion.button
                onClick={handleGetStarted}
                className="group flex items-center justify-center gap-3 px-8 py-4 rounded-xl font-semibold text-lg transition-all"
                style={{
                  background: 'var(--green-primary)',
                  color: 'var(--navy-primary)',
                  boxShadow: '0 4px 20px var(--green-glow)',
                }}
                whileHover={{
                  scale: 1.05,
                  boxShadow: '0 8px 30px var(--green-glow)',
                }}
                whileTap={{ scale: 0.95 }}
              >
                <span>{t('getStartedNow', language)}</span>
                <motion.div
                  animate={{ x: [0, 5, 0] }}
                  transition={{ duration: 1.5, repeat: Infinity }}
                >
                  <ArrowRight className="w-5 h-5" />
                </motion.div>
              </motion.button>

              <motion.button
                onClick={() => {
                  const element = document.getElementById('features')
                  element?.scrollIntoView({ behavior: 'smooth' })
                }}
                className="group flex items-center justify-center gap-3 px-8 py-4 rounded-xl font-semibold text-lg border-2 transition-all"
                style={{
                  background: 'transparent',
                  color: 'var(--text-primary)',
                  borderColor: 'var(--panel-border)',
                }}
                whileHover={{
                  borderColor: 'var(--green-primary)',
                  color: 'var(--green-primary)',
                  background: 'rgba(0, 255, 127, 0.05)',
                }}
                whileTap={{ scale: 0.95 }}
              >
                <span>{language === 'en' ? 'Learn More' : '了解更多'}</span>
              </motion.button>
            </motion.div>

            {/* Trust Indicators */}
            <motion.div
              className="flex flex-wrap items-center gap-6 pt-8 text-sm"
              style={{ color: 'var(--text-tertiary)' }}
              variants={fadeInUp}
            >
              <div className="flex items-center gap-2">
                <div className="w-2 h-2 rounded-full" style={{ background: 'var(--green-primary)' }} />
                <span>{language === 'en' ? 'Non-custodial' : '非托管'}</span>
              </div>
              <div className="flex items-center gap-2">
                <div className="w-2 h-2 rounded-full" style={{ background: 'var(--green-primary)' }} />
                <span>{language === 'en' ? 'Cloud-based SaaS' : '云端 SaaS'}</span>
              </div>
              <div className="flex items-center gap-2">
                <div className="w-2 h-2 rounded-full" style={{ background: 'var(--green-primary)' }} />
                <span>{language === 'en' ? 'Multi-exchange support' : '多交易所支持'}</span>
              </div>
            </motion.div>
          </motion.div>
        </div>
      </div>
    </section>
  )
}
