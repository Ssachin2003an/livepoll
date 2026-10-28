import type { LiveEvent, Results } from '../types'

export function applyLiveEvent(current: Results, event: LiveEvent, seen: Set<string>): Results {
  if (event.voteId && seen.has(event.voteId)) return current
  if (event.voteId) seen.add(event.voteId)
  if (event.totalVotes < current.totalVotes) return current
  return {
    ...current,
    poll: event.poll ?? current.poll,
    totalVotes: event.totalVotes,
    counts: event.counts,
    percentages: event.percentages,
  }
}
