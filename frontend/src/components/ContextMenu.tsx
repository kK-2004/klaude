import { useEffect, useLayoutEffect, useRef, useState } from 'react'
import type { CSSProperties } from 'react'
import { createPortal } from 'react-dom'
import type { LucideIcon } from 'lucide-react'

export type ContextMenuItem = {
  id: string
  label: string
  icon: LucideIcon
  danger?: boolean
  onSelect: () => void
}

export type ContextMenuAnchor = { x: number; y: number }

export function ContextMenu({ anchor, items, label, onClose }: {
  anchor: ContextMenuAnchor
  items: ContextMenuItem[]
  label: string
  onClose: () => void
}) {
  const ref = useRef<HTMLDivElement>(null)
  const [placement, setPlacement] = useState<CSSProperties>({ left: anchor.x, top: anchor.y, visibility: 'hidden' })

  // 先渲染再量尺寸，才能把菜单收进视口内；定位完成前保持隐藏，避免闪一下再跳位。
  useLayoutEffect(() => {
    const node = ref.current
    if (!node) return
    const { width, height } = node.getBoundingClientRect()
    setPlacement({
      left: Math.max(8, Math.min(anchor.x, window.innerWidth - width - 8)),
      top: Math.max(8, Math.min(anchor.y, window.innerHeight - height - 8)),
      visibility: 'visible',
    })
  }, [anchor.x, anchor.y, items.length])

  useEffect(() => {
    const dismiss = () => onClose()
    const onKeyDown = (event: KeyboardEvent) => { if (event.key === 'Escape') onClose() }
    window.addEventListener('mousedown', dismiss)
    window.addEventListener('resize', dismiss)
    window.addEventListener('blur', dismiss)
    window.addEventListener('wheel', dismiss, { passive: true })
    window.addEventListener('keydown', onKeyDown)
    return () => {
      window.removeEventListener('mousedown', dismiss)
      window.removeEventListener('resize', dismiss)
      window.removeEventListener('blur', dismiss)
      window.removeEventListener('wheel', dismiss)
      window.removeEventListener('keydown', onKeyDown)
    }
  }, [onClose])

  return createPortal(
    <div
      ref={ref}
      className="context-menu"
      style={placement}
      role="menu"
      aria-label={label}
      onMouseDown={(event) => event.stopPropagation()}
      onContextMenu={(event) => event.preventDefault()}
    >
      {items.map((item) => {
        const Icon = item.icon
        return (
          <button
            key={item.id}
            type="button"
            role="menuitem"
            className={item.danger ? 'danger' : undefined}
            // 先关菜单再执行，确认框之类的阻塞式弹窗才不会压在菜单上。
            onClick={() => { onClose(); window.setTimeout(item.onSelect, 0) }}
          >
            <Icon size={14} strokeWidth={1.75} />
            {item.label}
          </button>
        )
      })}
    </div>,
    document.body,
  )
}
