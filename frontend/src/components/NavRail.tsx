import React from 'react'
import type { Tool } from '../navigation/routes'
import type { Navigate } from '../navigation/useRouteNavigation'
import {
  Users,
  Settings,
  ShieldCheck,
  CheckCircle2,
  Circle,
  Cpu,
  Zap,
  Boxes,
  Coins,
  Wrench,
  Network,
  Globe,
} from 'lucide-react'
import type { SystemStatus } from '../types'

interface NavRailProps {
  currentTool: Tool
  navigate: Navigate
  status: SystemStatus | null
}

export const NavRail: React.FC<NavRailProps> = ({
  currentTool,
  navigate,
  status,
}) => {
  return (
    <aside
      style={{
        width: '200px',
        backgroundColor: 'var(--surface)',
        borderRight: '1px solid var(--border)',
        display: 'flex',
        flexDirection: 'column',
        flexShrink: 0,
        height: '100vh',
        boxSizing: 'border-box',
      }}
    >
      {/* Brand Header with mathematically aligned height and border */}
      <div
        style={{
          height: '72px',
          borderBottom: '1px solid var(--border)',
          boxSizing: 'border-box',
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
          justifyContent: 'center',
          padding: '0 12px',
          flexShrink: 0,
        }}
      >
        <div
          style={{
            width: 'fit-content',
            display: 'flex',
            flexDirection: 'column',
            alignItems: 'center',
          }}
        >
          <div
            id="brandTitle"
            style={{
              fontSize: '19px',
              fontWeight: 700,
              letterSpacing: '-0.2px',
              color: 'var(--text)',
              lineHeight: 1.15,
              whiteSpace: 'nowrap',
              textAlign: 'center',
            }}
          >
            Antigravity
          </div>
          <div
            id="brandSubtitle"
            style={{
              display: 'flex',
              justifyContent: 'space-between',
              alignItems: 'center',
              fontSize: '10.5px',
              fontWeight: 700,
              color: 'var(--primary)',
              textTransform: 'uppercase',
              marginTop: '2px',
              lineHeight: 1.15,
              width: '100%',
            }}
          >
            {'SWISS KNIFE'.split('').map((char, i) => (
              <span key={i} style={{ display: 'inline-block' }}>
                {char === ' ' ? '\u00A0' : char}
              </span>
            ))}
          </div>
        </div>
      </div>

      {/* Primary Navigation Content */}
      <div
        style={{
          display: 'flex',
          flexDirection: 'column',
          flex: 1,
          padding: '16px 12px',
          overflowY: 'auto',
          minHeight: 0,
        }}
      >

      {/* Primary Navigation Items */}
      <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
        <button
          id="btnNavAccountSwitcher"
          onClick={() => navigate({ tool: 'switcher', tab: 'dashboard' })}
          style={{
            width: '100%',
            textAlign: 'left',
            padding: '9px 12px',
            borderRadius: '20px',
            fontSize: '13px',
            fontWeight: currentTool === 'switcher' ? 600 : 500,
            color: currentTool === 'switcher' ? 'var(--on-primary-container)' : 'var(--text-muted)',
            backgroundColor: currentTool === 'switcher' ? 'var(--primary-container)' : 'transparent',
            display: 'flex',
            alignItems: 'center',
            gap: '10px',
            whiteSpace: 'nowrap',
          }}
        >
          <Users size={18} color={currentTool === 'switcher' ? 'var(--primary)' : 'var(--text-muted)'} />
          Account Switcher
        </button>

        <button
          id="btnNavCustomModels"
          onClick={() => navigate({ tool: 'custom-models' })}
          style={{
            width: '100%',
            textAlign: 'left',
            padding: '9px 12px',
            borderRadius: '20px',
            fontSize: '13px',
            fontWeight: currentTool === 'custom-models' ? 600 : 500,
            color: currentTool === 'custom-models' ? 'var(--on-primary-container)' : 'var(--text-muted)',
            backgroundColor: currentTool === 'custom-models' ? 'var(--primary-container)' : 'transparent',
            display: 'flex',
            alignItems: 'center',
            gap: '10px',
            whiteSpace: 'nowrap',
          }}
        >
          <Cpu size={18} color={currentTool === 'custom-models' ? 'var(--primary)' : 'var(--text-muted)'} />
          Custom Models
        </button>

        <button
          id="btnNavTokenMonitor"
          onClick={() => navigate({ tool: 'token-monitor' })}
          style={{
            width: '100%',
            textAlign: 'left',
            padding: '9px 12px',
            borderRadius: '20px',
            fontSize: '13px',
            fontWeight: currentTool === 'token-monitor' ? 600 : 500,
            color: currentTool === 'token-monitor' ? 'var(--on-primary-container)' : 'var(--text-muted)',
            backgroundColor: currentTool === 'token-monitor' ? 'var(--primary-container)' : 'transparent',
            display: 'flex',
            alignItems: 'center',
            gap: '10px',
            whiteSpace: 'nowrap',
          }}
        >
          <Coins size={18} color={currentTool === 'token-monitor' ? 'var(--primary)' : 'var(--text-muted)'} />
          Token Monitor
        </button>

        <button
          id="btnNavBridge"
          data-testid="btnNavRemoteConnect"
          onClick={() => navigate({ tool: 'remote-connect' })}
          style={{
            width: '100%',
            textAlign: 'left',
            padding: '9px 12px',
            borderRadius: '20px',
            fontSize: '13px',
            fontWeight: currentTool === 'remote-connect' ? 600 : 500,
            color: currentTool === 'remote-connect' ? 'var(--on-primary-container)' : 'var(--text-muted)',
            backgroundColor: currentTool === 'remote-connect' ? 'var(--primary-container)' : 'transparent',
            display: 'flex',
            alignItems: 'center',
            gap: '10px',
            whiteSpace: 'nowrap',
          }}
        >
          <Globe size={18} color={currentTool === 'remote-connect' ? 'var(--primary)' : 'var(--text-muted)'} />
          Remote Connect
        </button>

        <button
          id="btnNavACPAgentMesh"
          data-testid="btnNavACPAgentMesh"
          onClick={() => navigate({ tool: 'acp-adapter' })}
          style={{
            width: '100%',
            textAlign: 'left',
            padding: '9px 12px',
            borderRadius: '20px',
            fontSize: '13px',
            fontWeight: currentTool === 'acp-adapter' ? 600 : 500,
            color: currentTool === 'acp-adapter' ? 'var(--on-primary-container)' : 'var(--text-muted)',
            backgroundColor: currentTool === 'acp-adapter' ? 'var(--primary-container)' : 'transparent',
            display: 'flex',
            alignItems: 'center',
            gap: '10px',
            whiteSpace: 'nowrap',
          }}
        >
          <Network size={18} color={currentTool === 'acp-adapter' ? 'var(--primary)' : 'var(--text-muted)'} />
          ACP Adapter
        </button>

        {Boolean(status?.daemon_running) && (
          <button
            id="btnNavExtensions"
            data-testid="btnNavExtensions"
            onClick={() => navigate({ tool: 'extensions' })}
            style={{
              width: '100%',
              textAlign: 'left',
              padding: '9px 12px',
              borderRadius: '20px',
              fontSize: '13px',
              fontWeight: currentTool === 'extensions' ? 600 : 500,
              color: currentTool === 'extensions' ? 'var(--on-primary-container)' : 'var(--text-muted)',
              backgroundColor: currentTool === 'extensions' ? 'var(--primary-container)' : 'transparent',
              display: 'flex',
              alignItems: 'center',
              gap: '10px',
              whiteSpace: 'nowrap',
            }}
          >
            <Boxes size={18} color={currentTool === 'extensions' ? 'var(--primary)' : 'var(--text-muted)'} />
            Extensions
          </button>
        )}
        <button
          id="btnNavEnhancements"
          onClick={() => navigate({ tool: 'enhancements' })}
          style={{
            width: '100%',
            textAlign: 'left',
            padding: '9px 12px',
            borderRadius: '20px',
            fontSize: '13px',
            fontWeight: currentTool === 'enhancements' ? 600 : 500,
            color: currentTool === 'enhancements' ? 'var(--on-primary-container)' : 'var(--text-muted)',
            backgroundColor: currentTool === 'enhancements' ? 'var(--primary-container)' : 'transparent',
            display: 'flex',
            alignItems: 'center',
            gap: '10px',
            whiteSpace: 'nowrap',
          }}
        >
          <Zap size={18} color={currentTool === 'enhancements' ? 'var(--primary)' : 'var(--text-muted)'} />
          UI Enhancements
        </button>

        <button
          id="btnNavUtilities"
          onClick={() => navigate({ tool: 'utilities', tab: 'cache' })}
          style={{
            width: '100%',
            textAlign: 'left',
            padding: '9px 12px',
            borderRadius: '20px',
            fontSize: '13px',
            fontWeight: currentTool === 'utilities' ? 600 : 500,
            color: currentTool === 'utilities' ? 'var(--on-primary-container)' : 'var(--text-muted)',
            backgroundColor: currentTool === 'utilities' ? 'var(--primary-container)' : 'transparent',
            display: 'flex',
            alignItems: 'center',
            gap: '10px',
            whiteSpace: 'nowrap',
          }}
        >
          <Wrench size={18} color={currentTool === 'utilities' ? 'var(--primary)' : 'var(--text-muted)'} />
          Utilities
        </button>
      </div>

      <div style={{ flex: 1 }} />

      {/* Bottom System Status Card */}
      <div
        style={{
          backgroundColor: 'var(--canvas)',
          border: '1px solid var(--border)',
          borderRadius: '12px',
          padding: '10px 12px',
          marginBottom: '10px',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '4px' }}>
          {status?.daemon_running ? (
            <CheckCircle2 size={14} color="var(--green)" />
          ) : (
            <Circle size={14} color="var(--text-subtle)" />
          )}
          <span
            style={{
              fontSize: '11px',
              fontWeight: 600,
              color: status?.daemon_running ? 'var(--green)' : 'var(--text-muted)',
            }}
          >
            {status?.daemon_running ? 'Daemon Active' : 'Daemon Inactive'}
          </span>
        </div>
        <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
          <ShieldCheck size={14} color="var(--primary)" />
          <span style={{ fontSize: '11px', color: 'var(--text-muted)' }}>
            {status?.antigravity_running
              ? `Host PID ${status.antigravity_pid}`
              : 'Host Idle'}
          </span>
        </div>
      </div>

      {/* Dedicated System Settings Button Pinned at Bottom-Left of Whole GUI */}
      <button
        id="btnNavSystemSettings"
        onClick={() => navigate({ tool: 'system-settings' })}
        style={{
          width: '100%',
          textAlign: 'left',
          padding: '9px 12px',
          borderRadius: '20px',
          fontSize: '13px',
          fontWeight: currentTool === 'system-settings' ? 600 : 500,
          color: currentTool === 'system-settings' ? 'var(--on-primary-container)' : 'var(--text-muted)',
          backgroundColor: currentTool === 'system-settings' ? 'var(--primary-container)' : 'transparent',
          display: 'flex',
          alignItems: 'center',
          gap: '10px',
          whiteSpace: 'nowrap',
        }}
      >
        <Settings size={18} color={currentTool === 'system-settings' ? 'var(--primary)' : 'var(--text-muted)'} />
        System Settings
      </button>
      </div>
    </aside>
  )
}
