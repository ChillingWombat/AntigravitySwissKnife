# Credits & Open Source Acknowledgements

Antigravity Swiss Knife builds upon open-source libraries, protocol standards, and tools developed by the broader software engineering community. This page catalogs the projects, libraries, and concepts that have shaped this codebase.

---

## 1. Operating System Keyrings & Secret Management

- **zalando/go-keyring**: Keyring abstraction patterns inspiring our cross-platform secret storage drivers.
  - Linux: FreeDesktop Secret Service API via `libsecret` and DBus (`org.freedesktop.Secret.Generic`).
  - Windows: Windows Credential Manager via native `wincred.h` APIs.
  - macOS: Apple Keychain Services via the native `security` framework.

---

## 2. Core Go Daemon & Storage Engine

- **modernc.org/sqlite**: Pure-Go SQLite engine implemented without CGO dependencies. Enables concurrency under Write-Ahead Logging (WAL) and cross-compilation across `linux/amd64`, `windows/amd64`, and `darwin/arm64`.
- **google/uuid**: RFC 4122 compliant UUID generation used for virtual device installation IDs and transaction tokens.
- **dustin/go-humanize**: Byte unit formatting routines for disk cache metrics and token volume presentation.

---

## 3. Desktop Shell & Packaging Infrastructure

- **Electron & electron-builder**: Cross-platform desktop window shell, system tray management via DBus StatusNotifierItem (SNI), and package generation for Debian/Ubuntu (`.deb`), Windows (NSIS `.exe`), and macOS Apple Disk Image (`.dmg`).
- **Chrome DevTools Protocol (CDP)**: Protocol definitions enabling runtime script injection, live telemetry capture, and active conversation state preservation without modifying host binaries on disk.

---

## 4. Frontend Framework & Design System

- **React 19 & TypeScript**: Typed component rendering and application state modeling.
- **Vite**: Rapid frontend development server and ES module bundler.
- **Lucide React**: Clean vector icon library used across navigation rails, gauges, and status badges.
- **Google Material Design 3 (M3) & Google AI Studio**: Visual language, surface tokens, 16px card radii, and segmented pill tabs inspiring our desktop GUI aesthetics.

---

## 5. Security Standards & Cryptographic Algorithms

- **RFC 6238 & RFC 4226 (TOTP / HOTP)**: Open specifications for Time-Based One-Time Passwords, implemented natively in client memory via HMAC-SHA1 to provide 30-second verification codes without third-party authenticator dependencies.
- **RFC 2898 (PBKDF2)**: Password-Based Key Derivation Function 2 used for application lock screen password protection.
- **NIST SP 800-38D (AES-256-GCM)**: Galois/Counter Mode authenticated symmetric encryption protecting credential vaults on disk.

---

## 6. Agent Protocols & Interoperability

- **Agent Client Protocol (ACP)**: Protocol standard guiding inter-process communication, session handshakes, and event feeds between Antigravity, Claude Code, Cursor, OpenCode, Codex, and other local agent runtimes.

---

## 7. Testing, Tooling & Quality Assurance

- **Quickemu**: Virtualization framework enabling fast, automated testing of release packages inside native Windows 11 and macOS Ventura VMs directly from Linux hosts.
- **OxLint**: Fast JavaScript and TypeScript linter used in frontend continuous integration checks.
