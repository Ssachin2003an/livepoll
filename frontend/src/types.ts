export type User = {
  id: string
  name: string
  email: string
}

export type PollOption = {
  id: string
  text: string
}

export type Poll = {
  id: string
  ownerId?: string
  isOwner?: boolean
  hasVoted?: boolean
  question: string
  options: PollOption[]
  status: 'open' | 'closed'
  createdAt: string
  closesAt?: string | null
  anonymous: boolean
  allowMultiple: boolean
  maxSelections: number
  totalVotes: number
}

export type Results = {
  poll: Poll
  totalVotes: number
  counts: Record<string, number>
  percentages: Record<string, number>
}

export type LiveEvent = {
  type: 'snapshot' | 'vote' | 'status'
  voteId?: string
  poll?: Poll
  totalVotes: number
  counts: Record<string, number>
  percentages: Record<string, number>
}