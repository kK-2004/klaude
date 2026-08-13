import { Copy, Wrench } from 'lucide-react'
import { useEffect, useState } from 'react'
import ReactMarkdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { codeToHtml } from 'shiki'
import { boundToolOutput, safeLink } from '../lib/presentation'
import { useThemeStore } from '../stores/theme'

export function MarkdownView({ content }: { content: string }) {
  return (
    <div className="markdown">
      <ReactMarkdown
        remarkPlugins={[remarkGfm]}
        components={{
          a: ({ href, children }) => safeLink(href) ? <a href={href} target="_blank" rel="noreferrer">{children}</a> : <span>{children}</span>,
          code: ({ className, children }) => <HighlightedCode className={className} code={String(children).replace(/\n$/, '')} />,
        }}
      >
        {content}
      </ReactMarkdown>
    </div>
  )
}

function HighlightedCode({ className, code }: { className?: string; code: string }) {
  const [html, setHtml] = useState('')
  const theme = useThemeStore((state) => state.theme)
  const language = className?.replace('language-', '') || 'text'
  useEffect(() => {
    let active = true
    void codeToHtml(code, { lang: language, theme: theme === 'dark' ? 'github-dark' : 'github-light' })
      .then((value) => { if (active) setHtml(value) })
      .catch(() => undefined)
    return () => { active = false }
  }, [code, language, theme])
  return (
    <div className="code-block">
      <button type="button" className="copy-code" onClick={() => navigator.clipboard?.writeText(code)} aria-label="复制">
        <Copy size={13} strokeWidth={1.75} />
      </button>
      {html ? <div className="shiki" dangerouslySetInnerHTML={{ __html: html }} /> : <pre>{code}</pre>}
    </div>
  )
}

export function ToolResult({ content }: { content: string }) {
  const bounded = boundToolOutput(content)
  return (
    <div className="tool-card">
      <div className="tool-card-heading">
        <span><Wrench size={12} strokeWidth={1.75} /> 工具结果</span>
        <span>{content.length > 12000 ? '已截断' : '完成'}</span>
      </div>
      <MarkdownView content={bounded} />
    </div>
  )
}
