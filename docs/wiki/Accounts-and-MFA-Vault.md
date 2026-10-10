# Accounts & MFA Vault

[Features](Features.md) / Accounts & MFA Vault

The Accounts & MFA Vault subsystem provides local encrypted credential storage, automated Google OAuth token acquisition, and client-side Time-Based One-Time Password (TOTP) generation.

---

## 1. Cryptographic Architecture at Rest

All credentials stored on disk in `~/.config/antigravity-swiss/accounts.json` use AES-256-GCM authenticated encryption. Sensitive fields subject to encryption include account passwords, Google OAuth refresh tokens, access tokens, and MFA secret keys.

Serialized records use the versioned envelope format:

```text
enc:v1:<base64(12_byte_iv + ciphertext + 16_byte_auth_tag)>
```

Key derivation uses a local device secret stored within the host operating system's native keyring (Secret Service on Linux, Windows Credential Manager on Windows, and Keychain Services on macOS). The system supports automatic in-place migration, reading plaintext legacy configurations and writing encrypted records back to disk upon the first save.

---

## 2. Progressive Disclosure & Credential Protection

The Account Detail modal masks all sensitive authentication fields by default:
- **Account Password**: Concealed behind dots. A dedicated eye toggle button reveals the plaintext on demand.
- **OAuth Refresh Token**: Masked to prevent shoulder-surfing during screen shares.
- **MFA Secret Key**: Masked with inline visibility controls.
- **Free-Form Account Notes**: Multi-line plaintext field for local operational records, billing renewal notes, and tags.

---

## 3. Client-Side RFC 6238 TOTP Engine

The vault implements a pure-client RFC 6238 and RFC 4226 Time-Based One-Time Password algorithm. It derives verification codes directly in frontend memory without sending secrets over local IPC sockets or external networks.

### Technical Parameters
- **Hash Function**: HMAC-SHA1.
- **Time Step ($T_X$)**: 30 seconds.
- **Epoch Counter**: $T = \lfloor (\text{UnixTime} - T_0) / T_X \rfloor$ with $T_0 = 0$.
- **Digit Length**: 6 digits with dynamic truncation.
- **Input Formatting**: Decodes standard RFC 4648 Base32 secret strings and parses `otpauth://totp/` URI formats.

### Header Integration & 1-Click Clipboard Copy
The derived code renders directly in the header row of the MFA Secret input:
- The UI formats active codes into spaced triplets (`XXX XXX`, e.g., `582 914`) for legibility.
- Empty or invalid secret inputs display a muted fallback state (`--- ---`).
- An inline copy button copies the clean 6-digit numeric string to the clipboard and shows an instant visual badge (`Copied!`) for 1.5 seconds.
- An animated countdown indicator ticks down the remaining seconds of the active 30-second epoch window.

---

## 4. Google OAuth Loopback Extractor

Configuring accounts manually by copying refresh tokens from development consoles is error-prone. The vault provides a 1-click "Sign in with Google" flow:
1. The user clicks "Sign in with Google" in the account modal.
2. The daemon starts an ephemeral HTTP loopback listener on `127.0.0.1` binding to an available random port.
3. The desktop client opens the default system web browser to the Google OAuth consent endpoint with `access_type=offline` and `prompt=consent`.
4. Upon browser authentication, Google redirects to the local loopback URL with an authorization code.
5. The daemon exchanges the code at `oauth2.googleapis.com/token`, extracts the refresh token and user email, closes the loopback server, and populates the vault fields.
6. A manual callback fallback URL input is provided for headless or containerized environments where browser loopback redirects cannot bind to localhost.

---

## 5. Account Priority & Metadata Configuration

Each account profile contains settings that govern how the automated switcher selects standby successors:
- **Account Alias**: A human-readable display label (e.g., "Personal Work", "Pro Pilot") prioritized in table listings over raw email addresses.
- **Account Priority**: Discrete ranking (`High`, `Mid`, `Low`, defaulting to `High`). The auto-switch scheduler drains High priority accounts before activating Mid or Low tier reserves.
- **Allow Claude / Non-Gemini**: Controls whether the account permits usage of external non-Gemini model tokens.
- **Enable Credit Overages**: Toggles whether paid token overages beyond plan limits are permitted.
