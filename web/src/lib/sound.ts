/**
 * Sound system for playing audio feedback in the application
 * Provides methods for playing various sound effects
 */

class SoundSystem {
  private enabled: boolean = true
  private audioContext: AudioContext | null = null

  constructor() {
    // Initialize audio context lazily when first sound is played
    if (typeof window !== 'undefined' && 'AudioContext' in window) {
      try {
        this.audioContext = new (window.AudioContext || (window as any).webkitAudioContext)()
      } catch (e) {
        console.warn('AudioContext not supported, sounds will be disabled')
      }
    }
  }

  /**
   * Enable or disable sound effects
   */
  setEnabled(enabled: boolean): void {
    this.enabled = enabled
  }

  /**
   * Check if sound is enabled
   */
  isEnabled(): boolean {
    return this.enabled && this.audioContext !== null
  }

  /**
   * Play a simple beep sound
   */
  private playBeep(frequency: number, duration: number, volume: number = 0.1): void {
    if (!this.isEnabled()) return

    try {
      if (!this.audioContext) {
        this.audioContext = new (window.AudioContext || (window as any).webkitAudioContext)()
      }

      const oscillator = this.audioContext.createOscillator()
      const gainNode = this.audioContext.createGain()

      oscillator.connect(gainNode)
      gainNode.connect(this.audioContext.destination)

      oscillator.frequency.value = frequency
      oscillator.type = 'sine'

      gainNode.gain.setValueAtTime(0, this.audioContext.currentTime)
      gainNode.gain.linearRampToValueAtTime(volume, this.audioContext.currentTime + 0.01)
      gainNode.gain.exponentialRampToValueAtTime(0.01, this.audioContext.currentTime + duration)

      oscillator.start(this.audioContext.currentTime)
      oscillator.stop(this.audioContext.currentTime + duration)
    } catch (e) {
      // Silently fail if audio playback fails
      console.debug('Sound playback failed:', e)
    }
  }

  /**
   * Play notification sound
   */
  playNotification(): void {
    this.playBeep(800, 0.15, 0.15)
  }

  /**
   * Play chart switch sound
   */
  playChartSwitch(): void {
    this.playBeep(600, 0.1, 0.1)
  }

  /**
   * Play value change sound (positive or negative)
   */
  playValueChange(positive: boolean): void {
    if (positive) {
      // Upward sound
      this.playBeep(700, 0.1, 0.1)
    } else {
      // Downward sound
      this.playBeep(500, 0.1, 0.1)
    }
  }

  /**
   * Play animation complete sound
   */
  playAnimationComplete(): void {
    this.playBeep(1000, 0.2, 0.12)
  }
}

// Export singleton instance
export const soundSystem = new SoundSystem()
