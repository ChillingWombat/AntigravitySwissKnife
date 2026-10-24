/**
 * Utility functions for account presentation, formatting, and default behavior.
 */

export interface AccountTableDisplay {
  primaryText: string
  secondaryText: string | null
}

export interface CredentialAccount {
  email?: string
  password?: string
  totp_secret?: string
  access_token?: string
  refresh_token?: string
  mfa?: string
  credential?: {
    access_token?: string
    refresh_token?: string
  }
}

export interface AccountCredentialValues {
  password: string
  totpSecret: string
  accessToken: string
  refreshToken: string
}

export const REDACTED_CREDENTIAL = '[REDACTED]'

function isUsableCredential(value?: string): value is string {
  return typeof value === 'string' && value.trim() !== '' && value.trim() !== REDACTED_CREDENTIAL
}

function firstUsableCredential(...values: Array<string | undefined>): string | undefined {
  return values.find(isUsableCredential)
}

export function resolveMfaSecretForSave(
  account: { totp_secret?: string; has_totp?: boolean },
  totpSecret: string,
  mfaTouched: boolean
): string {
  const normalized = normalizeMfaSecret(totpSecret).toUpperCase()
  if (normalized) return normalized
  if (Boolean(account.has_totp || account.totp_secret)) {
    return mfaTouched ? '[CLEAR]' : REDACTED_CREDENTIAL
  }
  return ''
}

export function shouldHydrateAccountCredentials(
  account: CredentialAccount,
  isNewAccount: boolean
): boolean {
  if (isNewAccount || !account.email) return false

  return [
    account.password,
    account.totp_secret,
    account.access_token,
    account.refresh_token,
  ].some((value) => !isUsableCredential(value))
}

export function resolveHydratedAccountCredentials(
  account: CredentialAccount,
  match: CredentialAccount,
  current: AccountCredentialValues
): AccountCredentialValues {
  const resolveField = (
    initialValue: string | undefined,
    currentValue: string,
    matchValue: string | undefined
  ): string => {
    const isUnedited =
      currentValue === '' ||
      currentValue === REDACTED_CREDENTIAL ||
      currentValue === (initialValue || '')

    if (!isUsableCredential(initialValue) && isUnedited && isUsableCredential(matchValue)) {
      return matchValue
    }
    return currentValue
  }

  return {
    password: resolveField(
      account.password,
      current.password,
      match.password
    ),
    totpSecret: resolveField(
      account.totp_secret,
      current.totpSecret,
      firstUsableCredential(match.totp_secret, match.mfa)
    ),
    accessToken: resolveField(
      account.access_token,
      current.accessToken,
      firstUsableCredential(match.access_token, match.credential?.access_token)
    ),
    refreshToken: resolveField(
      account.refresh_token,
      current.refreshToken,
      firstUsableCredential(match.refresh_token, match.credential?.refresh_token)
    ),
  }
}

/**
 * Determines what is displayed in the account table row.
 * If account id (email) equals the alias (label), only the alias is shown and the id is hidden.
 */
export function getAccountTableDisplay(label?: string, email?: string): AccountTableDisplay {
  const cleanEmail = (email || '').trim()
  const cleanLabel = (label || '').trim()

  if (!cleanLabel) {
    return {
      primaryText: cleanEmail,
      secondaryText: null,
    }
  }

  if (cleanLabel.toLowerCase() === cleanEmail.toLowerCase()) {
    return {
      primaryText: cleanLabel,
      secondaryText: null,
    }
  }

  return {
    primaryText: cleanLabel,
    secondaryText: cleanEmail,
  }
}

export interface HeaderTitleDisplay {
  text: string
  isPlaceholder: boolean
  showStatusBadge: boolean
}

/**
 * Computes the header title text, styling (grey placeholder vs black active text),
 * and status badge visibility for AccountDetailModal.
 */
export function getAccountHeaderDisplay(
  isNewAccount: boolean,
  currentLabel: string,
  accountEmail?: string
): HeaderTitleDisplay {
  const trimmedLabel = currentLabel.trim()

  if (isNewAccount) {
    if (!trimmedLabel) {
      return {
        text: 'Account Alias',
        isPlaceholder: true,
        showStatusBadge: false,
      }
    }
    return {
      text: trimmedLabel,
      isPlaceholder: false,
      showStatusBadge: false,
    }
  }

  // Existing account
  if (trimmedLabel) {
    return {
      text: trimmedLabel,
      isPlaceholder: false,
      showStatusBadge: true,
    }
  }

  const cleanEmail = (accountEmail || '').trim()
  if (cleanEmail) {
    return {
      text: cleanEmail,
      isPlaceholder: false,
      showStatusBadge: true,
    }
  }

  return {
    text: 'Account Alias',
    isPlaceholder: true,
    showStatusBadge: true,
  }
}

/**
 * Resolves default alias value on blur or save if user hasn't entered one.
 */
export function resolveDefaultAlias(currentLabel: string, email: string): string {
  const trimmedLabel = currentLabel.trim()
  if (trimmedLabel) {
    return trimmedLabel
  }
  return email.trim()
}

/**
 * Normalizes MFA secret key by automatically removing all whitespace.
 * Handles continuous Base32 strings as well as spaced grouped 4-character formatting.
 */
export function normalizeMfaSecret(secret: string): string {
  if (!secret) return ''
  return secret.replace(/\s+/g, '')
}

export const ACCOUNT_SETUP_TEXTS = {
  PASSWORD_LABEL: 'Password (Optional):',
  PASSWORD_PLACEHOLDER: 'Optional login password',
  OAUTH_REFRESH_LABEL: 'OAuth Refresh Token:',
  OAUTH_REFRESH_PLACEHOLDER: '1//... (Sign in with Google or paste token)',
  MFA_SECRET_LABEL: 'MFA Secret Key:',
} as const

export interface AccountErrorAlertInfo {
  summaryText: string
  rawText: string
  verificationUrl: string | null
  isValidationRequired: boolean
}

/**
 * Parses upstream account error messages (including VALIDATION_REQUIRED and embedded
 * `(Verification: https://...)` challenge URLs) into structured UI alert details.
 * If the Google verification URL ends with an empty `authuser` parameter and the
 * account email is known, populates `authuser=<email>` so multi-account browsers
 * target the right Google session.
 */
export function parseAccountErrorAlert(
  raw?: string | null,
  accountEmail?: string
): AccountErrorAlertInfo {
  const rawText = (raw || '').trim()
  if (!rawText) {
    return {
      summaryText: 'Authentication error or token expired. Verification or token re-extraction required.',
      rawText: '',
      verificationUrl: null,
      isValidationRequired: false,
    }
  }

  let verificationUrl: string | null = null
  let summaryText = rawText

  const match = rawText.match(/\(Verification:\s*(https?:\/\/[^\s)]+)\)/i)
  if (match && match[1]) {
    let url = match[1].trim()
    const cleanEmail = (accountEmail || '').trim()
    if (cleanEmail) {
      if (/([?&])authuser=(?=&|$)/i.test(url)) {
        url = url.replace(/([?&])authuser=(?=&|$)/i, `$1authuser=${encodeURIComponent(cleanEmail)}`)
      } else if (/([?&])authuser$/i.test(url)) {
        url = url.replace(/([?&])authuser$/i, `$1authuser=${encodeURIComponent(cleanEmail)}`)
      }
    }
    verificationUrl = url
    summaryText = rawText.replace(match[0], '').trim()
  }

  const isValidationRequired =
    /VALIDATION_REQUIRED/i.test(rawText) || Boolean(verificationUrl)

  return {
    summaryText: summaryText || rawText,
    rawText,
    verificationUrl,
    isValidationRequired,
  }
}

export interface RequestGate {
  begin: () => number
  isCurrent: (requestId: number) => boolean
}

export function createRequestGate(): RequestGate {
  let currentRequest = 0
  return {
    begin: () => ++currentRequest,
    isCurrent: (requestId: number) => requestId === currentRequest,
  }
}

export async function commitLatest<T>(
  gate: RequestGate,
  requestId: number,
  result: Promise<T>,
  commit: (value: T) => void
): Promise<boolean> {
  const value = await result
  if (!gate.isCurrent(requestId)) return false
  commit(value)
  return true
}

export async function copyWithStatus(
  write: () => Promise<boolean | void>,
  setCopied: (copied: boolean) => void
): Promise<boolean> {
  try {
    const result = await write()
    if (result === false) throw new Error('clipboard copy failed')
    setCopied(true)
    return true
  } catch {
    setCopied(false)
    return false
  }
}

export async function copyTextWithFallback(text: string): Promise<boolean> {
  const electronWrite = (window as any).electronAPI?.writeClipboard
  if (electronWrite) {
    try {
      if (await electronWrite(text)) return true
    } catch {}
  }

  if (navigator?.clipboard?.writeText) {
    try {
      await navigator.clipboard.writeText(text)
      return true
    } catch {}
  }

  const textArea = document.createElement('textarea')
  textArea.value = text
  textArea.style.position = 'fixed'
  textArea.style.opacity = '0'
  document.body.appendChild(textArea)
  try {
    textArea.focus()
    textArea.select()
    const ok = document.execCommand('copy')
    if (!ok) throw new Error('execCommand copy failed')
    return true
  } catch {
    return false
  } finally {
    textArea.remove()
  }
}
