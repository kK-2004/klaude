import { CalendarClock } from 'lucide-react'

export function ScheduledPage() {
  return (
    <div className="page-plain">
      <header className="page-heading">
        <h1>已安排</h1>
        <p>定时任务会显示在这里。</p>
      </header>
      <p className="page-muted"><CalendarClock size={16} strokeWidth={1.75} /> 还没有已安排的任务。</p>
    </div>
  )
}
