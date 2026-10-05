/**
 * Client-side RFC 6238 Time-Based One-Time Password (TOTP) generator.
 * Uses the Web Cryptography API (crypto.subtle) to derive standard 6-digit verification codes.
 */

/**
 * Sanitizes input secret from plain base32 strings or otpauth:// URIs.
 */
export function sanitizeTotpSecret(secret: string): string {
  if (!secret) return ''
  let s = secret.trim()
  if (s.startsWith('otpauth://')) {
    try {
      const u = new URL(s)
      s = u.searchParams.get('secret') || ''
    } catch {
      // Fallback regex if URL parsing fails on custom schemes
      const m = s.match(/[?&]secret=([A-Za-z2-7=]+)/i)
      if (m && m[1]) {
        s = m[1]
      }
    }
  }
  return s.toUpperCase().replace(/[\s=-]/g, '')
}

/**
 * Decodes a Base32 string into a Uint8Array.
 */
export function base32Decode(input: string): Uint8Array {
  const cleaned = sanitizeTotpSecret(input)
  if (!cleaned) return new Uint8Array(0)

  const alphabet = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567'
  let bits = 0
  let value = 0
  const output: number[] = []

  for (let i = 0; i < cleaned.length; i++) {
    const val = alphabet.indexOf(cleaned[i])
    if (val === -1) continue
    value = (value << 5) | val
    bits += 5
    if (bits >= 8) {
      output.push((value >>> (bits - 8)) & 255)
      bits -= 8
    }
  }

  return new Uint8Array(output)
}

export interface TotpResult {
  code: string
  formattedCode: string
  remainingSeconds: number
}

/**
 * Generates an RFC 6238 6-digit TOTP verification code.
 * Returns null if the secret is empty or invalid.
 */
export async function generateTOTP(
  secret: string,
  timestampMs: number = Date.now()
): Promise<TotpResult | null> {
  const keyBytes = base32Decode(secret)
  if (keyBytes.length === 0) {
    return null
  }

  try {
    const stepSeconds = 30
    const currentStep = Math.floor(timestampMs / 1000 / stepSeconds)
    const remainingSeconds = stepSeconds - (Math.floor(timestampMs / 1000) % stepSeconds)

    // Build 8-byte big-endian counter buffer
    const counterBuf = new ArrayBuffer(8)
    const view = new DataView(counterBuf)
    view.setBigUint64(0, BigInt(currentStep))

    // HMAC-SHA-1 via Web Crypto
    const rawKeyBuffer = keyBytes.buffer.slice(
      keyBytes.byteOffset,
      keyBytes.byteOffset + keyBytes.byteLength
    ) as ArrayBuffer
    const key = await crypto.subtle.importKey(
      'raw',
      rawKeyBuffer,
      { name: 'HMAC', hash: 'SHA-1' },
      false,
      ['sign']
    )
    const signature = await crypto.subtle.sign('HMAC', key, counterBuf)
    const hmac = new Uint8Array(signature)

    // Dynamic truncation per RFC 4226 / RFC 6238
    const offset = hmac[hmac.length - 1] & 0x0f
    const binary =
      ((hmac[offset] & 0x7f) << 24) |
      ((hmac[offset + 1] & 0xff) << 16) |
      ((hmac[offset + 2] & 0xff) << 8) |
      (hmac[offset + 3] & 0xff)

    const code = (binary % 1000000).toString().padStart(6, '0')
    const formattedCode = `${code.slice(0, 3)} ${code.slice(3)}`

    return {
      code,
      formattedCode,
      remainingSeconds,
    }
  } catch (err) {
    console.warn('Failed to compute TOTP:', err)
    return null
  }
}
