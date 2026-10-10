import React from 'react'
import { Bot } from 'lucide-react'

export interface AcpBrandIconProps {
  size?: number
  className?: string
  style?: React.CSSProperties
}

/**
 * Official Vector / SVG Logos for ACP Product Nodes
 * Conforming strictly to David-Design (clean SVG, no decorative emojis, no blurred assets).
 */

/** Antigravity 2.0 Official Vector Logo (Gaussian arch in Google gradient) */
export const AntigravityLogoIcon: React.FC<AcpBrandIconProps> = ({ size = 18, className, style }) => (
  <svg
    width={size}
    height={size}
    viewBox="0 0 200 184"
    fill="none"
    xmlns="http://www.w3.org/2000/svg"
    className={className}
    style={style}
  >
    <defs>
      <linearGradient id="antigravity-brand-grad" x1="10" y1="180" x2="190" y2="180" gradientUnits="userSpaceOnUse">
        <stop offset="0%" stopColor="#2563EB" />
        <stop offset="35%" stopColor="#3B82F6" />
        <stop offset="50%" stopColor="#10B981" />
        <stop offset="75%" stopColor="#F59E0B" />
        <stop offset="100%" stopColor="#EF4444" />
      </linearGradient>
    </defs>
    <path
      d="M16 160C16 160 52 142 68 96C84 50 96 14 100 14C104 14 116 50 132 96C148 142 184 160 184 160C192 164 196 174 190 180C184 186 172 182 164 174C140 150 128 116 100 116C72 116 60 150 36 174C28 182 16 186 10 180C4 174 8 164 16 160Z"
      fill="url(#antigravity-brand-grad)"
    />
  </svg>
)

/** Antigravity CLI Official Vector Logo (Terminal window with Antigravity mark) */
export const AntigravityCliIcon: React.FC<AcpBrandIconProps> = ({ size = 18, className, style }) => (
  <svg
    width={size}
    height={size}
    viewBox="0 0 24 24"
    fill="none"
    xmlns="http://www.w3.org/2000/svg"
    className={className}
    style={style}
  >
    <rect x="2" y="3" width="20" height="18" rx="4" stroke="currentColor" strokeWidth="1.75" />
    <path d="M6 7.5h.01M9 7.5h.01M12 7.5h.01" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
    <path
      d="M6 18.5C7.5 16.5 9 12 12 12C15 12 16.5 16.5 18 18.5"
      stroke="#3B82F6"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
    />
    <path d="M5.5 13L8 15.5L5.5 18" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
  </svg>
)

/** Cognition Devin Official Vector Logo */
export const DevinLogoIcon: React.FC<AcpBrandIconProps> = ({ size = 18, className, style }) => (
  <svg
    width={size}
    height={size}
    viewBox="0 0 24 24"
    fill="none"
    xmlns="http://www.w3.org/2000/svg"
    className={className}
    style={style}
  >
    <rect x="2" y="2" width="20" height="20" rx="5" fill="#059669" />
    <path
      d="M7 6.5H12C15.0376 6.5 17.5 8.96243 17.5 12C17.5 15.0376 15.0376 17.5 12 17.5H7V6.5Z"
      stroke="#FFFFFF"
      strokeWidth="2.2"
      strokeLinejoin="round"
    />
    <path d="M10 10.5L12.5 12L10 13.5" stroke="#FFFFFF" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round" />
    <path d="M13.5 14.5H15.5" stroke="#34D399" strokeWidth="1.75" strokeLinecap="round" />
  </svg>
)

/** OpenCode Interpreter Official Vector Logo */
export const OpenCodeLogoIcon: React.FC<AcpBrandIconProps> = ({ size = 18, className, style }) => (
  <svg
    width={size}
    height={size}
    viewBox="0 0 24 24"
    fill="none"
    xmlns="http://www.w3.org/2000/svg"
    className={className}
    style={style}
  >
    <rect x="2" y="2" width="20" height="20" rx="5" fill="#0284C7" />
    <path d="M7.5 8.5L4.5 12L7.5 15.5" stroke="#FFFFFF" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
    <path d="M16.5 8.5L19.5 12L16.5 15.5" stroke="#FFFFFF" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" />
    <path d="M13.5 7L10.5 17" stroke="#38BDF8" strokeWidth="2" strokeLinecap="round" />
  </svg>
)

/** DeepSeek Harness Official Whale Vector Logo */
export const DeepSeekLogoIcon: React.FC<AcpBrandIconProps> = ({ size = 18, className, style }) => (
  <svg
    width={size}
    height={size}
    viewBox="0 0 24 24"
    fill="none"
    xmlns="http://www.w3.org/2000/svg"
    className={className}
    style={style}
  >
    <path
      d="M3 13.5C3 8.8 6.8 5 12.2 5C17.6 5 21 8.8 21 12.2C21 15.6 18.5 18 15 18C12.5 18 11.5 16.5 9.5 16.5C7.5 16.5 6 18 3 18C2.5 18 2.5 16.2 3 13.5Z"
      fill="#1E88E5"
    />
    <path
      d="M17 5C18.2 3.2 20.2 2.5 21 2.5C21 4 19.8 6.2 18.5 7.2"
      fill="#1565C0"
    />
    <path
      d="M2.5 17C1.5 18.2 0.8 20.2 1.2 20.8C2.5 20.8 4.2 19.5 5.2 18"
      fill="#1E88E5"
    />
    <circle cx="8" cy="9.5" r="1.25" fill="#FFFFFF" />
  </svg>
)

/** Inflection Pi Official Vector Logo */
export const PiLogoIcon: React.FC<AcpBrandIconProps> = ({ size = 18, className, style }) => (
  <svg
    width={size}
    height={size}
    viewBox="0 0 24 24"
    fill="none"
    xmlns="http://www.w3.org/2000/svg"
    className={className}
    style={style}
  >
    <rect x="2" y="2" width="20" height="20" rx="5" fill="#18181B" />
    <path
      d="M6.5 8H17.5M8.5 8V16M14.5 8V15.5C14.5 16.4 15.2 16.8 16.5 16.7"
      stroke="#FFFFFF"
      strokeWidth="2.2"
      strokeLinecap="round"
      strokeLinejoin="round"
    />
  </svg>
)

/** OpenAI Codex Official Rosette Vector Logo */
export const CodexLogoIcon: React.FC<AcpBrandIconProps> = ({ size = 18, className, style }) => (
  <svg
    width={size}
    height={size}
    viewBox="0 0 24 24"
    fill="none"
    xmlns="http://www.w3.org/2000/svg"
    className={className}
    style={style}
  >
    <g stroke="#10A37F" strokeWidth="1.75" strokeLinecap="round" strokeLinejoin="round">
      <path d="M12 2a4 4 0 0 1 3.46 2l-4.46 7.72A1.5 1.5 0 0 1 9.5 11L6 9" />
      <path d="M20.66 7a4 4 0 0 1 0 4l-8.46.78a1.5 1.5 0 0 1-1.3-.75L9 8" />
      <path d="M20.66 17a4 4 0 0 1-3.46 2L12.74 11.28a1.5 1.5 0 0 1 .2-1.5L15 7" />
      <path d="M12 22a4 4 0 0 1-3.46-2l4.46-7.72a1.5 1.5 0 0 1 1.5.72L18 15" />
      <path d="M3.34 17a4 4 0 0 1 0-4l8.46-.78a1.5 1.5 0 0 1 1.3.75L15 16" />
      <path d="M3.34 7a4 4 0 0 1 3.46-2l4.46 7.72a1.5 1.5 0 0 1-.2 1.5L9 17" />
    </g>
  </svg>
)

/** Anthropic Claude Code Official Terracotta Spark Asterisk */
export const ClaudeCodeLogoIcon: React.FC<AcpBrandIconProps> = ({ size = 18, className, style }) => (
  <svg
    width={size}
    height={size}
    viewBox="0 0 24 24"
    fill="none"
    xmlns="http://www.w3.org/2000/svg"
    className={className}
    style={style}
  >
    <g fill="#D97706">
      <rect x="10.75" y="2" width="2.5" height="20" rx="1.25" />
      <rect x="2" y="10.75" width="20" height="2.5" rx="1.25" />
      <rect x="10.75" y="2" width="2.5" height="20" rx="1.25" transform="rotate(45 12 12)" />
      <rect x="10.75" y="2" width="2.5" height="20" rx="1.25" transform="rotate(-45 12 12)" />
    </g>
  </svg>
)

/** Anysphere Cursor Official 3D Isometric Cube */
export const CursorLogoIcon: React.FC<AcpBrandIconProps> = ({ size = 18, className, style }) => (
  <svg
    width={size}
    height={size}
    viewBox="0 0 24 24"
    fill="none"
    xmlns="http://www.w3.org/2000/svg"
    className={className}
    style={style}
  >
    <path d="M12 2.5L20 7.2L12 11.9L4 7.2L12 2.5Z" fill="#64748B" />
    <path d="M4 7.2L12 11.9V21.5L4 16.8V7.2Z" fill="#334155" />
    <path d="M12 11.9L20 7.2V16.8L12 21.5V11.9Z" fill="#1E293B" />
    <path d="M12 11.9V21.5" stroke="#475569" strokeWidth="0.75" />
  </svg>
)

/**
 * Returns the official brand vector SVG icon for the given ACP node ID.
 * Falls back to Lucide Bot icon for unknown or unconfigured nodes.
 */
export function getAcpBrandIcon(nodeId: string, size = 18): React.ReactNode {
  switch (nodeId) {
    case 'agent-antigravity':
      return <AntigravityLogoIcon size={size} />
    case 'agent-antigravity-cli':
      return <AntigravityCliIcon size={size} />
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
    default:
      return <Bot size={size} strokeWidth={1.75} />
  }
}
