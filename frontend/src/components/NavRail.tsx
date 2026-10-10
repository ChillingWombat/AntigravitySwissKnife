import React from 'react'
import {
  Users,
  Settings,
  ShieldCheck,
  CheckCircle2,
  Circle,
  Cpu,
  Zap,
  Clock,
  Archive,
  Boxes,
  Coins,
  Wrench,
} from 'lucide-react'
import type { SystemStatus } from '../types'

interface NavRailProps {
  currentTool: number // 0: Switcher, 2: System Settings, 3: Custom Models, 4: Enhancements, 5: Automations, 6: Archived Projects, 7: Extensions, 8: Token Monitor, 9: Utilities
  onSelectTool: (idx: number) => void
  status: SystemStatus | null
}

export const NavRail: React.FC<NavRailProps> = ({
  currentTool,
  onSelectTool,
  status,
}) => {
  return (
    <aside
      style={{
        width: '200px',
        backgroundColor: '#ffffff',
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
          onClick={() => onSelectTool(0)}
          style={{
            width: '100%',
            textAlign: 'left',
            padding: '9px 12px',
            borderRadius: '20px',
            fontSize: '13px',
            fontWeight: currentTool === 0 ? 600 : 500,
            color: currentTool === 0 ? 'var(--on-primary-container)' : 'var(--text-muted)',
            backgroundColor: currentTool === 0 ? 'var(--primary-container)' : 'transparent',
            display: 'flex',
            alignItems: 'center',
            gap: '10px',
            whiteSpace: 'nowrap',
          }}
        >
          <Users size={18} color={currentTool === 0 ? 'var(--primary)' : 'var(--text-muted)'} />
          Account Switcher
        </button>

        <button
          id="btnNavCustomModels"
          onClick={() => onSelectTool(3)}
          style={{
            width: '100%',
            textAlign: 'left',
            padding: '9px 12px',
            borderRadius: '20px',
            fontSize: '13px',
            fontWeight: currentTool === 3 ? 600 : 500,
            color: currentTool === 3 ? 'var(--on-primary-container)' : 'var(--text-muted)',
            backgroundColor: currentTool === 3 ? 'var(--primary-container)' : 'transparent',
            display: 'flex',
            alignItems: 'center',
            gap: '10px',
            whiteSpace: 'nowrap',
          }}
        >
          <Cpu size={18} color={currentTool === 3 ? 'var(--primary)' : 'var(--text-muted)'} />
          Custom Models
        </button>

        <button
          id="btnNavEnhancements"
          onClick={() => onSelectTool(4)}
          style={{
            width: '100%',
            textAlign: 'left',
            padding: '9px 12px',
            borderRadius: '20px',
            fontSize: '13px',
            fontWeight: currentTool === 4 ? 600 : 500,
            color: currentTool === 4 ? 'var(--on-primary-container)' : 'var(--text-muted)',
            backgroundColor: currentTool === 4 ? 'var(--primary-container)' : 'transparent',
            display: 'flex',
            alignItems: 'center',
            gap: '10px',
            whiteSpace: 'nowrap',
          }}
        >
          <Zap size={18} color={currentTool === 4 ? 'var(--primary)' : 'var(--text-muted)'} />
          UI Enhancements
        </button>

        {Boolean(status?.daemon_running) && (
          <button
            id="btnNavExtensions"
            data-testid="btnNavExtensions"
            onClick={() => onSelectTool(7)}
            style={{
              width: '100%',
              textAlign: 'left',
              padding: '9px 12px',
              borderRadius: '20px',
              fontSize: '13px',
              fontWeight: currentTool === 7 || currentTool === 10 ? 600 : 500,
              color: currentTool === 7 || currentTool === 10 ? 'var(--on-primary-container)' : 'var(--text-muted)',
              backgroundColor: currentTool === 7 || currentTool === 10 ? 'var(--primary-container)' : 'transparent',
              display: 'flex',
              alignItems: 'center',
              gap: '10px',
              whiteSpace: 'nowrap',
            }}
          >
            <Boxes size={18} color={currentTool === 7 || currentTool === 10 ? 'var(--primary)' : 'var(--text-muted)'} />
            Extensions
          </button>
        )}

        <button
          id="btnNavTokenMonitor"
          onClick={() => onSelectTool(8)}
          style={{
            width: '100%',
            textAlign: 'left',
            padding: '9px 12px',
            borderRadius: '20px',
            fontSize: '13px',
            fontWeight: currentTool === 8 ? 600 : 500,
            color: currentTool === 8 ? 'var(--on-primary-container)' : 'var(--text-muted)',
            backgroundColor: currentTool === 8 ? 'var(--primary-container)' : 'transparent',
            display: 'flex',
            alignItems: 'center',
            gap: '10px',
            whiteSpace: 'nowrap',
          }}
        >
          <Coins size={18} color={currentTool === 8 ? 'var(--primary)' : 'var(--text-muted)'} />
          Token Monitor
        </button>

        <button
          id="btnNavUtilities"
          onClick={() => onSelectTool(9)}
          style={{
            width: '100%',
            textAlign: 'left',
            padding: '9px 12px',
            borderRadius: '20px',
            fontSize: '13px',
            fontWeight: currentTool === 9 ? 600 : 500,
            color: currentTool === 9 ? 'var(--on-primary-container)' : 'var(--text-muted)',
            backgroundColor: currentTool === 9 ? 'var(--primary-container)' : 'transparent',
            display: 'flex',
            alignItems: 'center',
            gap: '10px',
            whiteSpace: 'nowrap',
          }}
        >
          <Wrench size={18} color={currentTool === 9 ? 'var(--primary)' : 'var(--text-muted)'} />
          Utilities
        </button>

        <button
          id="btnNavAutomations"
          onClick={() => onSelectTool(5)}
          style={{
            width: '100%',
            textAlign: 'left',
            padding: '9px 12px',
            borderRadius: '20px',
            fontSize: '13px',
            fontWeight: currentTool === 5 ? 600 : 500,
            color: currentTool === 5 ? 'var(--on-primary-container)' : 'var(--text-muted)',
            backgroundColor: currentTool === 5 ? 'var(--primary-container)' : 'transparent',
            display: 'flex',
            alignItems: 'center',
            gap: '10px',
            whiteSpace: 'nowrap',
          }}
        >
          <Clock size={18} color={currentTool === 5 ? 'var(--primary)' : 'var(--text-muted)'} />
          Task Automations
        </button>

        <button
          id="btnNavArchivedProjects"
          onClick={() => onSelectTool(6)}
          style={{
            width: '100%',
            textAlign: 'left',
            padding: '9px 12px',
            borderRadius: '20px',
            fontSize: '13px',
            fontWeight: currentTool === 6 ? 600 : 500,
            color: currentTool === 6 ? 'var(--on-primary-container)' : 'var(--text-muted)',
            backgroundColor: currentTool === 6 ? 'var(--primary-container)' : 'transparent',
            display: 'flex',
            alignItems: 'center',
            gap: '10px',
            whiteSpace: 'nowrap',
          }}
        >
          <Archive size={18} color={currentTool === 6 ? 'var(--primary)' : 'var(--text-muted)'} />
          Archived Projects
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
            {status?.daemon_running ? 'Daemon Active' : 'Standalone Mode'}
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
        onClick={() => onSelectTool(2)}
        style={{
          width: '100%',
          textAlign: 'left',
          padding: '9px 12px',
          borderRadius: '20px',
          fontSize: '13px',
          fontWeight: currentTool === 2 ? 600 : 500,
          color: currentTool === 2 ? 'var(--on-primary-container)' : 'var(--text-muted)',
          backgroundColor: currentTool === 2 ? 'var(--primary-container)' : 'transparent',
          display: 'flex',
          alignItems: 'center',
          gap: '10px',
          whiteSpace: 'nowrap',
        }}
      >
        <Settings size={18} color={currentTool === 2 ? 'var(--primary)' : 'var(--text-muted)'} />
        System Settings
      </button>
      </div>
    </aside>
  )
}
