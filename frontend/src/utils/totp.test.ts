import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import { base32Decode, generateTOTP, sanitizeTotpSecret } from './totp.ts'

describe('TOTP Utility', () => {
  it('sanitizes base32 and otpauth URI secrets', () => {
    assert.equal(sanitizeTotpSecret('  jbsw y3dp ehpk 3pxp  '), 'JBSWY3DPEHPK3PXP')
    assert.equal(
      sanitizeTotpSecret('otpauth://totp/Google:user@gmail.com?secret=JBSWY3DPEHPK3PXP&issuer=Google'),
      'JBSWY3DPEHPK3PXP'
    )
    assert.equal(sanitizeTotpSecret(''), '')
  })

  it('decodes base32 correctly', () => {
    const bytes = base32Decode('JBSWY3DPEHPK3PXP')
    assert.equal(bytes.length, 10)
    // "Hello!\xde\xad\xbe\xef"
    assert.equal(new TextDecoder().decode(bytes.slice(0, 6)), 'Hello!')
  })

  it('generates consistent 6-digit TOTP code matching standard vector', async () => {
    const res = await generateTOTP('JBSWY3DPEHPK3PXP', 1700000000 * 1000)
    assert.ok(res !== null)
    assert.equal(res?.code, '324550')
    assert.equal(res?.formattedCode, '324 550')
    assert.equal(res?.remainingSeconds, 10)
  })

  it('returns null for empty or invalid secret', async () => {
    const res1 = await generateTOTP('')
    assert.equal(res1, null)

    const res2 = await generateTOTP('   ')
    assert.equal(res2, null)
  })
})
