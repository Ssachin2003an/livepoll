
import { useState, type FormEvent } from 'react'
import { ArrowLeft, ArrowRight, Check, Copy, Plus, X } from 'lucide-react'
import { Link, useNavigate } from 'react-router-dom'
import { api } from '../api'

export function CreatePollPage() {
  const navigate = useNavigate()
  const [question, setQuestion] = useState('')
  const [options, setOptions] = useState(['', ''])
  const [anonymous, setAnonymous] = useState(false)
  const [allowMultiple, setAllowMultiple] = useState(false)
  const [maxSelections, setMaxSelections] = useState(2)
  const [error, setError] = useState('')
  const [createdId, setCreatedId] = useState('')
  const [shareFeedback, setShareFeedback] = useState('')
  const [busy, setBusy] = useState(false)

  function updateOption(index: number, text: string) {
    setOptions(current => current.map((item, i) => i === index ? text : item))
  }

  function addOption() {
    if (options.length < 10) setOptions(current => [...current, ''])
  }

  function removeOption(index: number) {
    if (options.length > 2) {
      const next = options.filter((_, i) => i !== index)
      setOptions(next)
      setMaxSelections(current => Math.min(current, next.length))
    }
  }

  async function submit(event: FormEvent) {
    event.preventDefault()
    setError('')

    const q = question.trim()
    const cleanOptions = options.map(option => option.trim())

    if (q.length < 5 || q.length > 240) {
      setError('Your question needs 5–240 characters.')
      return
    }
    if (cleanOptions.some(option => option.length < 1 || option.length > 100)) {
      setError('Each answer needs 1–100 characters.')
      return
    }
    if (new Set(cleanOptions.map(item => item.toLowerCase())).size !== cleanOptions.length) {
      setError('Each answer option should be different.')
      return
    }
    if (allowMultiple && (maxSelections < 2 || maxSelections > cleanOptions.length)) {
      setError('Choose a valid maximum number of selections.')
      return
    }

    setBusy(true)
    try {
      const { poll } = await api.createPoll(q, cleanOptions, {
        anonymous,
        allowMultiple,
        maxSelections: allowMultiple ? maxSelections : 1,
      })
      setCreatedId(poll.id)
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : 'We couldn’t create the poll. Please try again.')
    } finally {
      setBusy(false)
    }
  }

  async function copyShare() {
    try {
      await navigator.clipboard.writeText(`${window.location.origin}/polls/${createdId}`)
      setShareFeedback('Link copied.')
      window.setTimeout(() => setShareFeedback(''), 2500)
    } catch {
      setShareFeedback('Copy is unavailable. Open the poll to copy its URL.')
    }
  }

  if (createdId) {
    return (
      <main className="created-page wrap">
        <div className="success-mark"><Check size={27} /></div>
        <span className="eyebrow"><span className="eyebrow-line" /> READY TO SHARE</span>
        <h1>Your question<br /><em>is in the room.</em></h1>
        <p>Send this link to the people whose perspective matters. They can vote without signing in.</p>
        <div className="share-link-box">
          <span>{window.location.origin}/polls/{createdId}</span>
          <button className="button button-primary" onClick={copyShare}><Copy size={16} /> Copy link</button>
        </div>
        {shareFeedback && <div className="inline-notice" role="status">{shareFeedback}</div>}
        <div className="created-actions">
          <Link className="button button-outline" to={`/polls/${createdId}`}>View your poll <ArrowRight size={16} /></Link>
          <button className="button button-text" onClick={() => navigate('/dashboard')}>Back to my polls</button>
        </div>
      </main>
    )
  }

  return (
    <main className="create-page wrap">
      <Link to="/dashboard" className="back-link"><ArrowLeft size={15} /> My polls</Link>
      <div className="create-layout">
        <section className="create-intro">
          <span className="eyebrow"><span className="eyebrow-line" /> PUT A QUESTION TO THE ROOM</span>
          <h1>Start with<br /><em>what matters.</em></h1>
          <p>Keep it clear. Add a few answers. Then share a link and watch the signal come in.</p>
          <div className="create-note">
            <span className="create-note-icon"><Check size={15} /></span>
            <span><b>Thoughtful by default</b><small>People vote once. Results update for everyone.</small></span>
          </div>
        </section>

        <section className="create-form-card">
          <div className="form-card-heading">
            <span className="step-chip">01 <i /> BUILD YOUR POLL</span>
            <span className="required-note">* REQUIRED</span>
          </div>

          <form onSubmit={submit} noValidate>
            <label className="field-label" htmlFor="question">
              Your question <span className="char-count">{question.length}/240</span>
              <textarea id="question" value={question} onChange={e => setQuestion(e.target.value)} maxLength={240} placeholder="What should we focus on next?" rows={3} required />
            </label>

            <div className="options-heading">
              <span className="field-label">Answer options</span>
              <span className="option-count">{options.length} / 10</span>
            </div>

            <div className="options-list">
              {options.map((option, index) => (
                <div className="option-input" key={index}>
                  <span className="option-bullet">{String.fromCharCode(65 + index)}</span>
                  <input aria-label={`Answer option ${index + 1}`} value={option} onChange={e => updateOption(index, e.target.value)} maxLength={100} placeholder={`Choice ${index + 1}`} required />
                  {options.length > 2 && (
                    <button type="button" className="remove-option" aria-label={`Remove choice ${index + 1}`} onClick={() => removeOption(index)}>
                      <X size={15} />
                    </button>
                  )}
                </div>
              ))}
            </div>

            {options.length < 10 && <button type="button" className="add-option" onClick={addOption}><Plus size={15} /> Add another option</button>}

            
            <div className="poll-settings">
              <h3>Poll settings</h3>

              <div className="settings-toggles">
                <label className="settings-toggle">
                  <span className="settings-toggle-copy">
                    <b>Anonymous poll</b>
                    <small>Mark this poll as anonymous.</small>
                  </span>
                  <input
                    type="checkbox"
                    checked={anonymous}
                    onChange={e => setAnonymous(e.target.checked)}
                  />
                </label>

                <label className="settings-toggle">
                  <span className="settings-toggle-copy">
                    <b>Allow multiple selections</b>
                    <small>Let each voter choose more than one option.</small>
                  </span>
                  <input
                    type="checkbox"
                    checked={allowMultiple}
                    onChange={e => {
                      setAllowMultiple(e.target.checked)
                      setMaxSelections(e.target.checked ? Math.min(2, options.length) : 1)
                    }}
                  />
                </label>
              </div>

              {allowMultiple && (
                <label className="settings-field settings-max-selections">
                  <span>Maximum selections per voter</span>
                  <select
                    value={maxSelections}
                    onChange={e => setMaxSelections(Number(e.target.value))}
                  >
                    {Array.from(
                      { length: Math.max(0, options.length - 1) },
                      (_, index) => index + 2
                    ).map(value => (
                      <option key={value} value={value}>
                        {value} selections
                      </option>
                    ))}
                  </select>
                </label>
              )}
            </div>


            {error && <div className="form-error" role="alert">{error}</div>}

            <button className="button button-primary create-submit" disabled={busy}>
              {busy ? 'Creating your poll…' : 'Create poll'} <ArrowRight size={17} />
            </button>
            <p className="privacy-note">Your poll is public to view and vote on. Only you can manage it.</p>
          </form>
        </section>
      </div>
    </main>
  )
}
