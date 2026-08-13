import { Loader2 } from 'lucide-react'
import { useEffect, useRef } from 'react'
import { useApp } from '../app/use-app'
import { MarkdownView, ToolResult } from './MarkdownView'

export function MessageStream() {
  const { messages, running, approval } = useApp()
  const endRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    endRef.current?.scrollIntoView({ block: 'end' })
  }, [messages, running, approval])

  return (
    <div className="message-stream">
      {messages.map((message, index) => (
        <article className={`message ${message.role}`} key={message.id || `${message.role}-${index}`}>
          {message.role === 'user' ? (
            <div className="user-bubble">{message.content}</div>
          ) : message.role === 'tool' ? (
            <ToolResult content={message.content} />
          ) : (
            <MarkdownView content={message.content} />
          )}
        </article>
      ))}
      {running && !approval && (
        <div className="agent-status">
          <Loader2 className="spin" size={14} strokeWidth={1.75} />
          正在准备回复
        </div>
      )}
      <div ref={endRef} />
    </div>
  )
}
