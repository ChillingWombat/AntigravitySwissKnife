# Antigravity Swiss Knife 🌌🗡️

<div align="center">
  <img src="assets/logo.png" alt="Antigravity Swiss Knife Logo" width="300" style="border-radius: 16px; box-shadow: 0 8px 32px rgba(0,0,0,0.5);" />
  <p><em>The ultimate native multi-tool, quota switcher, cache accelerator, device virtualizer, and UI extension for Google Antigravity 2.0.</em></p>
</div>

---

## 🌟 Overview

**Antigravity Swiss Knife** is a native companion daemon and in-app UI extension designed specifically for the **Google Antigravity 2.0** desktop application on Linux (with cross-platform architecture for macOS and Windows).

Unlike proxy-based or man-in-the-middle routing solutions, Antigravity Swiss Knife operates **100% natively and locally**, seamlessly managing credentials in the OS keyring (`secret-tool` / Secret Service) in full compliance with Terms of Service.

## 🚀 Key Features

### 1. 🔄 Native Zero-Loss Account Switcher
- **Live Quota Tracking**: Continuously monitors model quotas and exhaustion percentages via upstream Google APIs (`cloudcode-pa.googleapis.com`).
- **Atomic Credential Rotation**: Automatically switches credentials in the system keyring before exhaustion thresholds are reached.
- **Active Session Preservation**: Maintains active pane layouts and conversation IDs (`cascadeId` in `app_storage.json`), ensuring seamless resumption without lost context or `state.vscdb` missing errors.

### 2. 🖥️ Google Gemini-Inspired Desktop UI & Multi-Feature Architecture
- **Fixed Left Panel**: Collapsible/fixed navigation rail switching between primary modules (Account Switcher, Brain Cache Manager, Device Fingerprints, App Settings).
- **Sub-Page Top Ribbon**: Inside Account Switcher, toggle seamlessly between:
  1. *Quota Dashboard* (Front Page): Live gauge meters for Gemini 3.8 Flash, Flash Lite, Pro, and Claude models, active account status, reset countdowns, and 1-click manual switch.
  2. *Accounts & MFA Vault*: Account management with integrated RFC 6238 TOTP/MFA generator (live 6-digit codes, countdown progress rings, backup codes).
  3. *Automation Settings*: Auto-switch thresholds, polling frequencies, and warmup keep-alive toggles.
- **Gemini Design Language**: Clean Material Design 3 dark surfaces (`#131314` / `#1e1f20`), pill-shaped tabs, subtle glowing borders, and crisp typography.

### 3. 🛡️ Per-Account Device Fingerprint Virtualization
- **Anti-False-Ban Protection**: Prevents multi-account correlation by managing independent device profiles per account.
- **Isolated Hardware Identifiers**: Safely manages `machineid`, `.updaterId`, `installation_id`, and `installation_uuid`.

### 4. ⏰ Reset Horizon Keep-Alive Automation
- **Instant Window Trigger**: Upon quota window reset (`resetTime`), automatically dispatches a minimal warmup ping (`max_tokens: 1`) to immediately activate the next quota horizon.

### 5. ⚡ Brain & Prompt Cache Optimizer
- **Storage & Token Efficiency**: Scans and optimizes `~/.gemini/antigravity/brain/` and conversation logs.
- **Cache Acceleration**: Reduces redundant context payload sizes to save tokens and minimize latency.

---

## 🏗️ Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                 Antigravity 2.0 Desktop                     │
│  ┌─────────────────────────┐   ┌─────────────────────────┐  │
│  │    Sidebar UI Button    │──▶│  In-App Swiss Dashboard │  │
│  │ (Hooked via preload.js) │   │  (Quota gauges & tools) │  │
│  └─────────────────────────┘   └─────────────────────────┘  │
│               │                             ▲               │
└───────────────┼─────────────────────────────┼───────────────┘
                │ Loopback IPC                │
                ▼                             │
┌─────────────────────────────────────────────────────────────┐
│             Antigravity Swiss Knife Daemon                  │
│  ┌────────────────────────┐    ┌─────────────────────────┐  │
│  │ Quota Monitor & Poller │    │ Fingerprint Virtualizer │  │
│  └────────────────────────┘    └─────────────────────────┘  │
│  ┌────────────────────────┐    ┌─────────────────────────┐  │
│  │ Atomic Keyring Switcher│    │ Reset Horizon Trigger   │  │
│  └────────────────────────┘    └─────────────────────────┘  │
│  ┌────────────────────────┐    ┌─────────────────────────┐  │
│  │ Brain Cache Optimizer  │    │ Multi-Account Vault     │  │
│  └────────────────────────┘    └─────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
                │                             ▲
                ▼                             │
┌───────────────────────────┐   ┌─────────────────────────────┐
│ Linux Secret Service API  │   │ Upstream Google CloudCode   │
│ (secret-tool / libsecret) │   │ (cloudcode-pa.googleapis)   │
└───────────────────────────┘   └─────────────────────────────┘
```

---

## 📜 Roadmap & Milestones

- **Stage 1 (Current)**: Core Daemon, Native Linux Keyring Switcher, Upstream Quota Poller, Device Virtualizer, Reset Horizon Warmup, and Sidebar UI Injector.
- **Stage 2**: Advanced Cache Pruning & Token Reuse Optimizer.
- **Stage 3**: Cross-Platform Packaging (macOS Keychain, Windows Credential Manager).
