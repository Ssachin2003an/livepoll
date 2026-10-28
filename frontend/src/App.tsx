import type { ReactNode } from 'react'
import { Link, Navigate, Route, Routes, useNavigate } from 'react-router-dom'
import { Activity, LogOut, Plus, Radio, Sun, Moon } from 'lucide-react'
import { useAuth, ProtectedRoute } from './auth/AuthContext'
import { HomePage } from './pages/HomePage'
import { AuthPage } from './pages/AuthPage'
import { DashboardPage } from './pages/DashboardPage'
import { CreatePollPage } from './pages/CreatePollPage'
import { PollPage } from './pages/PollPage'
import { ThemeProvider, useTheme } from './theme/ThemeContext'

function Brand() {
  return <Link to="/" className="brand" aria-label="Live Poll home"><span className="brand-mark"><i /><i /><i /></span><span>live<span>poll</span><b>.</b></span></Link>
}

function Header() {
  const { user, logout } = useAuth()
  const navigate = useNavigate()
  const { theme, toggleTheme } = useTheme()

  async function signOut() {
    await logout()
    navigate('/')
  }

  return (
    <header className="topbar">
      <Brand />
      <nav className="nav-links" aria-label="Main navigation">
        <button
          type="button"
          className="theme-toggle"
          onClick={toggleTheme}
          aria-label={`Switch to ${theme === 'light' ? 'dark' : 'light'} mode`}
          title={`Switch to ${theme === 'light' ? 'dark' : 'light'} mode`}
        >
          {theme === 'light' ? <Moon size={18} /> : <Sun size={18} />}
        </button>
        {user ? (
          <>
            <Link to="/dashboard">My polls</Link>
            <Link className="nav-cta" to="/polls/new">
              <Plus size={16} /> New poll
            </Link>
            <button
              className="avatar-button"
              title="Sign out"
              onClick={signOut}
            >
              <span>{user.name.slice(0, 1).toUpperCase()}</span>
              <LogOut size={15} />
            </button>
          </>
        ) : (
          <>
            <Link to="/login">Sign in</Link>
            <Link className="nav-cta" to="/signup">
              Create account <span aria-hidden="true">↗</span>
            </Link>
          </>
        )}
      </nav>
    </header>
  )
}

function Layout({ children }: { children: ReactNode }) {
  return <><Header />{children}<footer className="site-footer"><Brand /><span>Simple questions. Clear signals.</span><span className="footer-live"><Activity size={14} /> Built for the live moment</span></footer></>
}

export function App() {
  return (
    <ThemeProvider>
      <Layout>
        <Routes>
          <Route path="/" element={<HomePage />} />
          <Route path="/login" element={<AuthPage mode="login" />} />
          <Route path="/signup" element={<AuthPage mode="signup" />} />
          <Route
            path="/dashboard"
            element={<ProtectedRoute><DashboardPage /></ProtectedRoute>}
          />
          <Route
            path="/polls/new"
            element={<ProtectedRoute><CreatePollPage /></ProtectedRoute>}
          />
          <Route path="/polls/:id" element={<PollPage />} />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </Layout>
    </ThemeProvider>
  )
}

export function LiveBadge() { return <span className="live-badge"><i /><Radio size={13} /> LIVE RESULTS</span> }
