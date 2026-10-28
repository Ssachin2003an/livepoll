import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { AuthPage } from './AuthPage'

afterEach(() => cleanup())

const authMock = vi.hoisted(() => ({ user: null as null, login: vi.fn(), signup: vi.fn(), logout: vi.fn(), loading: false }))
vi.mock('../auth/AuthContext', () => ({ useAuth: () => authMock }))

function renderSignup() {
  return render(<MemoryRouter><AuthPage mode="signup" /></MemoryRouter>)
}

describe('AuthPage signup form', () => {
  beforeEach(() => { authMock.login.mockReset(); authMock.signup.mockReset() })
  it('renders the hero heading with real line breaks and emphasis', () => {
    renderSignup()
    const heading = screen.getByRole('heading', { level: 1 })
    expect(heading).toHaveTextContent('Your next')
    expect(heading.querySelector('br')).not.toBeNull()
    expect(heading.querySelector('em')).toHaveTextContent('starts here.')
    expect(heading).not.toHaveTextContent('<br')
  })
  it('rejects a short password before making a request', () => {
    renderSignup()
    fireEvent.change(screen.getByLabelText(/Your name/), { target: { value: 'Alex Doe' } })
    fireEvent.change(screen.getByLabelText('Email address'), { target: { value: 'alex@example.com' } })
    fireEvent.change(screen.getByLabelText('Password'), { target: { value: 'short' } })
    fireEvent.click(screen.getByRole('button', { name: /create account/i }))
    expect(screen.getByRole('alert')).toHaveTextContent('at least 10 characters')
    expect(authMock.signup).not.toHaveBeenCalled()
  })
  it('rejects malformed email addresses', () => {
    renderSignup()
    fireEvent.change(screen.getByLabelText(/Your name/), { target: { value: 'Alex Doe' } })
    fireEvent.change(screen.getByLabelText('Email address'), { target: { value: 'not-an-email' } })
    fireEvent.change(screen.getByLabelText('Password'), { target: { value: 'correct-horse-battery' } })
    fireEvent.click(screen.getByRole('button', { name: /create account/i }))
    expect(screen.getByRole('alert')).toHaveTextContent('valid email')
    expect(authMock.signup).not.toHaveBeenCalled()
  })
})
