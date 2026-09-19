export type PasswordChangeErrorCode =
  | 'CURRENT_PASSWORD_INVALID'
  | 'WEAK_PASSWORD'
  | 'PASSWORD_MISMATCH'
  | 'UNAUTHENTICATED'
  | 'UNAVAILABLE'

interface SessionProbe {
  authenticated?: boolean
  csrf_token?: string
}

export class PasswordChangeError extends Error {
  constructor(public readonly code: PasswordChangeErrorCode) {
    super(code)
  }
}

export async function changeOwnPassword(input: {
  currentPassword: string
  newPassword: string
  confirmPassword: string
}) {
  const sessionResponse = await fetch('/auth/session', {
    credentials: 'include',
    headers: { Accept: 'application/json' },
  })
  if (!sessionResponse.ok) throw new PasswordChangeError('UNAVAILABLE')
  const session = (await sessionResponse.json()) as SessionProbe
  if (!session.authenticated || !session.csrf_token) throw new PasswordChangeError('UNAUTHENTICATED')

  const response = await fetch('/auth/password/change', {
    method: 'POST',
    credentials: 'include',
    headers: {
      Accept: 'application/json',
      'Content-Type': 'application/json',
      'X-CSRF-Token': session.csrf_token,
    },
    body: JSON.stringify({
      current_password: input.currentPassword,
      new_password: input.newPassword,
      confirm_password: input.confirmPassword,
    }),
  })
  const payload = (await response.json().catch(() => ({}))) as { error?: PasswordChangeErrorCode; changed?: boolean }
  if (!response.ok) {
    throw new PasswordChangeError(payload.error ?? (response.status === 401 ? 'UNAUTHENTICATED' : 'UNAVAILABLE'))
  }
  return payload
}
