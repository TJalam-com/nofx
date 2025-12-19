import { useState } from 'react'
import { Eye, EyeOff, Copy, Check } from 'lucide-react'
import { copyWithToast } from '../lib/clipboard'
import { t, type Language } from '../i18n/translations'

interface WalletAddressDisplayProps {
  address: string
  language: Language
  visibleChars?: number // Number of characters to show when truncated (default: 6)
}

/**
 * WalletAddressDisplay Component
 * 
 * Displays wallet addresses for perp-dex exchanges with:
 * - Visibility toggle (truncated/full view)
 * - Copy to clipboard functionality
 * - Responsive design
 * - Dark/light mode compatible
 * 
 * Usage:
 * <WalletAddressDisplay address="0x1234..." language="en" />
 */
export function WalletAddressDisplay({
  address,
  language,
  visibleChars = 6,
}: WalletAddressDisplayProps) {
  const [isVisible, setIsVisible] = useState(false)
  const [copied, setCopied] = useState(false)

  if (!address || address.trim() === '') {
    return null
  }

  const displayAddress = isVisible
    ? address
    : `${address.slice(0, visibleChars)}...${address.slice(-visibleChars)}`

  const handleCopy = async () => {
    const success = await copyWithToast(
      address,
      t('copiedToClipboard', language) || 'Copied to clipboard'
    )
    if (success) {
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    }
  }

  return (
    <div className="flex items-center gap-2 mt-1">
      <span
        className="text-xs font-mono truncate"
        style={{ color: '#848E9C' }}
        title={address}
      >
        {displayAddress}
      </span>
      <div className="flex items-center gap-1 flex-shrink-0">
        <button
          type="button"
          onClick={() => setIsVisible(!isVisible)}
          className="p-1 rounded transition-colors hover:bg-gray-700"
          style={{ color: '#848E9C' }}
          aria-label={isVisible ? t('hideAddress', language) : t('showAddress', language)}
          title={isVisible ? t('hideAddress', language) : t('showAddress', language)}
        >
          {isVisible ? (
            <EyeOff className="w-3 h-3" />
          ) : (
            <Eye className="w-3 h-3" />
          )}
        </button>
        <button
          type="button"
          onClick={handleCopy}
          className="p-1 rounded transition-colors hover:bg-gray-700"
          style={{ color: copied ? 'var(--green-primary)' : '#848E9C' }}
          aria-label={t('copyAddress', language)}
          title={t('copyAddress', language)}
        >
          {copied ? (
            <Check className="w-3 h-3" />
          ) : (
            <Copy className="w-3 h-3" />
          )}
        </button>
      </div>
    </div>
  )
}
