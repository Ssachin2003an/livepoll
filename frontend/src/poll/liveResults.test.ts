import { describe, expect, it } from 'vitest'
import { applyLiveEvent } from './liveResults'
import type { Results } from '../types'

const initial: Results = {
  poll: {
    id: 'poll-1',
    question: 'Which way?',
    options: [
      { id: 'a', text: 'North' },
      { id: 'b', text: 'South' },
    ],
    status: 'open',
    createdAt: '2026-01-01T00:00:00Z',
    totalVotes: 1,
    anonymous: false,
    allowMultiple: false,
    maxSelections: 1,
  },
  totalVotes: 1,
  counts: { a: 1, b: 0 },
  percentages: { a: 100, b: 0 },
}
describe('applyLiveEvent', () => {
  it('applies a newer complete snapshot once', () => {
    const seen = new Set<string>()
    const event = { type: 'vote' as const, voteId: 'v-2', totalVotes: 2, counts: { a: 1, b: 1 }, percentages: { a: 50, b: 50 } }
    const first = applyLiveEvent(initial, event, seen)
    expect(first.totalVotes).toBe(2)
    expect(first.counts.b).toBe(1)
    expect(applyLiveEvent(first, event, seen)).toBe(first)
  })
  it('ignores an older snapshot', () => {
    const old = { type: 'snapshot' as const, totalVotes: 0, counts: { a: 0, b: 0 }, percentages: { a: 0, b: 0 } }
    expect(applyLiveEvent(initial, old, new Set())).toBe(initial)
  })
})
