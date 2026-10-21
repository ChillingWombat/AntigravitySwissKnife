export interface GoogleOAuthURLStatus {
  success: boolean
  active?: boolean
  auth_url?: string
}

export interface GoogleOAuthURLPollOptions {
  timeoutMs?: number
  pollMs?: number
}

function waitForPoll(signal: AbortSignal, delayMs: number): Promise<void> {
  return new Promise((resolve) => {
    const finish = () => {
      clearTimeout(timer)
      signal.removeEventListener('abort', finish)
      resolve()
    }
    const timer = setTimeout(finish, delayMs)
    signal.addEventListener('abort', finish, { once: true })
  })
}

export async function waitForGoogleOAuthURL(
  getStatus: (signal?: AbortSignal) => Promise<GoogleOAuthURLStatus>,
  signal: AbortSignal,
  options: GoogleOAuthURLPollOptions = {}
): Promise<string | null> {
  const timeoutMs = options.timeoutMs ?? 8000
  const pollMs = options.pollMs ?? 150
  const deadline = Date.now() + timeoutMs

  while (!signal.aborted && Date.now() < deadline) {
    try {
      const status = await getStatus(signal)
      if (status.success && status.active && status.auth_url) {
        return status.auth_url
      }
    } catch {
      if (signal.aborted) return null
    }

    if (signal.aborted || Date.now() >= deadline) break
    await waitForPoll(signal, pollMs)
  }

  return null
}
