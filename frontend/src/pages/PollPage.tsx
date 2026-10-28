
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Activity, ArrowLeft, Check, LoaderCircle, Radio, Share2, Wifi, WifiOff } from 'lucide-react'
import { Link, useParams } from 'react-router-dom'
import { api, websocketURL } from '../api'
import { useAuth } from '../auth/AuthContext'
import { applyLiveEvent } from '../poll/liveResults'
import type { LiveEvent, Poll, Results } from '../types'

type Connection = 'connecting' | 'live' | 'reconnecting' | 'offline'

function normalizeMessage(raw: unknown): LiveEvent | null {
  if (!raw || typeof raw !== 'object') return null
  const outer = raw as Record<string, unknown>
  const nested = (outer.data ?? outer.results ?? outer.result ?? outer) as Record<string, unknown>
  if (!nested || typeof nested !== 'object' || !nested.counts || typeof nested.counts !== 'object') return null
  const total = Number(nested.totalVotes ?? (nested.poll as Record<string, unknown> | undefined)?.totalVotes)
  if (!Number.isFinite(total)) return null
  return {
    type: (outer.type as LiveEvent['type']) || 'snapshot',
    voteId: typeof outer.voteId === 'string' ? outer.voteId : undefined,
    poll: (nested.poll as Poll | undefined) ?? (outer.poll as Poll | undefined),
    totalVotes: total,
    counts: nested.counts as Record<string, number>,
    percentages: (nested.percentages as Record<string, number>) || {},
  }
}

export function PollPage() {
  const { id = '' } = useParams()
  const { user } = useAuth()
  const [poll, setPoll] = useState<Poll | null>(null)
  const [results, setResults] = useState<Results | null>(null)
  const [connection, setConnection] = useState<Connection>('connecting')
  const [loading, setLoading] = useState(true)
  const [selected, setSelected] = useState<string[]>([])
  const [voted, setVoted] = useState(false)
  const [voting, setVoting] = useState(false)
  const [error, setError] = useState('')
  const [notice, setNotice] = useState('')
  const [shareMessage, setShareMessage] = useState('')
  const socketRef = useRef<WebSocket | null>(null)
  const seenVotes = useRef(new Set<string>())
  const owner = Boolean(user && poll?.ownerId && user.id === poll.ownerId)

  const refresh = useCallback(async () => {
    if (!id) return
    const next = await api.results(id)
    setPoll(next.poll)
    setResults(next)
  }, [id])

  useEffect(() => {
    let active = true
    api.results(id).then(next => {
      if (!active) return
      setPoll(next.poll)
      setResults(next)
      setVoted(Boolean(next.poll.hasVoted))
    }).catch(reason => {
      if (active) setError(reason instanceof Error ? reason.message : 'This poll isn’t available.')
    }).finally(() => {
      if (active) setLoading(false)
    })
    return () => { active = false }
  }, [id])

  useEffect(() => {
    if (!id) return
    let disposed = false
    let retryTimer: number | undefined
    let fallbackTimer: number | undefined
    let socket: WebSocket | null = null
    let attempts = 0

    const scheduleFallback = () => {
      if (!fallbackTimer) {
        fallbackTimer = window.setInterval(() => { refresh().catch(() => {}) }, 10000)
      }
    }

    const scheduleRetry = () => {
      if (disposed || retryTimer) return
      attempts += 1
      const base = Math.min(1000 * 2 ** Math.min(attempts, 5), 30000)
      const jitter = 0.75 + Math.random() * 0.5
      retryTimer = window.setTimeout(() => {
        retryTimer = undefined
        connect()
      }, base * jitter)
    }

    const connect = () => {
      if (disposed) return
      setConnection(attempts === 0 ? 'connecting' : 'reconnecting')
      try {
        socket = new WebSocket(websocketURL(`/api/ws/polls/${encodeURIComponent(id)}`))
        socketRef.current = socket
      } catch {
        setConnection('offline')
        scheduleFallback()
        scheduleRetry()
        return
      }

      socket.onopen = () => {
        if (disposed) return
        attempts = 0
        setConnection('live')
        if (fallbackTimer) {
          window.clearInterval(fallbackTimer)
          fallbackTimer = undefined
        }
        refresh().catch(() => {})
      }

      socket.onmessage = event => {
        try {
          const parsed = JSON.parse(String(event.data)) as Record<string, unknown>
          if (parsed.type === 'status' && (parsed.status === 'open' || parsed.status === 'closed')) {
            setPoll(current => current ? { ...current, status: parsed.status as Poll['status'] } : current)
            return
          }
          const live = normalizeMessage(parsed)
          if (!live) return
          setResults(current => current ? applyLiveEvent(current, live, seenVotes.current) : null)
          if (live.poll) setPoll(live.poll)
        } catch {
          // Ignore malformed frames; the next snapshot restores an authoritative view.
        }
      }

      socket.onerror = () => { if (!disposed) setConnection('offline') }
      socket.onclose = () => {
        if (disposed) return
        setConnection('reconnecting')
        scheduleFallback()
        scheduleRetry()
      }
    }

    connect()
    return () => {
      disposed = true
      if (retryTimer) window.clearTimeout(retryTimer)
      if (fallbackTimer) window.clearInterval(fallbackTimer)
      socket?.close()
      socketRef.current = null
    }
  }, [id, refresh])

  const canVote = poll?.status === 'open' && !voted && !owner
  const sortedOptions = useMemo(() => poll?.options ?? [], [poll])
  const maxSelections = poll?.allowMultiple
    ? Math.max(1, Math.min(poll.maxSelections || sortedOptions.length, sortedOptions.length))
    : 1

  function toggleOption(optionID: string) {
    if (!poll?.allowMultiple) {
      setSelected([optionID])
      return
    }
    setSelected(current => {
      if (current.includes(optionID)) return current.filter(id => id !== optionID)
      if (current.length >= maxSelections) {
        setNotice(`You can select up to ${maxSelections} options.`)
        return current
      }
      setNotice('')
      return [...current, optionID]
    })
  }

  async function castVote() {
    if (!selected.length || !canVote || voting) return
    setVoting(true)
    setError('')
    setNotice('')
    try {
      const answer = await api.vote(id, selected)
      setResults(answer.results)
      setPoll(answer.results.poll)
      setVoted(true)
      setNotice(answer.pendingRealtime ? 'Vote saved. Live results will catch up shortly.' : 'Your vote is in. The room is up to date.')
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : 'We couldn’t submit your vote.')
    } finally {
      setVoting(false)
    }
  }

  async function share() {
    const link = window.location.href
    try {
      if (navigator.share) await navigator.share({ title: poll?.question || 'Live poll', url: link })
      else {
        await navigator.clipboard.writeText(link)
        setShareMessage('Link copied to clipboard.')
      }
    } catch {
      setShareMessage('Sharing was cancelled or unavailable.')
    }
    window.setTimeout(() => setShareMessage(''), 2800)
  }

  async function setStatus(action: 'close' | 'reopen') {
    if (!window.confirm(action === 'close' ? 'Close this poll? Visitors will no longer be able to vote.' : 'Reopen this poll for new votes?')) return
    try {
      const result = action === 'close' ? await api.close(id) : await api.reopen(id)
      setPoll(result.poll)
      setNotice(action === 'close' ? 'Poll closed.' : 'Poll reopened.')
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : 'Could not update poll status.')
    }
  }

  if (loading) return <main className="poll-loading wrap"><span className="spinner" /> Finding the room…</main>

  if (error && !poll) {
    return <main className="poll-missing wrap"><span className="empty-icon">?</span><span className="eyebrow">POLL NOT FOUND</span><h1>We couldn’t find<br /><em>this question.</em></h1><p>{error}</p><Link className="button button-outline" to="/">Back to Live Poll</Link></main>
  }

  if (!poll || !results) return null

  const connectionLabel = connection === 'live' ? 'Live connection' : connection === 'connecting' ? 'Connecting' : connection === 'reconnecting' ? 'Reconnecting' : 'Results temporarily offline'

  return (
    <main className="poll-page wrap">
      <div className="poll-toolbar">
        <Link to={owner ? '/dashboard' : '/'} className="back-link"><ArrowLeft size={15} /> {owner ? 'My polls' : 'Live Poll'}</Link>
        <div className={`connection-state connection-${connection}`} role="status" aria-live="polite">
          {connection === 'live' ? <Wifi size={15} /> : connection === 'offline' ? <WifiOff size={15} /> : <LoaderCircle size={15} className="spin" />}
          <span>{connectionLabel}</span>
        </div>
      </div>

      <div className="poll-content">
        <section className="poll-main">
          <div className="poll-meta">
            <span className={`status-pill ${poll.status === 'open' ? 'status-open' : 'status-closed'}`}>
              <i /> {poll.status === 'open' ? 'Voting is open' : 'Voting closed'}
            </span>
            <span className="poll-created">POLL · {new Date(poll.createdAt).toLocaleDateString(undefined, { month: 'long', day: 'numeric' })}</span>
          </div>

          <h1>{poll.question}</h1>
          <div className="poll-subhead">
            <span>{poll.allowMultiple ? `Choose up to ${maxSelections} answers.` : 'Choose the answer that feels right.'}</span>
            <button className="share-action" onClick={share}><Share2 size={15} /> Share poll</button>
          </div>

          {error && <div className="form-error poll-error" role="alert">{error}</div>}
          {notice && <div className="inline-notice" role="status">{notice}</div>}

          <div className="vote-options" role={poll.allowMultiple ? 'group' : 'radiogroup'} aria-label="Choose your answer">
            {sortedOptions.map((option, index) => {
              const count = results.counts[option.id] || 0
              const percentage = results.totalVotes ? Math.round(count / results.totalVotes * 100) : 0
              const chosen = selected.includes(option.id)
              return (
                <button
                  className={`vote-option ${chosen ? 'option-selected' : ''} ${voted && chosen ? 'option-voted' : ''}`}
                  key={option.id}
                  role={poll.allowMultiple ? 'checkbox' : 'radio'}
                  aria-checked={chosen}
                  disabled={!canVote}
                  onClick={() => toggleOption(option.id)}
                >
                  <span className="vote-option-letter">{String.fromCharCode(65 + index)}</span>
                  <span className="vote-option-body">
                    <span className="vote-option-top"><b>{option.text}</b><span className="option-stat">{count} <small>·</small> {percentage}%</span></span>
                    <span className="result-track"><i style={{ width: `${Math.min(100, Math.max(percentage, results.totalVotes ? 3 : 0))}%`, transformOrigin: 'left' }} /></span>
                  </span>
                  {chosen && canVote && <span className="radio-check"><Check size={13} /></span>}
                </button>
              )
            })}
          </div>

          <div className="vote-actions">
            {canVote ? (
              <button className="button button-primary vote-submit" disabled={!selected.length || voting} onClick={castVote}>
                {voting ? 'Submitting…' : 'Cast your vote'} <span>↗</span>
              </button>
            ) : voted ? (
              <div className="vote-confirm"><Check size={17} /> Your vote is counted</div>
            ) : owner ? (
              <div className="vote-confirm"><Activity size={17} /> You’re viewing as the poll owner</div>
            ) : (
              <div className="vote-confirm">This poll is closed to new votes.</div>
            )}
            <span className="vote-anon">
              {poll.anonymous ? 'Anonymous poll' : 'One ballot per person'} · No sign-in needed
            </span>
          </div>
        </section>

        <aside className="results-panel">
          <div className="results-head">
            <div><span className="eyebrow">THE ROOM SO FAR</span><h2>Live results<span>.</span></h2></div>
            <div className={`results-live ${poll.status === 'open' ? '' : 'results-closed'}`}>
              <i />
              {poll.status === 'open' ? 'LIVE' : 'CLOSED'}
            </div>
          </div>
          <div className="results-total"><strong>{results.totalVotes.toLocaleString()}</strong><span>{results.totalVotes === 1 ? 'response' : 'responses'}</span></div>
          <div className="results-breakdown">
            {sortedOptions.map((option, index) => {
              const count = results.counts[option.id] || 0
              const percent = results.totalVotes ? Math.round(count / results.totalVotes * 100) : 0
              return (
                <div className="result-row" key={option.id}>
                  <div className="result-row-info"><span className="result-color" data-index={index} /><span className="result-label">{option.text}</span><span className="result-percent">{percent}%</span></div>
                  <div className="result-row-track"><i data-index={index} style={{ width: `${Math.min(100, Math.max(percent, results.totalVotes ? 2 : 0))}%` }} /></div>
                  <div className="result-row-count">{count} {count === 1 ? 'vote' : 'votes'}</div>
                </div>
              )
            })}
          </div>
          {results.totalVotes === 0 && <div className="no-votes">No votes yet. Your first response sets the signal.</div>}
          <div className="results-foot"><span className="results-lock"><Radio size={14} /> Updates appear automatically</span><span className="result-total-label">TOTAL · {results.totalVotes}</span></div>
          
{owner && (
  <div className="poll-manage">
    <span className="eyebrow">POLL MANAGEMENT</span>
    <p>Control whether new votes can come in.</p>
    <button
      className="button button-outline manage-button"
      onClick={() => setStatus(poll.status === 'open' ? 'close' : 'reopen')}
    >
      {poll.status === 'open' ? 'Close voting' : 'Reopen voting'}
    </button>
  </div>
)}

        </aside>
      </div>

      {shareMessage && <div className="toast" role="status"><Check size={15} /> {shareMessage}</div>}
    </main>
  )
}
