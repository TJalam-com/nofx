import { motion } from 'framer-motion'
import { Link } from 'react-router-dom'
import useSWR from 'swr'
import { FileText, Calendar, ArrowRight } from 'lucide-react'
import { api } from '../../lib/api'
import type { Article } from '../../types'
import AnimatedSection from './AnimatedSection'
import { Language } from '../../i18n/translations'
import { convertImgBBUrl } from '../../utils/imgbb'

interface BlogPreviewSectionProps {
  language: Language
}

export default function BlogPreviewSection({
  language,
}: BlogPreviewSectionProps) {
  const isZh = language === 'zh'

  // Fetch latest 3 articles
  const { data: articles, isLoading: articlesLoading } = useSWR<Article[]>(
    'landing-blog-preview',
    () => api.getPublishedArticles(3, 0),
    { revalidateOnFocus: false }
  )

  const convertImgBBUrlLocal = (url: string): string => {
    return convertImgBBUrl(url)
  }

  return (
    <AnimatedSection backgroundColor="var(--navy-primary)">
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
            <FileText
              className="w-4 h-4"
              style={{ color: 'var(--green-primary)' }}
            />
            <span
              className="text-sm font-semibold"
              style={{ color: 'var(--green-primary)' }}
            >
              {isZh ? '最新文章' : 'Latest Articles'}
            </span>
          </motion.div>
          <h2
            className="text-4xl lg:text-5xl font-bold mb-4"
            style={{ color: 'var(--text-primary)' }}
          >
            {isZh ? '最新文章' : 'Latest Articles'}
          </h2>
          <p
            className="text-lg max-w-2xl mx-auto"
            style={{ color: 'var(--text-secondary)' }}
          >
            {isZh
              ? '了解AI交易、加密货币和交易策略的最新见解'
              : 'Stay updated with the latest insights on AI trading, cryptocurrency, and trading strategies'}
          </p>
        </motion.div>

        {/* Articles Grid */}
        {articlesLoading ? (
          <div
            className="text-center py-12"
            style={{ color: 'var(--text-secondary)' }}
          >
            {isZh ? '加载中...' : 'Loading articles...'}
          </div>
        ) : !articles || articles.length === 0 ? (
          <div
            className="text-center py-12"
            style={{ color: 'var(--text-secondary)' }}
          >
            <FileText size={48} className="mx-auto mb-4 opacity-50" />
            <p>{isZh ? '暂无文章' : 'No articles available'}</p>
          </div>
        ) : (
          <>
            <div className="grid md:grid-cols-3 gap-6 mb-10">
              {articles.map((article, index) => {
                const processedImageUrl = article.featured_image_url
                  ? convertImgBBUrlLocal(article.featured_image_url)
                  : null

                return (
                  <motion.div
                    key={article.id}
                    initial={{ opacity: 0, y: 20 }}
                    whileInView={{ opacity: 1, y: 0 }}
                    viewport={{ once: true }}
                    transition={{ duration: 0.5, delay: index * 0.1 }}
                  >
                    <Link
                      to={`/blog/${article.slug}`}
                      className="block rounded-xl border-2 overflow-hidden transition-all h-full group"
                      style={{
                        background: 'var(--navy-dark)',
                        borderColor: 'var(--panel-border)',
                      }}
                      onMouseEnter={(e) => {
                        e.currentTarget.style.borderColor =
                          'var(--green-primary)'
                        e.currentTarget.style.boxShadow =
                          '0 0 25px var(--green-glow)'
                      }}
                      onMouseLeave={(e) => {
                        e.currentTarget.style.borderColor =
                          'var(--panel-border)'
                        e.currentTarget.style.boxShadow = 'none'
                      }}
                    >
                      {processedImageUrl && (
                        <div className="aspect-[4/3] overflow-hidden bg-gray-100">
                          <img
                            src={processedImageUrl}
                            alt={article.title}
                            className="w-full h-full object-cover group-hover:scale-105 transition-transform duration-300"
                            onError={(e) => {
                              ;(e.target as HTMLImageElement).style.display =
                                'none'
                            }}
                          />
                        </div>
                      )}
                      <div className="p-6">
                        <h3
                          className="text-xl font-semibold mb-2 line-clamp-2 group-hover:text-green-400 transition-colors"
                          style={{ color: 'var(--text-primary)' }}
                        >
                          {article.title}
                        </h3>
                        <p
                          className="text-sm mb-4 line-clamp-3"
                          style={{ color: 'var(--text-secondary)' }}
                        >
                          {article.excerpt || article.meta_description}
                        </p>
                        <div
                          className="flex items-center gap-2 text-xs"
                          style={{ color: 'var(--text-tertiary)' }}
                        >
                          <Calendar size={14} />
                          {article.published_at
                            ? new Date(article.published_at).toLocaleDateString(
                                isZh ? 'zh-CN' : 'en-US',
                                {
                                  year: 'numeric',
                                  month: 'short',
                                  day: 'numeric',
                                }
                              )
                            : new Date(article.created_at).toLocaleDateString(
                                isZh ? 'zh-CN' : 'en-US',
                                {
                                  year: 'numeric',
                                  month: 'short',
                                  day: 'numeric',
                                }
                              )}
                        </div>
                      </div>
                    </Link>
                  </motion.div>
                )
              })}
            </div>

            {/* CTA Button */}
            <motion.div
              className="text-center"
              initial={{ opacity: 0, y: 20 }}
              whileInView={{ opacity: 1, y: 0 }}
              viewport={{ once: true }}
              transition={{ duration: 0.5, delay: 0.4 }}
            >
              <Link to="/blog">
                <motion.button
                  className="flex items-center gap-2 px-10 py-4 rounded-lg font-semibold text-lg mx-auto"
                  style={{
                    background: 'var(--brand-yellow)',
                    color: 'var(--navy-primary)',
                  }}
                  whileHover={{ scale: 1.05 }}
                  whileTap={{ scale: 0.95 }}
                >
                  {isZh ? '查看所有文章' : 'View All Articles'}
                  <motion.div
                    animate={{ x: [0, 5, 0] }}
                    transition={{ duration: 1.5, repeat: Infinity }}
                  >
                    <ArrowRight className="w-5 h-5" />
                  </motion.div>
                </motion.button>
              </Link>
            </motion.div>
          </>
        )}
      </div>
    </AnimatedSection>
  )
}
