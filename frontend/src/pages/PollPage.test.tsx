import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { PollPage } from './PollPage'

const { apiMock } = vi.hoisted(() => ({ apiMock: { results: vi.fn(), vote: vi.fn(), close: vi.fn(), reopen: vi.fn() } }))
vi.mock('../api', () => ({ api: apiMock, websocketURL: (path: string) => path }))
vi.mock('../auth/AuthContext', () => ({ useAuth: () => ({ user: null }) }))

const poll = { id: 'poll-1', question: 'Which direction feels right?', options: [{ id: 'north', text: 'North' }, { id: 'south', text: 'South' }], status: 'open' as const, createdAt: '2026-01-01T00:00:00Z', totalVotes: 0, hasVoted: false }
const initial = { poll, totalVotes: 0, counts: { north: 0, south: 0 }, percentages: { north: 0, south: 0 } }

class TestSocket {
  static instances: TestSocket[] = []
  onopen: ((event: Event) => void) | null = null
  onmessage: ((event: MessageEvent) => void) | null = null
  onerror: ((event: Event) => void) | null = null
  onclose: ((event: CloseEvent) => void) | null = null
  constructor(readonly url: string) { TestSocket.instances.push(this) }
  close() {}
  emitOpen() { this.onopen?.(new Event('open')) }
  emitClose() { this.onclose?.(new CloseEvent('close')) }
  emit(payload: unknown) { this.onmessage?.({ data: JSON.stringify(payload) } as MessageEvent) }
}

function renderPoll() {
  return render(<MemoryRouter initialEntries={['/polls/poll-1']}><Routes><Route path="/polls/:id" element={<PollPage />} /></Routes></MemoryRouter>)
}

describe('PollPage', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    TestSocket.instances = []
    vi.stubGlobal('WebSocket', TestSocket as unknown as typeof WebSocket)
    apiMock.results.mockResolvedValue(initial)
  })
  afterEach(() => { cleanup(); vi.unstubAllGlobals() })

  it('renders a poll, submits a vote, and applies one live event only once', async () => {
    const afterVote = { ...initial, totalVotes: 1, counts: { north: 1, south: 0 }, percentages: { north: 100, south: 0 }, poll: { ...poll, totalVotes: 1, hasVoted: true } }
    apiMock.vote.mockResolvedValue({ accepted: true, pendingRealtime: false, results: afterVote })
    renderPoll()
    expect(await screen.findByText(poll.question)).toBeInTheDocument()
    const socket = TestSocket.instances[0]
    act(() => socket.emitOpen())
    expect(await screen.findByText('Live connection')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('radio', { name: /North/ }))
    fireEvent.click(screen.getByRole('button', { name: /Cast your vote/ }))
    expect(await screen.findByText('Your vote is counted')).toBeInTheDocument()
    expect(apiMock.vote).toHaveBeenCalledWith('poll-1', 'north')

    const event = { type: 'vote', voteId: 'vote-2', totalVotes: 2, counts: { north: 1, south: 1 }, percentages: { north: 50, south: 50 } }
    act(() => socket.emit(event))
    await waitFor(() => expect(document.querySelector('.results-total strong')).toHaveTextContent('2'))
    act(() => socket.emit(event))
    expect(document.querySelector('.results-total strong')).toHaveTextContent('2')
    act(() => socket.emitClose())
    expect(await screen.findByText('Reconnecting')).toBeInTheDocument()
    expect(apiMock.results.mock.calls.length).toBeGreaterThanOrEqual(2)
  })

  it('shows a useful unavailable-poll state when initial results fail', async () => {
    apiMock.results.mockRejectedValue(new Error('Poll not found'))
    renderPoll()
    expect(await screen.findByText('POLL NOT FOUND')).toBeInTheDocument()
    expect(screen.getByText('Poll not found')).toBeInTheDocument()
  })
})
