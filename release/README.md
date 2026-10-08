# Production Releases (`release/`)

This directory stores compiled, self-contained production-ready release builds of **Antigravity Swiss Knife** for all three supported operating systems:
- **Linux** (x86_64 AppImage, deb, and standalone executable bundle)
- **Windows** (x86_64 installer and portable unpacked `.exe`)
- **macOS** (Apple Silicon `arm64` and Intel `x86_64` `.app` bundle / DMG)

**All compiled artifacts and unpacked bundles in this directory are gitignored.**
Having compiled release builds here allows running and testing the production application locally without relying on active development branches or having your running app disrupted when agents switch or modify branches.

## Directory Structure

```
release/
├── linux/
│   ├── app/ (or linux-unpacked/)   # Fully packaged Linux desktop application
│   ├── swiss                       # Standalone Go daemon binary (Linux amd64)
│   └── run.sh                      # Launch script for Linux release app
├── windows/
│   ├── app/ (or win-unpacked/)     # Fully packaged Windows desktop application
│   ├── swiss.exe                   # Standalone Go daemon binary (Windows amd64)
│   └── run.bat                     # Launch script for Windows release app
├── macos/
│   ├── app/ (or mac-arm64/ / mac/) # Fully packaged macOS .app application (arm64 & x64)
│   ├── swiss-darwin-arm64          # Standalone Go daemon binary (macOS arm64)
│   ├── swiss-darwin-amd64          # Standalone Go daemon binary (macOS amd64)
│   └── run.sh                      # Launch script for macOS release app (auto-selects arm64/x64)
├── run.sh                          # Universal launcher (auto-detects current OS)
└── README.md                       # This documentation
```

## How to Build Releases

Run the automated release build scripts from the project root:

```bash
# Build production releases for all 3 operating systems
npm run release

# Or build for a specific operating system
npm run release:linux
npm run release:win
npm run release:mac
```

## How to Run the Production Release App

To launch the compiled production release application on your laptop without touching active Git branches:

```bash
# Using npm (spawns detached process)
npm run app:prod

# Using the universal launcher (supports foreground or --detached)
bash release/run.sh
bash release/run.sh --detached

# Directly on Linux (foreground or background)
bash release/linux/run.sh
bash release/linux/run.sh --detached
```
