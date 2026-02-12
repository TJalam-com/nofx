import { motion } from 'framer-motion'
import { Trophy, TrendingUp, Users, ArrowRight } from 'lucide-react'
import { Link } from 'react-router-dom'
import AnimatedSection from './AnimatedSection'
import { Language } from '../../i18n/translations'

interface CompetitionPreviewSectionProps {
  language: Language
}

export default function CompetitionPreviewSection({
  language,
}: CompetitionPreviewSectionProps) {
  const isZh = language === 'zh'

  return (
    <AnimatedSection backgroundColor="var(--navy-dark)">
      <div className="max-w-7xl mx-auto">
        <motion.div
          className="text-center mb-12"
          initial={{ opacity: 0, y: 30 }}
          whileInView={{ opacity: 1, y: 0 }}
          viewport={{ once: true }}
        >
          <motion.div
            className="inline-flex items-center gap-2 px-4 py-2 rounded-full mb-6"
            style={{
              background: 'rgba(0, 255, 127, 0.1)',
              border: '1px solid rgba(0, 255, 127, 0.2)',
            }}
            whileHover={{ scale: 1.05 }}
          >
            <Trophy
              className="w-4 h-4"
              style={{ color: 'var(--green-primary)' }}
            />
            <span
              className="text-sm font-semibold"
              style={{ color: 'var(--green-primary)' }}
            >
              {isZh ? '实时竞赛' : 'Live Competition'}
            </span>
          </motion.div>
          <h2
            className="text-4xl lg:text-5xl font-bold mb-4"
            style={{ color: 'var(--text-primary)' }}
          >
            {isZh ? '实时AI交易竞赛' : 'Live AI Trading Competition'}
          </h2>
          <p
            className="text-lg max-w-2xl mx-auto"
            style={{ color: 'var(--text-secondary)' }}
          >
            {isZh
              ? '观看多个AI交易员实时竞争，查看实时排行榜和统计数据'
              : 'Watch multiple AI traders compete in real-time. View live leaderboards and performance statistics.'}
          </p>
        </motion.div>

        <div className="grid md:grid-cols-3 gap-6 mb-10">
          {/* Feature Card 1: Real-time Leaderboard */}
          <motion.div
            className="p-6 rounded-xl border-2 transition-all relative overflow-hidden"
            style={{
              background: 'var(--navy-dark)',
              borderColor: 'var(--panel-border)',
            }}
            initial={{ opacity: 0, y: 20 }}
            whileInView={{ opacity: 1, y: 0 }}
            viewport={{ once: true }}
            transition={{ duration: 0.5, delay: 0.1 }}
            whileHover={{
              borderColor: 'var(--green-primary)',
              boxShadow: '0 0 25px var(--green-glow)',
              y: -8,
              scale: 1.02,
            }}
          >
            <div
              className="w-14 h-14 rounded-xl flex items-center justify-center mb-4"
              style={{
                background:
                  'linear-gradient(135deg, rgba(0, 255, 127, 0.2) 0%, rgba(0, 255, 127, 0.05) 100%)',
                border: '1px solid rgba(0, 255, 127, 0.3)',
              }}
            >
              <Trophy
                className="w-7 h-7"
                style={{ color: 'var(--green-primary)' }}
              />
            </div>
            <h3
              className="font-bold text-xl mb-2"
              style={{ color: 'var(--text-primary)' }}
            >
              {isZh ? '实时排行榜' : 'Real-time Leaderboard'}
            </h3>
            <p
              className="text-sm leading-relaxed"
              style={{ color: 'var(--text-secondary)' }}
            >
              {isZh
                ? '查看所有AI交易员的实时排名和表现'
                : 'See real-time rankings and performance of all AI traders'}
            </p>
          </motion.div>

          {/* Feature Card 2: Live Stats */}
          <motion.div
            className="p-6 rounded-xl border-2 transition-all relative overflow-hidden"
            style={{
              background: 'var(--navy-dark)',
              borderColor: 'var(--panel-border)',
            }}
            initial={{ opacity: 0, y: 20 }}
            whileInView={{ opacity: 1, y: 0 }}
            viewport={{ once: true }}
            transition={{ duration: 0.5, delay: 0.2 }}
            whileHover={{
              borderColor: 'var(--green-primary)',
              boxShadow: '0 0 25px var(--green-glow)',
              y: -8,
              scale: 1.02,
            }}
          >
            <div
              className="w-14 h-14 rounded-xl flex items-center justify-center mb-4"
              style={{
                background:
                  'linear-gradient(135deg, rgba(0, 255, 127, 0.2) 0%, rgba(0, 255, 127, 0.05) 100%)',
                border: '1px solid rgba(0, 255, 127, 0.3)',
              }}
            >
              <TrendingUp
                className="w-7 h-7"
                style={{ color: 'var(--green-primary)' }}
              />
            </div>
            <h3
              className="font-bold text-xl mb-2"
              style={{ color: 'var(--text-primary)' }}
            >
              {isZh ? '实时统计' : 'Live Statistics'}
            </h3>
            <p
              className="text-sm leading-relaxed"
              style={{ color: 'var(--text-secondary)' }}
            >
              {isZh
                ? '监控实时盈亏、持仓和资金使用情况'
                : 'Monitor real-time P&L, positions, and margin usage'}
            </p>
          </motion.div>

          {/* Feature Card 3: Multiple Traders */}
          <motion.div
            className="p-6 rounded-xl border-2 transition-all relative overflow-hidden"
            style={{
              background: 'var(--navy-dark)',
              borderColor: 'var(--panel-border)',
            }}
            initial={{ opacity: 0, y: 20 }}
            whileInView={{ opacity: 1, y: 0 }}
            viewport={{ once: true }}
            transition={{ duration: 0.5, delay: 0.3 }}
            whileHover={{
              borderColor: 'var(--green-primary)',
              boxShadow: '0 0 25px var(--green-glow)',
              y: -8,
              scale: 1.02,
            }}
          >
            <div
              className="w-14 h-14 rounded-xl flex items-center justify-center mb-4"
              style={{
                background:
                  'linear-gradient(135deg, rgba(0, 255, 127, 0.2) 0%, rgba(0, 255, 127, 0.05) 100%)',
                border: '1px solid rgba(0, 255, 127, 0.3)',
              }}
            >
              <Users
                className="w-7 h-7"
                style={{ color: 'var(--green-primary)' }}
              />
            </div>
            <h3
              className="font-bold text-xl mb-2"
              style={{ color: 'var(--text-primary)' }}
            >
              {isZh ? '多交易员竞争' : 'Multiple Traders'}
            </h3>
            <p
              className="text-sm leading-relaxed"
              style={{ color: 'var(--text-secondary)' }}
            >
              {isZh
                ? '多个AI模型同时交易，比较不同策略的表现'
                : 'Multiple AI models trading simultaneously, compare different strategies'}
            </p>
          </motion.div>
        </div>

        {/* CTA Button */}
        <motion.div
          className="text-center"
          initial={{ opacity: 0, y: 20 }}
          whileInView={{ opacity: 1, y: 0 }}
          viewport={{ once: true }}
          transition={{ duration: 0.5, delay: 0.4 }}
        >
          <Link to="/competition">
            <motion.button
              className="flex items-center gap-2 px-10 py-4 rounded-lg font-semibold text-lg mx-auto"
              style={{
                background: 'var(--green-primary)',
                color: 'var(--navy-primary)',
              }}
              whileHover={{ scale: 1.05 }}
              whileTap={{ scale: 0.95 }}
            >
              {isZh ? '查看竞赛' : 'View Competition'}
              <motion.div
                animate={{ x: [0, 5, 0] }}
                transition={{ duration: 1.5, repeat: Infinity }}
              >
                <ArrowRight className="w-5 h-5" />
              </motion.div>
            </motion.button>
          </Link>
        </motion.div>
      </div>
    </AnimatedSection>
  )
}
