import React from 'react'
import { Bot } from 'lucide-react'

// Assets are referenced through new URL(..., import.meta.url) so the module can
// also be imported by the Node-based component tests, where image extensions
// have no loader. Vite still resolves and fingerprints these URLs at build time.
const antigravityLogo = new URL('../assets/antigravity_logo.png', import.meta.url).href
const antigravityCliLogo = new URL('../assets/logo/Antigravity_CLI.webp', import.meta.url).href
const devinLogo = new URL('../assets/logo/devin.png', import.meta.url).href
const opencodeLogo = new URL('../assets/logo/opencode.png', import.meta.url).href
const deepseekLogo = new URL('../assets/logo/deepseek.png', import.meta.url).href
const piLogo = new URL('../assets/logo/pi.png', import.meta.url).href
const codexLogo = new URL('../assets/logo/codex.jpeg', import.meta.url).href
const claudeLogo = new URL('../assets/logo/claude.svg', import.meta.url).href
const cursorLogo = new URL('../assets/logo/cursor.jpeg', import.meta.url).href
const gcpLogo = new URL('../assets/logo/GCP.png', import.meta.url).href

export interface AcpBrandIconProps {
  size?: number
  className?: string
  style?: React.CSSProperties
}

/**
 * Official Real Logos for ACP Product Nodes & Cloud Providers
 * Conforming strictly to David-Design (real official logos from assets, zero decorative emojis).
 */

/** Antigravity 2.0 Official Logo */
export const AntigravityLogoIcon: React.FC<AcpBrandIconProps> = ({ size = 18, className, style }) => (
  <img
    src={antigravityLogo}
    alt="Antigravity 2.0"
    width={size}
    height={size}
    className={className}
    style={{ objectFit: 'contain', width: size, height: size, display: 'inline-block', verticalAlign: 'middle', ...style }}
  />
)

/** Antigravity CLI Official Logo */
export const AntigravityCliIcon: React.FC<AcpBrandIconProps> = ({ size = 18, className, style }) => (
  <img
    src={antigravityCliLogo}
    alt="Antigravity CLI"
    width={size}
    height={size}
    className={className}
    style={{ objectFit: 'contain', width: size, height: size, display: 'inline-block', verticalAlign: 'middle', ...style }}
  />
)

/** Antigravity VS Code Extension Logo */
export const AntigravityVSCodeIcon: React.FC<AcpBrandIconProps> = ({ size = 18, className, style }) => (
  <img
    src={antigravityLogo}
    alt="Antigravity VS Code Extension"
    width={size}
    height={size}
    className={className}
    style={{ objectFit: 'contain', width: size, height: size, display: 'inline-block', verticalAlign: 'middle', ...style }}
  />
)

/** Cognition Devin Official Logo */
export const DevinLogoIcon: React.FC<AcpBrandIconProps> = ({ size = 18, className, style }) => (
  <img
    src={devinLogo}
    alt="Devin"
    width={size}
    height={size}
    className={className}
    style={{ objectFit: 'contain', width: size, height: size, display: 'inline-block', verticalAlign: 'middle', borderRadius: '4px', ...style }}
  />
)

/** OpenCode Interpreter Official Logo */
export const OpenCodeLogoIcon: React.FC<AcpBrandIconProps> = ({ size = 18, className, style }) => (
  <img
    src={opencodeLogo}
    alt="OpenCode"
    width={size}
    height={size}
    className={className}
    style={{ objectFit: 'contain', width: size, height: size, display: 'inline-block', verticalAlign: 'middle', borderRadius: '4px', ...style }}
  />
)

/** DeepSeek Harness Official Logo */
export const DeepSeekLogoIcon: React.FC<AcpBrandIconProps> = ({ size = 18, className, style }) => (
  <img
    src={deepseekLogo}
    alt="DeepSeek"
    width={size}
    height={size}
    className={className}
    style={{ objectFit: 'contain', width: size, height: size, display: 'inline-block', verticalAlign: 'middle', borderRadius: '4px', ...style }}
  />
)

/** Inflection Pi Official Logo */
export const PiLogoIcon: React.FC<AcpBrandIconProps> = ({ size = 18, className, style }) => (
  <img
    src={piLogo}
    alt="Pi"
    width={size}
    height={size}
    className={className}
    style={{ objectFit: 'contain', width: size, height: size, display: 'inline-block', verticalAlign: 'middle', borderRadius: '4px', ...style }}
  />
)

/** OpenAI Codex Official Logo */
export const CodexLogoIcon: React.FC<AcpBrandIconProps> = ({ size = 18, className, style }) => (
  <img
    src={codexLogo}
    alt="Codex"
    width={size}
    height={size}
    className={className}
    style={{ objectFit: 'contain', width: size, height: size, display: 'inline-block', verticalAlign: 'middle', borderRadius: '4px', ...style }}
  />
)

/** Anthropic Claude Code Official Logo */
export const ClaudeCodeLogoIcon: React.FC<AcpBrandIconProps> = ({ size = 18, className, style }) => (
  <img
    src={claudeLogo}
    alt="Claude Code"
    width={size}
    height={size}
    className={className}
    style={{ objectFit: 'contain', width: size, height: size, display: 'inline-block', verticalAlign: 'middle', borderRadius: '4px', ...style }}
  />
)

/** Anysphere Cursor Official Logo */
export const CursorLogoIcon: React.FC<AcpBrandIconProps> = ({ size = 18, className, style }) => (
  <img
    src={cursorLogo}
    alt="Cursor"
    width={size}
    height={size}
    className={className}
    style={{ objectFit: 'contain', width: size, height: size, display: 'inline-block', verticalAlign: 'middle', borderRadius: '4px', ...style }}
  />
)

/** Google Cloud Platform Official Logo */
export const GoogleCloudLogoIcon: React.FC<AcpBrandIconProps> = ({ size = 18, className, style }) => (
  <img
    src={gcpLogo}
    alt="Google Cloud Platform"
    width={size}
    height={size}
    className={className}
    style={{ objectFit: 'contain', width: size, height: size, display: 'inline-block', verticalAlign: 'middle', ...style }}
  />
)

/**
 * Returns the official brand icon for the given ACP node ID.
 * Falls back to Lucide Bot icon for unknown or unconfigured nodes.
 */
export function getAcpBrandIcon(nodeId: string, size = 18): React.ReactNode {
  switch (nodeId) {
    case 'agent-antigravity':
      return <AntigravityLogoIcon size={size} />
    case 'agent-antigravity-cli':
      return <AntigravityCliIcon size={size} />
    case 'agent-antigravity-vscode':
      return <AntigravityVSCodeIcon size={size} />
    case 'agent-devin':
      return <DevinLogoIcon size={size} />
    case 'agent-opencode':
      return <OpenCodeLogoIcon size={size} />
    case 'agent-deepseek-harness':
      return <DeepSeekLogoIcon size={size} />
    case 'agent-pi':
      return <PiLogoIcon size={size} />
    case 'agent-codex':
      return <CodexLogoIcon size={size} />
    case 'agent-claude-code':
      return <ClaudeCodeLogoIcon size={size} />
    case 'agent-cursor':
      return <CursorLogoIcon size={size} />
    case 'provider-gcp':
    case 'agent-gcp':
      return <GoogleCloudLogoIcon size={size} />
    default:
      return <Bot size={size} strokeWidth={1.75} />
  }
}
