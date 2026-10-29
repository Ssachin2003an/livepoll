import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { CreatePollPage } from './CreatePollPage'

afterEach(() => cleanup())

const { createPoll } = vi.hoisted(() => ({ createPoll: vi.fn() }))
vi.mock('../api', () => ({ api: { createPoll } }))

function renderPage() { return render(<MemoryRouter><CreatePollPage /></MemoryRouter>) }
function fillValidPoll() {
  fireEvent.change(screen.getByRole('textbox', { name: /Your question/ }), { target: { value: 'Which idea should ship first?' } })
  fireEvent.change(screen.getByRole('textbox', { name: 'Answer option 1' }), { target: { value: 'Search' } })
  fireEvent.change(screen.getByRole('textbox', { name: 'Answer option 2' }), { target: { value: 'Sharing' } })
}

describe('CreatePollPage', () => {
  beforeEach(() => { createPoll.mockReset() })

  it('rejects repeated options in the form before making an API call', () => {
    renderPage()
    fireEvent.change(screen.getByRole('textbox', { name: /Your question/ }), { target: { value: 'Which idea should ship first?' } })
    fireEvent.change(screen.getByRole('textbox', { name: 'Answer option 1' }), { target: { value: 'Search' } })
    fireEvent.change(screen.getByRole('textbox', { name: 'Answer option 2' }), { target: { value: ' search ' } })
    fireEvent.click(screen.getByRole('button', { name: /Create poll/ }))
    expect(screen.getByRole('alert')).toHaveTextContent('different')
    expect(createPoll).not.toHaveBeenCalled()
  })

  it('creates a real poll request and presents the share link with copy feedback', async () => {
    createPoll.mockResolvedValue({ poll: { id: 'a1b2c3' } })
    const writeText = vi.fn().mockResolvedValue(undefined)
    Object.defineProperty(navigator, 'clipboard', { configurable: true, value: { writeText } })
    renderPage()
    fillValidPoll()
    fireEvent.click(screen.getByRole('button', { name: /Create poll/ }))
    await screen.findByText('READY TO SHARE')
    expect(createPoll).toHaveBeenCalledWith( 'Which idea should ship first?', ['Search', 'Sharing'], { anonymous: false, allowMultiple: false, maxSelections: 1, } )
    expect(screen.getByText(`${window.location.origin}/polls/a1b2c3`)).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: /Copy link/ }))
    await waitFor(() => expect(writeText).toHaveBeenCalledWith(`${window.location.origin}/polls/a1b2c3`))
    expect(await screen.findByRole('status')).toHaveTextContent('Link copied')
  })

  it('shows an API failure without hiding the form', async () => {
    createPoll.mockRejectedValue(new Error('Poll service unavailable'))
    renderPage()
    fillValidPoll()
    fireEvent.click(screen.getByRole('button', { name: /Create poll/ }))
    expect(await screen.findByRole('alert')).toHaveTextContent('Poll service unavailable')
    expect(screen.getByRole('textbox', { name: /Your question/ })).toBeInTheDocument()
  })
})
