export function safeLink(href?: string) {
  if (!href) return false
  try {
    const base = typeof window === 'undefined' ? 'http://localhost' : window.location.origin
    const url = new URL(href, base)
    return url.protocol === 'https:' || url.protocol === 'http:' || url.protocol === 'mailto:'
  } catch {
    return false
  }
}

export function boundToolOutput(content: string, max = 12000) {
  return content.length > max ? `${content.slice(0, max)}\n… output truncated; see diagnostics for the full result` : content
}
