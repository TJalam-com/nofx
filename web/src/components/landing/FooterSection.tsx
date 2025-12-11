import Footer from '../Footer'
import { Language } from '../../i18n/translations'

interface FooterSectionProps {
  language: Language
}

export default function FooterSection({}: FooterSectionProps) {
  return <Footer variant="full" />
}
