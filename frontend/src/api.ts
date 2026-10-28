
import type { Poll, Results, User } from './types'

const API_BASE = (import.meta.env.VITE_API_BASE ?? '').replace(/\/$/, '')

export class ApiError extends Error {
  constructor(message: string, public status: number) {
    super(message)
    this.name = 'ApiError'
  }
}

let cachedCsrfToken = ''

async function getCsrfToken(): Promise<string> {
  if (cachedCsrfToken) return cachedCsrfToken

  const response = await fetch(`${API_BASE}/api/auth/csrf`, {
    method: 'GET',
    credentials: 'include',
    headers: { Accept: 'application/json' },
  })

  const body = await response.json().catch(() => ({}))

  if (!response.ok || typeof body.csrfToken !== 'string' || !body.csrfToken) {
    throw new ApiError(
      body.error || 'Could not initialize security token. Please try again.',
      response.status || 500,
    )
  }

  cachedCsrfToken = body.csrfToken
  return cachedCsrfToken
}

async function request<T>(
  path: string,
  init: RequestInit = {},
): Promise<T> {
  const method = (init.method || 'GET').toUpperCase()
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(init.headers as Record<string, string> | undefined),
  }

  const needsCsrf =
    ['POST', 'PATCH', 'PUT', 'DELETE'].includes(method) &&
    !path.endsWith('/auth/login') &&
    !path.endsWith('/auth/signup')

  if (needsCsrf) {
    headers['X-CSRF-Token'] = await getCsrfToken()
  }

  const response = await fetch(`${API_BASE}${path}`, {
    ...init,
    credentials: 'include',
    headers,
  })

  const body = await response.json().catch(() => ({}))

  if (!response.ok) {
    throw new ApiError(
      body.error || 'Something went wrong. Please try again.',
      response.status,
    )
  }

  return body as T
}

function unwrapPoll(value: Poll | { poll: Poll }): Poll {
  return 'poll' in value ? value.poll : value
}

export const api = {
  me: () => request<{ user: User }>('/api/auth/me'),

  signup: (name: string, email: string, password: string) =>
    request<{ user: User }>('/api/auth/signup', {
      method: 'POST',
      body: JSON.stringify({ name, email, password }),
    }),

  login: (email: string, password: string) =>
    request<{ user: User }>('/api/auth/login', {
      method: 'POST',
      body: JSON.stringify({ email, password }),
    }),

  logout: () => request<{ message: string }>('/api/auth/logout', {
    method: 'POST',
  }),

  async polls() {
    const data = await request<Poll[] | { polls: Poll[] }>('/api/polls')
    return Array.isArray(data) ? { polls: data } : data
  },

  async createPoll(
    question: string,
    options: string[],
    settings: {
      closesAt?: string | null
      anonymous?: boolean
      allowMultiple?: boolean
      maxSelections?: number
    } = {},
  ) {
    const data = await request<Poll | { poll: Poll }>('/api/polls', {
      method: 'POST',
      body: JSON.stringify({
        question,
        options,
        anonymous: settings.anonymous ?? false,
        allowMultiple: settings.allowMultiple ?? false,
        maxSelections: settings.allowMultiple
          ? settings.maxSelections ?? 2
          : 1,
        ...(settings.closesAt ? { closesAt: settings.closesAt } : {}),
      }),
    })

    return { poll: unwrapPoll(data) }
  },

  async poll(id: string) {
    const data = await request<Poll | { poll: Poll }>(
      `/api/polls/${encodeURIComponent(id)}`,
    )
    return { poll: unwrapPoll(data) }
  },

  results: (id: string) =>
    request<Results>(`/api/polls/${encodeURIComponent(id)}/results`),

  async vote(id: string, optionId: string | string[]) {
    const optionIds = Array.isArray(optionId) ? optionId : [optionId]

    const data = await request<{
      voteId?: string
      accepted?: boolean
      pendingRealtime?: boolean
    }>(`/api/polls/${encodeURIComponent(id)}/votes`, {
      method: 'POST',
      body: JSON.stringify({ optionIds }),
    })

    return {
      ...data,
      accepted: data.accepted ?? true,
      results: await api.results(id),
    }
  },

  async close(id: string) {
    return {
      poll: unwrapPoll(
        await request<Poll | { poll: Poll }>(
          `/api/polls/${encodeURIComponent(id)}/close`,
          { method: 'POST' },
        ),
      ),
    }
  },

  async reopen(id: string) {
    return {
      poll: unwrapPoll(
        await request<Poll | { poll: Poll }>(
          `/api/polls/${encodeURIComponent(id)}/reopen`,
          { method: 'POST' },
        ),
      ),
    }
  },

  async deletePoll(id: string) {
    return request<{ message: string }>(
      `/api/polls/${encodeURIComponent(id)}`,
      { method: 'DELETE' },
    )
  },
}

export function websocketURL(path: string): string {
  const explicit = import.meta.env.VITE_WS_BASE as string | undefined

  if (explicit) {
    return `${explicit.replace(/\/$/, '')}${path}`
  }

  const base = API_BASE
    ? new URL(API_BASE, window.location.href)
    : new URL(window.location.href)

  base.protocol = base.protocol === 'https:' ? 'wss:' : 'ws:'
  base.pathname = path
  base.search = ''

  return base.toString()
}
