import { Loader2 } from 'lucide-react'

export function LoadingPage() {
  return (
    <div className="boot-screen">
      <Loader2 className="spin" size={20} strokeWidth={1.75} />
      <strong>正在打开工作区</strong>
      <span>读取本地配置并检查环境能力</span>
    </div>
  )
}
