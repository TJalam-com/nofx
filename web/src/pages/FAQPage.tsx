import { FAQLayout } from '../components/faq/FAQLayout'
import { useLanguage } from '../contexts/LanguageContext'
import { FAQPageSchema } from '../components/StructuredData'
import { faqCategories } from '../data/faqData'
import { t } from '../i18n/translations'

/**
 * FAQ 页面
 *
 * HeaderBar 和 Footer 现在由 MainLayout 提供
 *
 * 所有 FAQ 相关的逻辑都在子组件中：
 * - FAQLayout: 整体布局和搜索逻辑
 * - FAQSearchBar: 搜索框
 * - FAQSidebar: 左侧目录
 * - FAQContent: 右侧内容区
 *
 * FAQ 数据配置在 data/faqData.ts
 */
export function FAQPage() {
  const { language } = useLanguage()

  // Extract FAQ data for structured data
  const faqs = faqCategories.flatMap((category) =>
    category.items.map((item) => ({
      question: t(item.questionKey, language),
      answer: t(item.answerKey, language),
    }))
  )

  return (
    <>
      <FAQPageSchema faqs={faqs} />
      <FAQLayout language={language} />
    </>
  )
}
