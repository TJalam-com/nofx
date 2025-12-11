import { AlertTriangle } from 'lucide-react'
import { t, type Language } from '../../../i18n/translations'

interface SignalSourceWarningProps {
  language: Language
  onConfigure: () => void
}

export function SignalSourceWarning({
  language,
  onConfigure,
}: SignalSourceWarningProps) {
  return (
    <div
      className="rounded-lg px-4 py-3 flex items-start gap-3 animate-slide-in"
      style={{
        background: 'rgba(246, 70, 93, 0.1)',
        border: '1px solid rgba(246, 70, 93, 0.3)',
      }}
    >
      <AlertTriangle
        size={20}
        className="flex-shrink-0 mt-0.5"
        style={{ color: '#F6465D' }}
      />
      <div className="flex-1">
        <div className="font-semibold mb-1" style={{ color: '#F6465D' }}>
          ⚠️ {t('signalSourceNotConfigured', language)}
        </div>
        <div className="text-sm" style={{ color: '#848E9C' }}>
          <p className="mb-2">{t('signalSourceWarningMessage', language)}</p>
          <p>
            <strong>{t('solutions', language)}</strong>
          </p>
          <ul className="list-disc list-inside space-y-1 ml-2 mt-1">
            <li>{t('solution1', language, { signalSource: t('signalSource', language) })}</li>
            <li>{t('solution2', language)}</li>
            <li>{t('solution3', language)}</li>
          </ul>
        </div>
        <button
          onClick={onConfigure}
          className="mt-3 px-3 py-1.5 rounded text-sm font-semibold transition-all hover:scale-105"
          style={{
            background: 'var(--green-primary)',
            color: 'var(--navy-primary)',
          }}
        >
          {t('configureSignalSourceNow', language)}
        </button>
      </div>
    </div>
  )
}
