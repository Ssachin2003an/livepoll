import { ArrowDownRight, ArrowRight, Check, CircleHelp, Radio, Share2, Vote } from 'lucide-react'
import { Link } from 'react-router-dom'
import { useAuth } from '../auth/AuthContext'
import { LiveBadge } from '../App'

const steps = [
  { number: '01', icon: CircleHelp, title: 'Ask a good question', text: 'Write a question your room can answer in a tap.' },
  { number: '02', icon: Share2, title: 'Send one simple link', text: 'Share it anywhere. No account needed to take part.' },
  { number: '03', icon: Vote, title: 'Watch the room respond', text: 'Votes land in real time, for everyone watching.' },
]

export function HomePage() {
  const { user } = useAuth()
  return <main>
    <section className="hero wrap">
      <div className="hero-copy">
        <div className="eyebrow"><span className="eyebrow-line" /> YOUR ROOM, IN REAL TIME</div>
        <h1>Ask the room.<br /><span>See the signal.</span></h1>
        <p className="hero-lede">Make better decisions together. Launch a live poll, share one link, and watch every vote move the conversation.</p>
        <div className="hero-actions"><Link className="button button-primary" to={user ? '/polls/new' : '/signup'}>{user ? 'Create your poll' : 'Start a poll'} <ArrowRight size={17} /></Link><Link className="button button-text" to={user ? '/dashboard' : '/login'}>{user ? 'Go to your polls' : 'I already have an account'}</Link></div>
        <div className="hero-note"><span className="avatars" aria-hidden="true"><i>✓</i><i>↗</i><i>•</i></span><span>One clean link. Everyone on the same page.</span></div>
      </div>
      <div className="hero-art" aria-label="Poll link connects participants to a shared live result">
        <div className="art-orbit orbit-one" /><div className="art-orbit orbit-two" />
        <div className="signal-card">
          <div className="signal-top"><span className="signal-label"><span className="tiny-signal"><i /><i /><i /></span> LIVEPOLL / ROOM 01</span><LiveBadge /></div>
          <div className="signal-question">What should we focus on next?</div>
          <div className="signal-subline">One shared question. A clearer next move.</div>
          <div className="signal-divider" />
          <div className="signal-flow"><span className="flow-icon"><Share2 size={18} /></span><span className="flow-line" /><span className="flow-icon flow-icon-active"><Radio size={18} /></span><span className="flow-line" /><span className="flow-icon"><Vote size={18} /></span></div>
          <div className="flow-captions"><span>Share</span><span>Connect</span><span>Decide</span></div>
          <div className="signal-foot"><span><i className="connection-dot" /> READY WHEN YOU ARE</span><ArrowDownRight size={16} /></div>
        </div>
        <div className="floating-note"><span className="floating-check"><Check size={14} /></span> No refresh needed</div>
        <div className="hero-index">01 <span>—</span> 03</div>
      </div>
    </section>
    <section className="trust-strip"><div className="wrap trust-inner"><span>MADE FOR THE MOMENT A ROOM NEEDS AN ANSWER</span><div className="trust-tags"><span>MEETINGS</span><i /> <span>COMMUNITIES</span><i /> <span>CLASSROOMS</span></div></div></section>
    <section className="how-section wrap" id="how-it-works">
      <div className="section-intro"><div className="eyebrow"><span className="eyebrow-line" /> HOW IT WORKS</div><h2>Less waiting.<br />More <em>knowing.</em></h2><p>A useful poll should take seconds to join and give the whole group the same live picture.</p></div>
      <div className="step-list">{steps.map(({ number, icon: Icon, title, text }) => <article className="step-row" key={number}><span className="step-number">{number}</span><span className="step-icon"><Icon size={20} /></span><div><h3>{title}</h3><p>{text}</p></div><ArrowRight className="step-arrow" size={18} /></article>)}</div>
    </section>
    <section className="closing-band"><div className="wrap closing-inner"><div><span className="eyebrow eyebrow-light">ONE QUESTION CAN CHANGE THE ROOM</span><h2>Bring everyone<br /><em>into the answer.</em></h2></div><Link className="button button-light" to={user ? '/polls/new' : '/signup'}>Make your first poll <ArrowRight size={17} /></Link><span className="closing-mark"><i /><i /><i /></span></div></section>
  </main>
}
