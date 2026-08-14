export function createLocalID(prefix = 'local'): string {
  const cryptoAPI = globalThis.crypto

  if (typeof cryptoAPI?.randomUUID === 'function') {
    return `${prefix}-${cryptoAPI.randomUUID()}`
  }

  if (typeof cryptoAPI?.getRandomValues === 'function') {
    const bytes = cryptoAPI.getRandomValues(new Uint8Array(16))
    const suffix = Array.from(bytes, (byte) => byte.toString(16).padStart(2, '0')).join('')
    return `${prefix}-${suffix}`
  }

  return `${prefix}-${Date.now().toString(36)}-${Math.random().toString(36).slice(2)}`
}
