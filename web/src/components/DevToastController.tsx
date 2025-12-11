/// <reference types="vite/client" />

import { useState } from 'react'
import { confirmToast, notify } from '../lib/notify'

const toastOptions = [
  'message',
  'success',
  'info',
  'warning',
  'error',
  'custom',
] as const

type ToastType = (typeof toastOptions)[number]

const customRenderer = () => (
  <div className="dev-custom-toast">
    <p className="dev-custom-title">Sonner Custom Notification</p>
    <p className="dev-custom-body">
      This is a test Toast rendered via `notify.custom`
    </p>
  </div>
)

export function DevToastController() {
  const [type, setType] = useState<ToastType>('success')
  const [message, setMessage] = useState('Test notification from Dev controller')
  const [duration, setDuration] = useState(2200)

  if (!import.meta.env.DEV) {
    return null
  }

  const triggerToast = async () => {
    switch (type) {
      case 'message':
        notify.message(message, { duration })
        break
      case 'success':
        notify.success(message, { duration })
        break
      case 'info':
        notify.info(message, { duration })
        break
      case 'warning':
        notify.warning(message, { duration })
        break
      case 'error':
        notify.error(message, { duration })
        break
      case 'custom':
        notify.custom(() => customRenderer(), { duration })
        break
    }
  }

  const triggerConfirm = async () => {
    const confirmed = await confirmToast(message, {
      okText: 'Continue',
      cancelText: 'Cancel',
    })
    if (confirmed) {
      notify.success('Confirm button clicked', { duration: 2000 })
    } else {
      notify.message('Confirmation cancelled', { duration: 2000 })
    }
  }

  return (
    <div className="dev-toast-controller">
      <div className="dev-toast-controller__header">
        <span>Dev Sonner Controller</span>
        <small>Only visible in dev mode</small>
      </div>
      <div className="dev-toast-controller__content">
        <label className="dev-toast-controller__label">
          Type
          <select
            value={type}
            onChange={(event) => setType(event.target.value as ToastType)}
          >
            {toastOptions.map((option) => (
              <option key={option} value={option}>
                {option}
              </option>
            ))}
          </select>
        </label>
        <label className="dev-toast-controller__label">
          Message
          <input
            value={message}
            onChange={(event) => setMessage(event.target.value)}
            placeholder="Enter notification/confirmation message"
          />
        </label>
        <label className="dev-toast-controller__label">
          Duration (ms)
          <input
            type="number"
            min={600}
            value={duration}
            onChange={(event) => setDuration(Number(event.target.value))}
          />
        </label>
        <div className="dev-toast-controller__actions">
          <button onClick={triggerToast}>Trigger Notification</button>
          <button onClick={triggerConfirm}>Trigger Confirmation</button>
        </div>
      </div>
    </div>
  )
}

export default DevToastController
