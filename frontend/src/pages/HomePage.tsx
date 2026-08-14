import { useApp } from '../app/use-app'
import { ApprovalBanner } from '../components/ApprovalBanner'
import { Composer } from '../components/Composer'
import { MessageStream } from '../components/MessageStream'

export function HomePage() {
  const { messages, approval } = useApp()
  const empty = messages.length === 0

  return (
    <div className={`page-home ${empty ? 'is-empty' : ''}`}>
      <div className="page-home-scroll">
        {!empty && <MessageStream />}
        {approval && <div className="home-approval"><ApprovalBanner /></div>}
      </div>
      <Composer />
    </div>
  )
}
