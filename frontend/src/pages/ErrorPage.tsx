import { CircleAlert, RotateCcw } from 'lucide-react'
import { useApp } from '../app/use-app'

export function ErrorPage() {
  const { diagnostic, retryInit } = useApp()

  return (
    <div className="boot-screen is-error">
      <CircleAlert size={20} strokeWidth={1.75} />
      <strong>初始化失败</strong>
      <span>{diagnostic}</span>
      <span className="boot-hint">诊断信息已显示在上方。修复配置或数据目录后可以重试。</span>
      <button type="button" className="primary-save" onClick={retryInit}>
        <RotateCcw size={14} strokeWidth={2} /> 重试
      </button>
    </div>
  )
}
