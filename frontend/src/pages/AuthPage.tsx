import { useState, type FormEvent } from 'react'
import { ArrowRight, Check, LockKeyhole, Mail } from 'lucide-react'
import { Link, Navigate, useLocation, useNavigate } from 'react-router-dom'
import { useAuth } from '../auth/AuthContext'

type Props = { mode: 'login' | 'signup' }
export function AuthPage({ mode }: Props) {
  const signup = mode === 'signup'
  const auth = useAuth()
  const navigate = useNavigate()
  const location = useLocation()
  const [name, setName] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  async function submit(event: FormEvent) {
    event.preventDefault(); setError('')
    const normalizedEmail = email.trim().toLowerCase()
    if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(normalizedEmail)) { setError('Enter a valid email address.'); return }
    if (password.length < 10) { setError('Use at least 10 characters for your password.'); return }
    if (signup && name.trim().length < 2) { setError('Your name should be at least 2 characters.'); return }
    setBusy(true)
    try {
      if (signup) await auth.signup(name.trim(), normalizedEmail, password)
      else await auth.login(normalizedEmail, password)
      const from = (location.state as { from?: string } | null)?.from
      navigate(from || '/dashboard', { replace: true })
    } catch (reason) { setError(reason instanceof Error ? reason.message : 'We could not sign you in. Please try again.') }
    finally { setBusy(false) }
  }
  if (auth.user) return <Navigate to="/dashboard" replace />
  return <main className="auth-page wrap">
    <section className="auth-intro"><span className="eyebrow"><span className="eyebrow-line" /> A BETTER READ ON THE ROOM</span><h1>{signup ? <>Your next<br />good question<br /><em>starts here.</em></> : <>Good to<br />have you<br /><em>back.</em></>}</h1><p>Fast, thoughtful polls for the moments your group needs to move forward.</p><div className="auth-points"><span><Check size={15} /> Simple to share</span><span><Check size={15} /> Live by design</span></div></section>
    <section className="auth-card"><div className="auth-card-top"><span className="eyebrow">{signup ? 'CREATE YOUR ACCOUNT' : 'SIGN IN TO LIVEPOLL'}</span><span className="auth-step">01 <i /> 01</span></div><h2>{signup ? 'Make room for every voice.' : 'Pick up where the room left off.'}</h2><p className="auth-card-desc">{signup ? 'One account to create and manage your polls.' : 'Your polls are waiting for you.'}</p>
      <form onSubmit={submit} noValidate>
        {signup && <label className="field-label" htmlFor="name">Your name<div className="input-wrap"><input id="name" autoComplete="name" maxLength={80} value={name} onChange={e => setName(e.target.value)} placeholder="Jordan Lee" required /><span className="input-icon" aria-hidden="true">✳</span></div></label>}
        <label className="field-label" htmlFor="email">Email address<div className="input-wrap"><input id="email" type="email" autoComplete="email" maxLength={254} value={email} onChange={e => setEmail(e.target.value)} placeholder="you@company.com" required /><Mail className="input-icon" size={17} /></div></label>
        <label className="field-label" htmlFor="password">Password<div className="input-wrap"><input id="password" type="password" autoComplete={signup ? 'new-password' : 'current-password'} minLength={10} value={password} onChange={e => setPassword(e.target.value)} placeholder="At least 10 characters" required /><LockKeyhole className="input-icon" size={16} /></div></label>
        {signup && <p className="field-hint">At least 10 characters. Keep it somewhere safe.</p>}
        {error && <div className="form-error" role="alert">{error}</div>}
        <button className="button button-primary auth-submit" type="submit" disabled={busy}>{busy ? 'Please wait…' : signup ? 'Create account' : 'Sign in'} {!busy && <ArrowRight size={17} />}</button>
      </form>
      <div className="auth-switch">{signup ? <>Already have an account? <Link to="/login">Sign in</Link></> : <>New to Live Poll? <Link to="/signup">Create an account</Link></>}</div>
      <div className="auth-security"><LockKeyhole size={14} /> Your sign-in stays private and secure.</div>
    </section>
  </main>
}
