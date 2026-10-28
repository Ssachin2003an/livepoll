import { useEffect, useState } from 'react'
import { Copy, Eye, Plus, Radio, Search, Share2, SlidersHorizontal, Sparkles, Trash2 } from 'lucide-react'
import { Link } from 'react-router-dom'
import { api } from '../api'
import { useAuth } from '../auth/AuthContext'
import type { Poll } from '../types'

export function DashboardPage() {
  const { user } = useAuth()
  const [polls, setPolls] = useState<Poll[]>([])
  const [deletingId, setDeletingId] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [query, setQuery] = useState('')
  const [statusFilter, setStatusFilter] = useState<'all' | 'open' | 'closed'>('all')
  const [notice, setNotice] = useState('')
  useEffect(() => { api.polls().then(data => setPolls(data.polls || [])).catch(err => setError(err.message)).finally(() => setLoading(false)) }, [])
  async function copyLink(id: string) {
    try { await navigator.clipboard.writeText(`${window.location.origin}/polls/${id}`); setNotice('Poll link copied.'); window.setTimeout(() => setNotice(''), 2500) }
    catch { setNotice('Copy is unavailable here. Open the poll and copy its URL.'); window.setTimeout(() => setNotice(''), 3500) }
  }
  
  async function deletePoll(id: string, question: string) {
    if (!window.confirm(`Delete "${question}"? This will permanently delete the poll and its votes.`)) {
      return
    }

    setDeletingId(id)

    try {
      await api.deletePoll(id)
      setPolls(current => current.filter(poll => poll.id !== id))
      setNotice('Poll deleted.')
    } catch (err) {
      setNotice(err instanceof Error ? err.message : 'Could not delete poll.')
    } finally {
      setDeletingId(null)
      window.setTimeout(() => setNotice(''), 3000)
    }
  }

const visible = polls.filter(poll =>
  poll.question.toLowerCase().includes(query.toLowerCase()) &&
  (statusFilter === 'all' || poll.status === statusFilter)
)
  const total = polls.reduce((sum, poll) => sum + (poll.totalVotes || 0), 0)
  return <main className="dashboard-page wrap">
    <div className="page-kicker"><span className="eyebrow"><span className="eyebrow-line" /> YOUR WORKSPACE</span><span className="page-kicker-date">A CLEARER VIEW, AT A GLANCE</span></div>
    <div className="dashboard-heading"><div><h1>Good to see you,<br /><em>{user?.name.split(' ')[0]}.</em></h1><p>Your questions are bringing the room into focus.</p></div><Link className="button button-primary" to="/polls/new"><Plus size={17} /> Create a poll</Link></div>
    <div className="stat-strip"><div><span className="stat-label">YOUR POLLS</span><strong>{loading ? '—' : polls.length.toString().padStart(2, '0')}</strong></div><div><span className="stat-label">TOTAL RESPONSES</span><strong>{loading ? '—' : total.toLocaleString()}</strong></div><div className="stat-live"><span className="live-orbit"><Radio size={18} /></span><span><b>Every result</b><small>Arrives as it happens</small></span></div></div>
    <section className="poll-list-section"><div className="list-title"><div><span className="eyebrow">YOUR POLL LIBRARY</span><h2>The questions that move things forward<span>.</span></h2></div><div className="list-tools"><label className="search-box"><Search size={15} /><input aria-label="Search polls" value={query} onChange={e => setQuery(e.target.value)} placeholder="Find a question" /></label><button className="icon-button" title={`Showing ${statusFilter} polls; click to change`} aria-label={`Filter polls: ${statusFilter}`} onClick={() => setStatusFilter(current => current === 'all' ? 'open' : current === 'open' ? 'closed' : 'all')}><SlidersHorizontal size={17} /></button></div></div>
      {notice && <div className="inline-notice" role="status">{notice}</div>}
      {error && <div className="empty-state error-state"><span className="empty-icon">!</span><h3>We couldn’t load your polls.</h3><p>{error}</p><button className="button button-outline" onClick={() => { setLoading(true); setError(''); api.polls().then(data => setPolls(data.polls || [])).catch(err => setError(err.message)).finally(() => setLoading(false)) }}>Try again</button></div>}
      {loading && <div className="loading-row"><span className="spinner" /> Getting your polls…</div>}
      {!loading && !error && visible.length > 0 && <div className="poll-table"><div className="poll-table-head"><span>QUESTION</span><span>STATUS</span><span>RESPONSES</span><span>CREATED</span><span /></div>{visible.map((poll, index) => <article className="poll-row" key={poll.id}><span className="poll-index">{String(index + 1).padStart(2, '0')}</span><div className="poll-row-title"><Link to={`/polls/${poll.id}`}>{poll.question}</Link><small>Public link · {poll.options.length} choices</small></div>
      <span className={`status-pill ${
  poll.status === 'open' ? 'status-open' : 'status-closed'
}`}>
  <i />
  {poll.status === 'open' ? 'Live' : 'Closed'}
</span>
      <span className="vote-count">{poll.totalVotes || 0}<small> votes</small></span><span className="poll-date">{new Date(poll.createdAt).toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' })}</span>
<Link
  className="row-open"
  to={`/polls/${poll.id}`}
  aria-label={`View ${poll.question}`}
  title="View poll"
>
  <Eye size={16} />
</Link>

<button
  className="row-share"
  onClick={() => copyLink(poll.id)}
  aria-label={`Copy link for ${poll.question}`}
  title="Copy poll link"
>
  <Share2 size={16} />
</button>

<button
  className="row-delete"
  onClick={() => deletePoll(poll.id, poll.question)}
  disabled={deletingId === poll.id}
  aria-label={`Delete ${poll.question}`}
  title="Delete poll"
>
  <Trash2 size={16} />
</button>
</article>)}</div>}
      {!loading && !error && polls.length > 0 && visible.length === 0 && <div className="empty-state"><span className="empty-icon"><Search size={18} /></span><h3>No questions match that search.</h3><p>Try a different phrase to find a poll.</p></div>}
      {!loading && !error && polls.length === 0 && <div className="empty-state"><span className="empty-icon"><Sparkles size={19} /></span><h3>There’s a first question waiting.</h3><p>Create a poll, share one link, and let the room weigh in.</p><Link className="button button-primary" to="/polls/new"><Plus size={16} /> Create your first poll</Link></div>}
    </section>
    <div className="dashboard-bottom-note"><Copy size={15} /><span>Every poll has its own link. Share it with anyone who should weigh in.</span></div>
  </main>
}
