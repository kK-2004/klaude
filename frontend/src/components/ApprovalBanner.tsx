import { Check, X } from 'lucide-react'
import { useApp } from '../app/use-app'

export function ApprovalBanner() {
  const { approval, resolveApproval } = useApp()
  if (!approval) return null

  return (
    <div className="approval-card">
      <div>
        <strong>需要批准</strong>
        <p>{approval.summary}</p>
        <ul>
          <li>工作目录：{approval.cwd}</li>
          <li>风险：{approval.risk ?? '工作区变更'}</li>
          <li>超时：{approval.timeout ?? '提供方默认'}</li>
          <li>请求哈希：{approval.hash}</li>
        </ul>
      </div>
      <div className="approval-actions">
        <button type="button" onClick={() => void resolveApproval('rejected')}>
          <X size={14} strokeWidth={2} /> 拒绝
        </button>
        <button type="button" className="primary" onClick={() => void resolveApproval('approved')}>
          <Check size={14} strokeWidth={2} /> 允许一次
        </button>
      </div>
    </div>
  )
}
