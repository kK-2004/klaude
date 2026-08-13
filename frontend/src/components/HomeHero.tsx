import { Bug, Cloud, FolderSync, Hammer, Telescope } from 'lucide-react'
import { useApp } from '../app/use-app'

const cards = [
  { icon: Telescope, color: 'blue', prompt: '探索并理解这个项目的代码结构。', label: '探索并理解代码' },
  { icon: Hammer, color: 'purple', prompt: '帮我在这个项目里构建一个新功能。', label: '构建新功能、应用或工具' },
  { icon: FolderSync, color: 'green', prompt: '审查当前代码并提出修改建议。', label: '审查代码并提出修改建议' },
  { icon: Bug, color: 'orange', prompt: '查找并修复现有问题和失败。', label: '修复问题和失败' },
] as const

export function HomeHero() {
  const { project, setComposer } = useApp()
  const name = project?.name ?? '这个工作区'

  return (
    <div className="home-hero">
      <div className="hero-mark" aria-hidden>
        <Cloud size={18} strokeWidth={1.75} />
      </div>
      <h1>要在 {name} 内开发什么？</h1>
      <div className="hero-cards">
        {cards.map((card) => {
          const Icon = card.icon
          return (
            <button type="button" key={card.label} className="hero-card" onClick={() => setComposer(card.prompt)}>
              <span className={`hero-card-icon ${card.color}`}><Icon size={18} strokeWidth={1.75} /></span>
              <span>{card.label}</span>
            </button>
          )
        })}
      </div>
    </div>
  )
}
