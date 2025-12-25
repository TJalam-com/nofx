import React, { useState, useEffect } from 'react'
import type { Exchange } from '../../types'
import { t, type Language } from '../../i18n/translations'
import { api } from '../../lib/api'
import { getExchangeIcon } from '../ExchangeIcons'
import {
  TwoStageKeyModal,
  type TwoStageKeyModalResult,
} from '../TwoStageKeyModal'
import {
  WebCryptoEnvironmentCheck,
  type WebCryptoCheckStatus,
} from '../WebCryptoEnvironmentCheck'
import { BookOpen, Trash2, HelpCircle } from 'lucide-react'
import { toast } from 'sonner'
import { Tooltip } from './Tooltip'
import { getShortName } from './utils'

interface ExchangeConfigModalProps {
  allExchanges: Exchange[]
  editingExchangeId: string | null
  onSave: (
    exchangeId: string,
    apiKey: string,
    secretKey?: string,
    testnet?: boolean,
    hyperliquidWalletAddr?: string,
    asterUser?: string,
    asterSigner?: string,
    asterPrivateKey?: string,
    okxPassphrase?: string,
    lighterWalletAddr?: string,
    lighterAPIKeyPrivateKey?: string,
    lighterAPIKeyIndex?: number
  ) => Promise<void>
  onDelete: (exchangeId: string) => void
  onClose: () => void
  language: Language
}

export function ExchangeConfigModal({
  allExchanges,
  editingExchangeId,
  onSave,
  onDelete,
  onClose,
  language,
}: ExchangeConfigModalProps) {
  const [selectedExchangeId, setSelectedExchangeId] = useState(
    editingExchangeId || ''
  )
  const [apiKey, setApiKey] = useState('')
  const [secretKey, setSecretKey] = useState('')
  const [passphrase, setPassphrase] = useState('')
  const [testnet, setTestnet] = useState(false)
  const [showGuide, setShowGuide] = useState(false)
  const [serverIP, setServerIP] = useState<{
    public_ip: string
    message: string
  } | null>(null)
  const [loadingIP, setLoadingIP] = useState(false)
  const [copiedIP, setCopiedIP] = useState(false)
  const [webCryptoStatus, setWebCryptoStatus] =
    useState<WebCryptoCheckStatus>('idle')
  const [isLoading, setIsLoading] = useState(false)

  // Binance configuration guide expanded state
  const [showBinanceGuide, setShowBinanceGuide] = useState(false)

  // Aster specific fields
  const [asterUser, setAsterUser] = useState('')
  const [asterSigner, setAsterSigner] = useState('')
  const [asterPrivateKey, setAsterPrivateKey] = useState('')

  // Hyperliquid specific fields
  const [hyperliquidWalletAddr, setHyperliquidWalletAddr] = useState('')

  // Lighter specific fields
  const [lighterWalletAddr, setLighterWalletAddr] = useState('')
  const [lighterAPIKeyPrivateKey, setLighterAPIKeyPrivateKey] = useState('')
  const [lighterAPIKeyIndex, setLighterAPIKeyIndex] = useState(0)

  // Secure input state
  const [secureInputTarget, setSecureInputTarget] = useState<
    null | 'hyperliquid' | 'aster' | 'lighter'
  >(null)

  // Get current editing exchange information
  const selectedExchange = allExchanges?.find(
    (e) => e.id === selectedExchangeId
  )

  // If editing existing exchange, initialize form data
  useEffect(() => {
    if (editingExchangeId && selectedExchange) {
      setApiKey(selectedExchange.apiKey || '')
      setSecretKey(selectedExchange.secretKey || '')
      setPassphrase('') // Don't load existing passphrase for security
      setTestnet(selectedExchange.testnet || false)

      // Aster fields
      setAsterUser(selectedExchange.asterUser || '')
      setAsterSigner(selectedExchange.asterSigner || '')
      setAsterPrivateKey('') // Don't load existing private key for security

      // Hyperliquid fields
      setHyperliquidWalletAddr(selectedExchange.hyperliquidWalletAddr || '')

      // Lighter fields
      setLighterWalletAddr(selectedExchange.lighterWalletAddr || '')
      setLighterAPIKeyPrivateKey('') // Don't load existing private key for security
      setLighterAPIKeyIndex(selectedExchange.lighterAPIKeyIndex || 0)
    }
  }, [editingExchangeId, selectedExchange])

  // Load server IP (when binance is selected)
  useEffect(() => {
    if (selectedExchangeId === 'binance' && !serverIP) {
      setLoadingIP(true)
      api
        .getServerIP()
        .then((data) => {
          setServerIP(data)
        })
        .catch((err) => {
          console.error('Failed to load server IP:', err)
        })
        .finally(() => {
          setLoadingIP(false)
        })
    }
  }, [selectedExchangeId])

  const handleCopyIP = async (ip: string) => {
    try {
      // Prefer modern Clipboard API
      if (navigator.clipboard && navigator.clipboard.writeText) {
        await navigator.clipboard.writeText(ip)
        setCopiedIP(true)
        setTimeout(() => setCopiedIP(false), 2000)
        toast.success(t('ipCopied', language))
      } else {
        // Fallback: use traditional execCommand method
        const textArea = document.createElement('textarea')
        textArea.value = ip
        textArea.style.position = 'fixed'
        textArea.style.left = '-999999px'
        textArea.style.top = '-999999px'
        document.body.appendChild(textArea)
        textArea.focus()
        textArea.select()

        try {
          const successful = document.execCommand('copy')
          if (successful) {
            setCopiedIP(true)
            setTimeout(() => setCopiedIP(false), 2000)
            toast.success(t('ipCopied', language))
          } else {
            throw new Error('Copy command execution failed')
          }
        } finally {
          document.body.removeChild(textArea)
        }
      }
    } catch (err) {
      console.error('Failed to copy:', err)
      // Show error message
      toast.error(
        t('copyIPFailed', language) || `Failed to copy: ${ip}\nPlease manually copy this IP address`
      )
    }
  }

  // Secure input handler function
  const secureInputContextLabel =
    secureInputTarget === 'aster'
      ? t('asterExchangeName', language)
      : secureInputTarget === 'hyperliquid'
        ? t('hyperliquidExchangeName', language)
        : undefined

  const handleSecureInputCancel = () => {
    setSecureInputTarget(null)
  }

  const handleSecureInputComplete = ({
    value,
    obfuscationLog,
  }: TwoStageKeyModalResult) => {
    const trimmed = value.trim()
    if (secureInputTarget === 'hyperliquid') {
      setApiKey(trimmed)
    }
    if (secureInputTarget === 'aster') {
      setAsterPrivateKey(trimmed)
    }
    // Only output debug information in development environment
    if (import.meta.env.DEV) {
      console.log('Secure input obfuscation log:', obfuscationLog)
    }
    setSecureInputTarget(null)
  }

  // Mask sensitive data display
  const maskSecret = (secret: string) => {
    if (!secret || secret.length === 0) return ''
    if (secret.length <= 8) return '*'.repeat(secret.length)
    return (
      secret.slice(0, 4) +
      '*'.repeat(Math.max(secret.length - 8, 4)) +
      secret.slice(-4)
    )
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!selectedExchangeId || isLoading) return

    setIsLoading(true)
    try {
      // Validate different fields based on exchange type
      if (selectedExchange?.id === 'binance') {
        if (!apiKey.trim() || !secretKey.trim()) {
          setIsLoading(false)
          return
        }
        await onSave(selectedExchangeId, apiKey.trim(), secretKey.trim(), testnet, undefined, undefined, undefined, undefined, undefined, undefined, undefined, undefined)
      } else if (selectedExchange?.id === 'hyperliquid') {
        if (!apiKey.trim() || !hyperliquidWalletAddr.trim()) {
          setIsLoading(false)
          return
        }
        await onSave(
          selectedExchangeId,
          apiKey.trim(),
          '',
          testnet,
          hyperliquidWalletAddr.trim(),
          undefined,
          undefined,
          undefined,
          undefined,
          undefined,
          undefined,
          undefined
        )
      } else if (selectedExchange?.id === 'aster') {
        if (!asterUser.trim() || !asterSigner.trim() || !asterPrivateKey.trim()) {
          setIsLoading(false)
          return
        }
        await onSave(
          selectedExchangeId,
          '',
          '',
          testnet,
          undefined,
          asterUser.trim(),
          asterSigner.trim(),
          asterPrivateKey.trim(),
          undefined,
          undefined,
          undefined,
          undefined
        )
      } else if (selectedExchange?.id === 'lighter') {
        if (!lighterWalletAddr.trim() || !lighterAPIKeyPrivateKey.trim()) {
          setIsLoading(false)
          return
        }
        // #region agent log
        const trimmedWalletAddr = lighterWalletAddr.trim()
        const trimmedAPIKeyPrivateKey = lighterAPIKeyPrivateKey.trim()
        console.log('🔍 ExchangeConfigModal: Calling onSave for Lighter', {
          exchangeId: selectedExchangeId,
          lighterWalletAddr: trimmedWalletAddr,
          lighterWalletAddr_length: trimmedWalletAddr.length,
          lighterAPIKeyPrivateKey_len: trimmedAPIKeyPrivateKey.length,
          lighterAPIKeyIndex: lighterAPIKeyIndex,
          lighterAPIKeyIndex_type: typeof lighterAPIKeyIndex,
          parameterOrder: {
            pos1: 'selectedExchangeId',
            pos2: "'' (apiKey)",
            pos3: "'' (secretKey)",
            pos4: 'testnet',
            pos5: 'undefined (hyperliquidWalletAddr)',
            pos6: 'undefined (asterUser)',
            pos7: 'undefined (asterSigner)',
            pos8: 'undefined (asterPrivateKey)',
            pos9: 'undefined (okxPassphrase)',
            pos10: 'lighterWalletAddr.trim()',
            pos11: 'lighterAPIKeyPrivateKey.trim()',
            pos12: 'lighterAPIKeyIndex'
          }
        })
        await onSave(
          selectedExchangeId,
          '',
          '',
          testnet,
          undefined,
          undefined,
          undefined,
          undefined,
          undefined,
          trimmedWalletAddr,
          trimmedAPIKeyPrivateKey,
          lighterAPIKeyIndex
        )
      } else if (selectedExchange?.id === 'okx') {
        if (!apiKey.trim() || !secretKey.trim() || !passphrase.trim()) {
          setIsLoading(false)
          return
        }
        await onSave(selectedExchangeId, apiKey.trim(), secretKey.trim(), testnet, undefined, undefined, undefined, undefined, passphrase.trim(), undefined, undefined, undefined)
      } else if (selectedExchange?.id === 'bitget') {
        if (!apiKey.trim() || !secretKey.trim() || !passphrase.trim()) {
          setIsLoading(false)
          return
        }
        await onSave(selectedExchangeId, apiKey.trim(), secretKey.trim(), testnet, undefined, undefined, undefined, undefined, passphrase.trim(), undefined, undefined, undefined)
      } else {
        // Default case (other CEX exchanges)
        if (!apiKey.trim() || !secretKey.trim()) {
          setIsLoading(false)
          return
        }
        await onSave(selectedExchangeId, apiKey.trim(), secretKey.trim(), testnet, undefined, undefined, undefined, undefined, undefined, undefined, undefined, undefined)
      }
    } catch (error) {
      // Error handling is done in parent component
    } finally {
      setIsLoading(false)
    }
  }

  // Available exchange list (all supported exchanges)
  const availableExchanges = allExchanges || []

  return (
    <div className="fixed inset-0 flex items-center justify-center z-50 p-4 overflow-y-auto" style={{ background: 'rgba(0, 31, 63, 0.5)' }}>
      <div
        className="bg-gray-800 rounded-lg w-full max-w-lg relative my-8"
        style={{
          background: 'var(--navy-dark)',
          maxHeight: 'calc(100vh - 4rem)',
        }}
      >
        <div
          className="flex items-center justify-between p-6 pb-4 sticky top-0 z-10"
          style={{ background: 'var(--navy-dark)' }}
        >
          <h3 className="text-xl font-bold" style={{ color: '#EAECEF' }}>
            {editingExchangeId
              ? t('editExchange', language)
              : t('addExchange', language)}
          </h3>
          <div className="flex items-center gap-2">
            {selectedExchange?.id === 'binance' && (
              <button
                type="button"
                onClick={() => setShowGuide(true)}
                className="px-3 py-2 rounded text-sm font-semibold transition-all hover:scale-105 flex items-center gap-2"
                style={{
                  background: 'rgba(0, 255, 127, 0.1)',
                  color: 'var(--green-primary)',
                }}
              >
                <BookOpen className="w-4 h-4" />
                {t('viewGuide', language)}
              </button>
            )}
            {editingExchangeId && (
              <button
                type="button"
                onClick={() => onDelete(editingExchangeId)}
                className="p-2 rounded hover:bg-red-100 transition-colors"
                style={{
                  background: 'rgba(246, 70, 93, 0.1)',
                  color: '#F6465D',
                }}
                title={t('delete', language)}
              >
                <Trash2 className="w-4 h-4" />
              </button>
            )}
          </div>
        </div>

        <form onSubmit={handleSubmit} className="px-6 pb-6">
          <div
            className="space-y-4 overflow-y-auto"
            style={{ maxHeight: 'calc(100vh - 16rem)' }}
          >
            {!editingExchangeId && (
              <div className="space-y-3">
                <div className="space-y-2">
                  <div
                    className="text-xs font-semibold uppercase tracking-wide"
                    style={{ color: 'var(--green-primary)' }}
                  >
                    {t('environmentSteps.checkTitle', language)}
                  </div>
                  <WebCryptoEnvironmentCheck
                    language={language}
                    variant="card"
                    onStatusChange={setWebCryptoStatus}
                  />
                </div>
                <div className="space-y-2">
                  <div
                    className="text-xs font-semibold uppercase tracking-wide"
                    style={{ color: 'var(--green-primary)' }}
                  >
                    {t('environmentSteps.selectTitle', language)}
                  </div>
                  <select
                    value={selectedExchangeId}
                    onChange={(e) => setSelectedExchangeId(e.target.value)}
                    className="w-full px-3 py-2 rounded"
                    style={{
                      background: 'var(--navy-primary)',
                      border: '1px solid var(--panel-border)',
                      color: '#EAECEF',
                    }}
                    aria-label={t('selectExchange', language)}
                    disabled={webCryptoStatus !== 'secure' && webCryptoStatus !== 'disabled'}
                    required
                  >
                    <option value="">
                      {t('pleaseSelectExchange', language)}
                    </option>
                    {availableExchanges.map((exchange) => (
                      <option key={exchange.id} value={exchange.id}>
                        {getShortName(exchange.name)} (
                        {exchange.type.toUpperCase()})
                      </option>
                    ))}
                  </select>
                </div>
              </div>
            )}

            {selectedExchange && (
              <div
                className="p-4 rounded"
                style={{ background: 'var(--navy-primary)', border: '1px solid var(--panel-border)' }}
              >
                <div className="flex items-center gap-3 mb-3">
                  <div className="w-8 h-8 flex items-center justify-center">
                    {getExchangeIcon(selectedExchange.id, {
                      width: 32,
                      height: 32,
                    })}
                  </div>
                  <div>
                    <div className="font-semibold" style={{ color: '#EAECEF' }}>
                      {getShortName(selectedExchange.name)}
                    </div>
                    <div className="text-xs" style={{ color: '#848E9C' }}>
                      {selectedExchange.type.toUpperCase()} •{' '}
                      {selectedExchange.id}
                    </div>
                  </div>
                </div>
              </div>
            )}

            {selectedExchange && (
              <>
                {/* Common fields for Binance/Bybit/OKX/Bitget */}
                {(selectedExchange.id === 'binance' ||
                  selectedExchange.id === 'bybit' ||
                  selectedExchange.id === 'okx' ||
                  selectedExchange.id === 'bitget') && (
                    <>
                      {/* Binance user configuration guide (D1 solution) */}
                      {selectedExchange.id === 'binance' && (
                        <div
                          className="mb-4 p-3 rounded cursor-pointer transition-colors"
                          style={{
                            background: '#1a3a52',
                            border: '1px solid #2b5278',
                          }}
                          onClick={() => setShowBinanceGuide(!showBinanceGuide)}
                        >
                          <div className="flex items-center justify-between">
                            <div className="flex items-center gap-2">
                              <span style={{ color: '#58a6ff' }}>ℹ️</span>
                              <span
                                className="text-sm font-medium"
                                style={{ color: '#EAECEF' }}
                              >
                                <strong>Binance Users Must Read:</strong>
                                Use the 'Spot and Futures Trading' API, do not use the 'Unified Account
                                API'
                              </span>
                            </div>
                            <span style={{ color: '#8b949e' }}>
                              {showBinanceGuide ? '▲' : '▼'}
                            </span>
                          </div>

                          {/* Expanded detailed instructions */}
                          {showBinanceGuide && (
                            <div
                              className="mt-3 pt-3"
                              style={{
                                borderTop: '1px solid #2b5278',
                                fontSize: '0.875rem',
                                color: '#c9d1d9',
                              }}
                              onClick={(e) => e.stopPropagation()}
                            >
                              <p className="mb-2" style={{ color: '#8b949e' }}>
                                <strong>Reason:</strong> The Unified Account API
                                has a different permission structure, which will cause order submission to fail
                              </p>

                              <p
                                className="font-semibold mb-1"
                                style={{ color: '#EAECEF' }}
                              >
                                Correct Configuration Steps:
                              </p>
                              <ol
                                className="list-decimal list-inside space-y-1 mb-3"
                                style={{ paddingLeft: '0.5rem' }}
                              >
                                <li>
                                  Log in to Binance → Personal Center →{' '}
                                  <strong>API Management</strong>
                                </li>
                                <li>
                                  Create API → Select '
                                  <strong>System-generated API Key</strong>'
                                </li>
                                <li>
                                  Check '<strong>Spot and Futures Trading</strong>' (
                                  <span style={{ color: '#f85149' }}>
                                    do not select Unified Account
                                  </span>
                                  )
                                </li>
                                <li>
                                  IP Restriction: Select '<strong>Unrestricted</strong>
                                  ' or add server IP
                                </li>
                              </ol>

                              <p
                                className="mb-2 p-2 rounded"
                                style={{
                                  background: '#3d2a00',
                                  border: '1px solid #9e6a03',
                                }}
                              >
                                💡 <strong>Multi-Asset Mode Users Note:</strong>
                                If you have enabled Multi-Asset Mode, it will force the use of Cross Margin mode. It is recommended to disable Multi-Asset Mode to support Isolated Margin trading.
                              </p>

                              <a
                                href="https://www.binance.com/en/support/faq/how-to-create-api-keys-on-binance-360002502072"
                                target="_blank"
                                rel="noopener noreferrer"
                                className="inline-block text-sm hover:underline"
                                style={{ color: '#58a6ff' }}
                              >
                                📖 View Binance Official Tutorial ↗
                              </a>
                            </div>
                          )}
                        </div>
                      )}

                      <div>
                        <label
                          className="block text-sm font-semibold mb-2"
                          style={{ color: '#EAECEF' }}
                        >
                          {t('apiKey', language)}
                        </label>
                        <input
                          type="password"
                          value={apiKey}
                          onChange={(e) => setApiKey(e.target.value)}
                          placeholder={t('enterAPIKey', language)}
                          className="w-full px-3 py-2 rounded"
                          style={{
                            background: 'var(--navy-primary)',
                            border: '1px solid var(--panel-border)',
                            color: '#EAECEF',
                          }}
                          required
                        />
                      </div>

                      <div>
                        <label
                          className="block text-sm font-semibold mb-2"
                          style={{ color: '#EAECEF' }}
                        >
                          {t('secretKey', language)}
                        </label>
                        <input
                          type="password"
                          value={secretKey}
                          onChange={(e) => setSecretKey(e.target.value)}
                          placeholder={t('enterSecretKey', language)}
                          className="w-full px-3 py-2 rounded"
                          style={{
                            background: 'var(--navy-primary)',
                            border: '1px solid var(--panel-border)',
                            color: '#EAECEF',
                          }}
                          required
                        />
                      </div>

                      {(selectedExchange.id === 'okx' || selectedExchange.id === 'bitget') && (
                        <div>
                          <label
                            className="block text-sm font-semibold mb-2"
                            style={{ color: '#EAECEF' }}
                          >
                            {t('passphrase', language)}
                          </label>
                          <input
                            type="password"
                            value={passphrase}
                            onChange={(e) => setPassphrase(e.target.value)}
                            placeholder={t('enterPassphrase', language)}
                            className="w-full px-3 py-2 rounded"
                            style={{
                              background: 'var(--navy-primary)',
                              border: '1px solid var(--panel-border)',
                              color: '#EAECEF',
                            }}
                            required
                          />
                        </div>
                      )}

                      {/* Binance whitelist IP prompt */}
                      {selectedExchange.id === 'binance' && (
                        <div
                          className="p-4 rounded"
                          style={{
                            background: 'rgba(0, 255, 127, 0.1)',
                            border: '1px solid rgba(0, 255, 127, 0.2)',
                          }}
                        >
                          <div
                            className="text-sm font-semibold mb-2"
                            style={{ color: 'var(--green-primary)' }}
                          >
                            {t('whitelistIP', language)}
                          </div>
                          <div
                            className="text-xs mb-3"
                            style={{ color: '#848E9C' }}
                          >
                            {t('whitelistIPDesc', language)}
                          </div>

                          {loadingIP ? (
                            <div
                              className="text-xs"
                              style={{ color: '#848E9C' }}
                            >
                              {t('loadingServerIP', language)}
                            </div>
                          ) : serverIP && serverIP.public_ip ? (
                            <div
                              className="flex items-center gap-2 p-2 rounded"
                              style={{ background: 'var(--navy-primary)' }}
                            >
                              <code
                                className="flex-1 text-sm font-mono"
                                style={{ color: 'var(--green-primary)' }}
                              >
                                {serverIP.public_ip}
                              </code>
                              <button
                                type="button"
                                onClick={() => handleCopyIP(serverIP.public_ip)}
                                className="px-3 py-1 rounded text-xs font-semibold transition-all hover:scale-105"
                                style={{
                                  background: 'rgba(0, 255, 127, 0.2)',
                                  color: 'var(--green-primary)',
                                }}
                              >
                                {copiedIP
                                  ? t('ipCopied', language)
                                  : t('copyIP', language)}
                              </button>
                            </div>
                          ) : null}
                        </div>
                      )}
                    </>
                  )}

                {/* Aster exchange fields */}
                {selectedExchange.id === 'aster' && (
                  <>
                    <div>
                      <label
                        className="block text-sm font-semibold mb-2 flex items-center gap-2"
                        style={{ color: '#EAECEF' }}
                      >
                        {t('user', language)}
                        <Tooltip content={t('asterUserDesc', language)}>
                          <HelpCircle
                            className="w-4 h-4 cursor-help"
                            style={{ color: 'var(--green-primary)' }}
                          />
                        </Tooltip>
                      </label>
                      <input
                        type="text"
                        value={asterUser}
                        onChange={(e) => setAsterUser(e.target.value)}
                        placeholder={t('enterUser', language)}
                        className="w-full px-3 py-2 rounded"
                        style={{
                          background: 'var(--navy-primary)',
                          border: '1px solid var(--panel-border)',
                          color: '#EAECEF',
                        }}
                        required
                      />
                    </div>

                    <div>
                      <label
                        className="block text-sm font-semibold mb-2 flex items-center gap-2"
                        style={{ color: '#EAECEF' }}
                      >
                        {t('signer', language)}
                        <Tooltip content={t('asterSignerDesc', language)}>
                          <HelpCircle
                            className="w-4 h-4 cursor-help"
                            style={{ color: 'var(--green-primary)' }}
                          />
                        </Tooltip>
                      </label>
                      <input
                        type="text"
                        value={asterSigner}
                        onChange={(e) => setAsterSigner(e.target.value)}
                        placeholder={t('enterSigner', language)}
                        className="w-full px-3 py-2 rounded"
                        style={{
                          background: 'var(--navy-primary)',
                          border: '1px solid var(--panel-border)',
                          color: '#EAECEF',
                        }}
                        required
                      />
                    </div>

                    <div>
                      <label
                        className="block text-sm font-semibold mb-2 flex items-center gap-2"
                        style={{ color: '#EAECEF' }}
                      >
                        {t('privateKey', language)}
                        <Tooltip content={t('asterPrivateKeyDesc', language)}>
                          <HelpCircle
                            className="w-4 h-4 cursor-help"
                            style={{ color: 'var(--green-primary)' }}
                          />
                        </Tooltip>
                      </label>
                      <input
                        type="password"
                        value={asterPrivateKey}
                        onChange={(e) => setAsterPrivateKey(e.target.value)}
                        placeholder={t('enterPrivateKey', language)}
                        className="w-full px-3 py-2 rounded"
                        style={{
                          background: 'var(--navy-primary)',
                          border: '1px solid var(--panel-border)',
                          color: '#EAECEF',
                        }}
                        required
                      />
                    </div>
                  </>
                )}

                {/* Lighter exchange fields */}
                {selectedExchange.id === 'lighter' && (
                  <>
                    {/* Security warning banner */}
                    <div
                      className="p-3 rounded mb-4"
                      style={{
                        background: 'rgba(0, 255, 127, 0.1)',
                        border: '1px solid rgba(0, 255, 127, 0.3)',
                      }}
                    >
                      <div className="flex items-start gap-2">
                        <span style={{ color: 'var(--green-primary)', fontSize: '16px' }}>
                          🔐
                        </span>
                        <div className="flex-1">
                          <div
                            className="text-sm font-semibold mb-1"
                            style={{ color: 'var(--green-primary)' }}
                          >
                            Lighter Agent Wallet
                          </div>
                          <div
                            className="text-xs"
                            style={{ color: '#848E9C', lineHeight: '1.5' }}
                          >
                            Use Agent Wallet for secure trading. Never expose your main wallet private key.
                          </div>
                        </div>
                      </div>
                    </div>

                    {/* Wallet Address field */}
                    <div>
                      <label
                        className="block text-sm font-semibold mb-2"
                        style={{ color: '#EAECEF' }}
                      >
                        Wallet Address
                      </label>
                      <input
                        type="text"
                        value={lighterWalletAddr}
                        onChange={(e) => setLighterWalletAddr(e.target.value)}
                        placeholder="0x..."
                        className="w-full px-3 py-2 rounded"
                        style={{
                          background: 'var(--navy-primary)',
                          border: '1px solid var(--panel-border)',
                          color: '#EAECEF',
                        }}
                        required
                      />
                    </div>

                    {/* API Key Private Key field */}
                    <div>
                      <label
                        className="block text-sm font-semibold mb-2"
                        style={{ color: '#EAECEF' }}
                      >
                        API Key Private Key
                      </label>
                      <input
                        type="password"
                        value={lighterAPIKeyPrivateKey}
                        onChange={(e) => setLighterAPIKeyPrivateKey(e.target.value)}
                        placeholder="Enter API key private key"
                        className="w-full px-3 py-2 rounded"
                        style={{
                          background: 'var(--navy-primary)',
                          border: '1px solid var(--panel-border)',
                          color: '#EAECEF',
                        }}
                        required
                      />
                    </div>

                    {/* API Key Index field */}
                    <div>
                      <label
                        className="block text-sm font-semibold mb-2"
                        style={{ color: '#EAECEF' }}
                      >
                        API Key Index (0-254)
                      </label>
                      <input
                        type="number"
                        value={lighterAPIKeyIndex}
                        onChange={(e) => setLighterAPIKeyIndex(parseInt(e.target.value) || 0)}
                        placeholder="0"
                        min="0"
                        max="254"
                        className="w-full px-3 py-2 rounded"
                        style={{
                          background: 'var(--navy-primary)',
                          border: '1px solid var(--panel-border)',
                          color: '#EAECEF',
                        }}
                      />
                    </div>
                  </>
                )}

                {/* Hyperliquid exchange fields */}
                {selectedExchange.id === 'hyperliquid' && (
                  <>
                    {/* Security warning banner */}
                    <div
                      className="p-3 rounded mb-4"
                      style={{
                        background: 'rgba(0, 255, 127, 0.1)',
                        border: '1px solid rgba(0, 255, 127, 0.3)',
                      }}
                    >
                      <div className="flex items-start gap-2">
                        <span style={{ color: 'var(--green-primary)', fontSize: '16px' }}>
                          🔐
                        </span>
                        <div className="flex-1">
                          <div
                            className="text-sm font-semibold mb-1"
                            style={{ color: 'var(--green-primary)' }}
                          >
                            {t('hyperliquidAgentWalletTitle', language)}
                          </div>
                          <div
                            className="text-xs"
                            style={{ color: '#848E9C', lineHeight: '1.5' }}
                          >
                            {t('hyperliquidAgentWalletDesc', language)}
                          </div>
                        </div>
                      </div>
                    </div>

                    {/* Agent Private Key field */}
                    <div>
                      <label
                        className="block text-sm font-semibold mb-2"
                        style={{ color: '#EAECEF' }}
                      >
                        {t('hyperliquidAgentPrivateKey', language)}
                      </label>
                      <div className="flex flex-col gap-2">
                        <div className="flex gap-2">
                          <input
                            type="text"
                            value={maskSecret(apiKey)}
                            readOnly
                            placeholder={t(
                              'enterHyperliquidAgentPrivateKey',
                              language
                            )}
                            className="w-full px-3 py-2 rounded"
                            style={{
                              background: 'var(--navy-primary)',
                              border: '1px solid var(--panel-border)',
                              color: '#EAECEF',
                            }}
                          />
                          <button
                            type="button"
                            onClick={() => setSecureInputTarget('hyperliquid')}
                            className="px-3 py-2 rounded text-xs font-semibold transition-all hover:scale-105"
                            style={{
                              background: 'var(--green-primary)',
                              color: 'var(--navy-primary)',
                              whiteSpace: 'nowrap',
                            }}
                          >
                            {apiKey
                              ? t('secureInputReenter', language)
                              : t('secureInputButton', language)}
                          </button>
                          {apiKey && (
                            <button
                              type="button"
                              onClick={() => setApiKey('')}
                              className="px-3 py-2 rounded text-xs font-semibold transition-all hover:scale-105"
                              style={{
                                background: '#1B1F2B',
                                color: '#848E9C',
                                whiteSpace: 'nowrap',
                              }}
                            >
                              {t('secureInputClear', language)}
                            </button>
                          )}
                        </div>
                        {apiKey && (
                          <div className="text-xs" style={{ color: '#848E9C' }}>
                            {t('secureInputHint', language)}
                          </div>
                        )}
                      </div>
                      <div
                        className="text-xs mt-1"
                        style={{ color: '#848E9C' }}
                      >
                        {t('hyperliquidAgentPrivateKeyDesc', language)}
                      </div>
                    </div>

                    {/* Main Wallet Address field */}
                    <div>
                      <label
                        className="block text-sm font-semibold mb-2"
                        style={{ color: '#EAECEF' }}
                      >
                        {t('hyperliquidMainWalletAddress', language)}
                      </label>
                      <input
                        type="text"
                        value={hyperliquidWalletAddr}
                        onChange={(e) =>
                          setHyperliquidWalletAddr(e.target.value)
                        }
                        placeholder={t(
                          'enterHyperliquidMainWalletAddress',
                          language
                        )}
                        className="w-full px-3 py-2 rounded"
                        style={{
                          background: 'var(--navy-primary)',
                          border: '1px solid var(--panel-border)',
                          color: '#EAECEF',
                        }}
                        required
                      />
                      <div
                        className="text-xs mt-1"
                        style={{ color: '#848E9C' }}
                      >
                        {t('hyperliquidMainWalletAddressDesc', language)}
                      </div>
                    </div>
                  </>
                )}

              </>
            )}
          </div>

          <div
            className="flex gap-3 mt-6 pt-4 sticky bottom-0"
            style={{ background: 'var(--navy-dark)' }}
          >
            <button
              type="button"
              onClick={onClose}
              className="flex-1 px-4 py-2 rounded text-sm font-semibold"
              style={{ background: 'var(--navy-light)', color: '#848E9C' }}
            >
              {t('cancel', language)}
            </button>
            <button
              type="submit"
              disabled={
                isLoading ||
                !selectedExchange ||
                (selectedExchange.id === 'binance' &&
                  (!apiKey.trim() || !secretKey.trim())) ||
                (selectedExchange.id === 'okx' &&
                  (!apiKey.trim() ||
                    !secretKey.trim() ||
                    !passphrase.trim())) ||
                (selectedExchange.id === 'bitget' &&
                  (!apiKey.trim() ||
                    !secretKey.trim() ||
                    !passphrase.trim())) ||
                (selectedExchange.id === 'hyperliquid' &&
                  (!apiKey.trim() || !hyperliquidWalletAddr.trim())) || // Validate private key and wallet address
                (selectedExchange.id === 'aster' &&
                  (!asterUser.trim() ||
                    !asterSigner.trim() ||
                    !asterPrivateKey.trim())) ||
                (selectedExchange.id === 'lighter' &&
                  (!lighterWalletAddr.trim() ||
                    !lighterAPIKeyPrivateKey.trim())) ||
                (selectedExchange.id === 'bybit' &&
                  (!apiKey.trim() || !secretKey.trim())) ||
                (selectedExchange.type === 'cex' &&
                  selectedExchange.id !== 'hyperliquid' &&
                  selectedExchange.id !== 'aster' &&
                  selectedExchange.id !== 'lighter' &&
                  selectedExchange.id !== 'binance' &&
                  selectedExchange.id !== 'bybit' &&
                  selectedExchange.id !== 'okx' &&
                  (!apiKey.trim() || !secretKey.trim()))
              }
              className="flex-1 px-4 py-2 rounded text-sm font-semibold disabled:opacity-50"
              style={{ background: 'var(--green-primary)', color: 'var(--navy-primary)' }}
            >
              {isLoading ? t('saving', language) || 'Saving...' : t('saveConfig', language)}
            </button>
          </div>
        </form>
      </div>

      {/* Binance Setup Guide Modal */}
      {showGuide && (
        <div
          className="fixed inset-0 flex items-center justify-center z-50 p-4"
          style={{ background: 'rgba(0, 31, 63, 0.75)' }}
          onClick={() => setShowGuide(false)}
        >
          <div
            className="bg-gray-800 rounded-lg p-6 w-full max-w-4xl relative"
            style={{ background: 'var(--navy-dark)' }}
            onClick={(e) => e.stopPropagation()}
          >
            <div className="flex items-center justify-between mb-4">
              <h3
                className="text-xl font-bold flex items-center gap-2"
                style={{ color: '#EAECEF' }}
              >
                <BookOpen className="w-6 h-6" style={{ color: 'var(--green-primary)' }} />
                {t('binanceSetupGuide', language)}
              </h3>
              <button
                onClick={() => setShowGuide(false)}
                className="px-4 py-2 rounded text-sm font-semibold transition-all hover:scale-105"
                style={{ background: 'var(--navy-light)', color: '#848E9C' }}
              >
                {t('closeGuide', language)}
              </button>
            </div>
            <div className="overflow-y-auto max-h-[80vh]">
              <picture>
                <source srcSet="/images/guide.webp" type="image/webp" />
                <img
                  src="/images/guide.png"
                  alt={t('binanceSetupGuide', language)}
                  className="w-full h-auto rounded"
                  loading="lazy"
                  decoding="async"
                />
              </picture>
            </div>
          </div>
        </div>
      )}

      {/* Two Stage Key Modal */}
      <TwoStageKeyModal
        isOpen={secureInputTarget !== null}
        language={language}
        contextLabel={secureInputContextLabel}
        expectedLength={64}
        onCancel={handleSecureInputCancel}
        onComplete={handleSecureInputComplete}
      />
    </div>
  )
}
