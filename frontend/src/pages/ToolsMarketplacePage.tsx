import React from 'react'
import {
  Users,
  HardDrive,
  ShieldCheck,
  TrendingUp,
  Cpu,
  Bookmark,
  Sparkles,
} from 'lucide-react'

const MODULES = [
  {
    category: 'CORE',
    title: 'Account Switcher',
    description: 'Atomic zero-loss OAuth credential rotation with session preservation & RFC 6238 MFA.',
    status: 'ACTIVE',
    badgeClass: 'badge-green',
    icon: Users,
    actionId: 'switcher',
  },
  {
    category: 'AI MODELS',
    title: 'Custom Model Provider',
    description: 'Inject BYOM endpoints (OpenAI, Anthropic, Gemini, Ollama, vLLM) directly into Antigravity with project routing & quota tracking.',
    status: 'ACTIVE',
    badgeClass: 'badge-green',
    icon: Sparkles,
    actionId: 'custom-models',
  },
  {
    category: 'CACHE',
    title: 'Brain Cache Optimizer',
    description: 'Deep scanner & safe disk reclamation for ~/.gemini/ with cascade immunity.',
    status: 'INSTALLED',
    badgeClass: 'badge-green',
    icon: HardDrive,
    actionId: 'cache',
  },
  {
    category: 'SECURITY',
    title: 'Fingerprint Virtualizer',
    description: 'Anti-ban per-account hardware & UUID profile virtualization.',
    status: 'INSTALLED',
    badgeClass: 'badge-green',
    icon: ShieldCheck,
  },
  {
    category: 'METRICS',
    title: 'Token Cost Tracker',
    description: 'Real-time consumption analytics and quota burn rate forecasting.',
    status: 'COMING SOON',
    badgeClass: 'badge-neutral',
    icon: TrendingUp,
  },
  {
    category: 'PROMPT',
    title: 'Prompt Bloat Compressor',
    description: 'Automatic context bloat stripper and JSON-RPC deduplicator.',
    status: 'BETA',
    badgeClass: 'badge-yellow',
    icon: Cpu,
  },
  {
    category: 'SESSION',
    title: 'Session Bookmarker',
    description: 'Preserve named workspace states and quick-jump between tasks.',
    status: 'COMING SOON',
    badgeClass: 'badge-neutral',
    icon: Bookmark,
  },
]

interface ToolsMarketplacePageProps {
  onSelectTool?: (toolIndex: number) => void
}

export const ToolsMarketplacePage: React.FC<ToolsMarketplacePageProps> = ({ onSelectTool }) => {
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
      {/* Header Info Card */}
      <div className="google-card">
        <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.8px', textTransform: 'uppercase' }}>
          Swiss Knife Tools Marketplace
        </div>
        <div style={{ fontSize: '13px', color: 'var(--text)', marginTop: '4px' }}>
          Extend your Antigravity companion with native productivity, security, and automation modules.
        </div>
      </div>

      {/* Grid of Extension Cards */}
      <div
        style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(auto-fill, minmax(320px, 1fr))',
          gap: '16px',
        }}
      >
        {MODULES.map((mod) => {
          const Icon = mod.icon
          return (
            <div
              key={mod.title}
              className="google-card"
              style={{
                display: 'flex',
                flexDirection: 'column',
                justifyContent: 'space-between',
                padding: '20px',
              }}
            >
              <div>
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '12px' }}>
                  <span
                    style={{
                      backgroundColor: 'var(--tonal)',
                      color: 'var(--text-muted)',
                      border: '1px solid var(--border)',
                      borderRadius: '4px',
                      padding: '2px 8px',
                      fontSize: '10px',
                      fontWeight: 700,
                      letterSpacing: '0.5px',
                    }}
                  >
                    {mod.category}
                  </span>

                  <span className={`badge-chip ${mod.badgeClass}`}>
                    {mod.status}
                  </span>
                </div>

                <div style={{ display: 'flex', alignItems: 'center', gap: '10px', marginBottom: '8px' }}>
                  <Icon size={20} color="var(--primary)" />
                  <h3 style={{ fontSize: '15px', fontWeight: 700, color: 'var(--text)' }}>
                    {mod.title}
                  </h3>
                </div>

                <p style={{ fontSize: '12px', color: 'var(--text-muted)', lineHeight: '1.5' }}>
                  {mod.description}
                </p>
              </div>

              <div style={{ marginTop: '16px', borderTop: '1px solid var(--border-subtle)', paddingTop: '12px', display: 'flex', justifyContent: 'flex-end' }}>
                <button
                  onClick={() => {
                    if (mod.actionId === 'custom-models' && onSelectTool) {
                      onSelectTool(3)
                    } else if (mod.actionId === 'switcher' && onSelectTool) {
                      onSelectTool(0)
                    }
                  }}
                  className="btn-pill-tonal"
                  style={{ padding: '5px 14px', fontSize: '11px' }}
                >
                  {mod.actionId === 'custom-models' ? 'Configure Provider' : 'Manage Module'}
                </button>
              </div>
            </div>
          )
        })}
      </div>
    </div>
  )
}
