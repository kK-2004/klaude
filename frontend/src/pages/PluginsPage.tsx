import { CircleAlert, CircleCheck, Puzzle } from 'lucide-react'
import { useApp } from '../app/use-app'

export function PluginsPage() {
  const { capabilities } = useApp()
  const items = capabilities.length
    ? capabilities
    : [
      { name: 'git', available: true, detail: 'detected' },
      { name: 'rg', available: true, detail: 'detected' },
    ]

  return (
    <div className="page-plain">
      <header className="page-heading">
        <h1>插件</h1>
        <p>当前环境探测到的能力。不可用的项会标明原因。</p>
      </header>
      <ul className="plugin-list">
        {items.map((item) => (
          <li key={item.name} className={item.available ? 'ok' : 'missing'}>
            <Puzzle size={16} strokeWidth={1.75} />
            <strong>{item.name}</strong>
            <span>
              {item.available ? <CircleCheck size={14} strokeWidth={1.75} /> : <CircleAlert size={14} strokeWidth={1.75} />}
              {item.available ? '可用' : '不可用'}{item.detail ? ` · ${item.detail}` : ''}
            </span>
          </li>
        ))}
      </ul>
    </div>
  )
}
