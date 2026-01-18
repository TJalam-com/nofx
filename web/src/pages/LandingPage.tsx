import { useState } from 'react'
import { motion } from 'framer-motion'
import { ArrowRight } from 'lucide-react'
import HeaderBar from '../components/HeaderBar'
import HeroSection from '../components/landing/HeroSection'
import AboutSection from '../components/landing/AboutSection'
import FeaturesSection from '../components/landing/FeaturesSection'
import CompetitionPreviewSection from '../components/landing/CompetitionPreviewSection'
import BlogPreviewSection from '../components/landing/BlogPreviewSection'
import AnimatedSection from '../components/landing/AnimatedSection'
import LoginModal from '../components/landing/LoginModal'
import FooterSection from '../components/landing/FooterSection'
import { useAuth, isFollower } from '../contexts/AuthContext'
import { useLanguage } from '../contexts/LanguageContext'
import { t } from '../i18n/translations'
import { useNavigate } from 'react-router-dom'
import { useSEO } from '../hooks/useSEO'
import { OrganizationSchema, WebSiteSchema, SoftwareApplicationSchema } from '../components/StructuredData'

export function LandingPage() {
  const [showLoginModal, setShowLoginModal] = useState(false)
  const { user, logout } = useAuth()
  const { language } = useLanguage()
  const navigate = useNavigate()
  const isLoggedIn = !!user
  const userIsFollower = isFollower(user)
  const { SEOComponent } = useSEO()

  console.log('LandingPage - user:', user, 'isLoggedIn:', isLoggedIn)
  const baseUrl = import.meta.env.VITE_BASE_URL || 'https://aitrading247.com'

  return (
    <>
      <SEOComponent />
      <OrganizationSchema />
      <WebSiteSchema />
      <SoftwareApplicationSchema
        name="AI Trading 24x7"
        description="AI-powered copy trading platform supporting multiple exchanges and AI models. Automate trades with AI decision engine, copy top traders, multi-exchange support."
        applicationCategory="FinanceApplication"
        operatingSystem="Web"
        offers={[
          {
            name: 'Free Trial',
            price: '0',
            priceCurrency: 'USD',
            availability: 'https://schema.org/InStock',
            url: `${baseUrl}/register`,
          },
          // Add more pricing tiers as they become available
          // {
          //   name: 'Professional',
          //   price: '29',
          //   priceCurrency: 'USD',
          //   availability: 'https://schema.org/InStock',
          //   url: `${baseUrl}/register`,
          // },
        ]}
        aggregateRating={{
          ratingValue: 4.8,
          ratingCount: 1250,
          bestRating: 5,
          worstRating: 1,
        }}
      />
      <HeaderBar
        onLoginClick={() => setShowLoginModal(true)}
        isLoggedIn={isLoggedIn}
        isHomePage={true}
        language={language}
        user={user}
        onLogout={logout}
        onPageChange={(page) => {
          console.log('LandingPage onPageChange called with:', page)
          if (page === 'competition') {
            window.location.href = '/competition'
          } else if (page === 'traders') {
            window.location.href = '/traders'
          } else if (page === 'trader') {
            window.location.href = '/dashboard'
          }
        }}
      />
      <div
        className="min-h-screen px-4 sm:px-6 lg:px-8 overflow-x-hidden"
        style={{
          background: 'var(--navy-primary)',
          color: 'var(--text-primary)',
        }}
      >
        <HeroSection language={language} onGetStarted={() => setShowLoginModal(true)} />
        <CompetitionPreviewSection language={language} />
        <BlogPreviewSection language={language} />
        <AboutSection language={language} />
        <FeaturesSection language={language} />

        {/* Become a Trader CTA for Followers */}
        {isLoggedIn && userIsFollower && (
          <AnimatedSection backgroundColor="var(--navy-dark)">
            <div className="max-w-4xl mx-auto text-center">
              <motion.h2
                className="text-4xl font-bold mb-4"
                style={{ color: 'var(--text-primary)' }}
                initial={{ opacity: 0, y: 30 }}
                whileInView={{ opacity: 1, y: 0 }}
                viewport={{ once: true }}
              >
                {language === 'en' ? 'Become a Trader' : '成为交易员'}
              </motion.h2>
              <motion.p
                className="text-lg mb-8"
                style={{ color: 'var(--text-secondary)' }}
                initial={{ opacity: 0, y: 30 }}
                whileInView={{ opacity: 1, y: 0 }}
                viewport={{ once: true }}
                transition={{ delay: 0.1 }}
              >
                {language === 'en'
                  ? 'Apply to become a trader and let others follow your trading strategies. Share your expertise and build your trading community.'
                  : '申请成为交易员，让其他用户跟随您的交易策略。分享您的专业知识，建立您的交易社区。'}
              </motion.p>
              <motion.button
                onClick={() => navigate('/become-trader')}
                className="flex items-center gap-2 px-10 py-4 rounded-lg font-semibold text-lg mx-auto"
                style={{
                  background: 'var(--green-primary)',
                  color: 'var(--navy-primary)',
                }}
                whileHover={{ scale: 1.05 }}
                whileTap={{ scale: 0.95 }}
                initial={{ opacity: 0, y: 30 }}
                whileInView={{ opacity: 1, y: 0 }}
                viewport={{ once: true }}
                transition={{ delay: 0.2 }}
              >
                {language === 'en' ? 'Apply Now' : '立即申请'}
                <motion.div
                  animate={{ x: [0, 5, 0] }}
                  transition={{ duration: 1.5, repeat: Infinity }}
                >
                  <ArrowRight className="w-5 h-5" />
                </motion.div>
              </motion.button>
            </div>
          </AnimatedSection>
        )}

        {/* CTA */}
        <AnimatedSection backgroundColor="var(--navy-dark)">
          <div className="max-w-4xl mx-auto text-center">
            <motion.h2
              className="text-5xl font-bold mb-6"
              style={{ color: 'var(--brand-light-gray)' }}
              initial={{ opacity: 0, y: 30 }}
              whileInView={{ opacity: 1, y: 0 }}
              viewport={{ once: true }}
            >
              {t('readyToDefine', language)}
            </motion.h2>
            <motion.p
              className="text-xl mb-12"
              style={{ color: 'var(--text-secondary)' }}
              initial={{ opacity: 0, y: 30 }}
              whileInView={{ opacity: 1, y: 0 }}
              viewport={{ once: true }}
              transition={{ delay: 0.1 }}
            >
              {t('startWithCrypto', language)}
            </motion.p>
            <div className="flex flex-wrap justify-center gap-4">
              <motion.button
                onClick={() => setShowLoginModal(true)}
                className="flex items-center gap-2 px-10 py-4 rounded-lg font-semibold text-lg"
                style={{
                  background: 'var(--brand-yellow)',
                  color: 'var(--navy-primary)',
                }}
                whileHover={{ scale: 1.05 }}
                whileTap={{ scale: 0.95 }}
              >
                {t('getStartedNow', language)}
                <motion.div
                  animate={{ x: [0, 5, 0] }}
                  transition={{ duration: 1.5, repeat: Infinity }}
                >
                  <ArrowRight className="w-5 h-5" />
                </motion.div>
              </motion.button>
            </div>
          </div>
        </AnimatedSection>

        {showLoginModal && (
          <LoginModal
            onClose={() => setShowLoginModal(false)}
            language={language}
          />
        )}
        <FooterSection language={language} />
      </div>
    </>
  )
}
