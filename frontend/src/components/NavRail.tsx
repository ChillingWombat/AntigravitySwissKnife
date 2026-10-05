import React from 'react'
import {
  Users,
  Grid,
  Settings,
  ShieldCheck,
  CheckCircle2,
  Circle,
  Cpu,
  Zap,
  Clock,
  Archive,
} from 'lucide-react'
import type { SystemStatus } from '../types'

interface NavRailProps {
  currentTool: number // 0: Switcher, 1: Marketplace, 2: System Settings, 3: Custom Models, 4: Enhancements, 5: Automations, 6: Archived Projects
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
        width: '220px',
        backgroundColor: '#ffffff',
        borderRight: '1px solid var(--border)',
        display: 'flex',
        flexDirection: 'column',
        padding: '24px 16px 20px',
        flexShrink: 0,
        height: '100vh',
        boxSizing: 'border-box',
      }}
    >
      {/* Brand Header with mathematically aligned widths */}
      <div
        style={{
          marginBottom: '20px',
          width: 'fit-content',
          alignSelf: 'center',
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
        }}
      >
        <div
          id="brandTitle"
          style={{
            fontSize: '18px',
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
            fontSize: '10px',
            fontWeight: 700,
            color: 'var(--primary)',
            textTransform: 'uppercase',
            marginTop: '3px',
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

      <div
        style={{
          height: '1px',
          backgroundColor: 'var(--border)',
          marginBottom: '16px',
        }}
      />

      {/* Nav Section Label */}
      <div
        style={{
          fontSize: '10px',
          fontWeight: 700,
          color: 'var(--text-muted)',
          letterSpacing: '1.2px',
          paddingLeft: '6px',
          marginBottom: '8px',
          textTransform: 'uppercase',
        }}
      >
        Navigation
      </div>

      {/* Primary Navigation Items */}
      <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
        <button
          onClick={() => onSelectTool(0)}
          style={{
            width: '100%',
            textAlign: 'left',
            padding: '10px 16px',
            borderRadius: '20px',
            fontSize: '13px',
            fontWeight: currentTool === 0 ? 600 : 500,
            color: currentTool === 0 ? 'var(--on-primary-container)' : 'var(--text-muted)',
            backgroundColor: currentTool === 0 ? 'var(--primary-container)' : 'transparent',
            display: 'flex',
            alignItems: 'center',
            gap: '12px',
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
            padding: '10px 16px',
            borderRadius: '20px',
            fontSize: '13px',
            fontWeight: currentTool === 3 ? 600 : 500,
            color: currentTool === 3 ? 'var(--on-primary-container)' : 'var(--text-muted)',
            backgroundColor: currentTool === 3 ? 'var(--primary-container)' : 'transparent',
            display: 'flex',
            alignItems: 'center',
            gap: '12px',
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
            padding: '10px 16px',
            borderRadius: '20px',
            fontSize: '13px',
            fontWeight: currentTool === 4 ? 600 : 500,
            color: currentTool === 4 ? 'var(--on-primary-container)' : 'var(--text-muted)',
            backgroundColor: currentTool === 4 ? 'var(--primary-container)' : 'transparent',
            display: 'flex',
            alignItems: 'center',
            gap: '12px',
          }}
        >
          <Zap size={18} color={currentTool === 4 ? 'var(--primary)' : 'var(--text-muted)'} />
          App Enhancements
        </button>

        <button
          id="btnNavAutomations"
          onClick={() => onSelectTool(5)}
          style={{
            width: '100%',
            textAlign: 'left',
            padding: '10px 16px',
            borderRadius: '20px',
            fontSize: '13px',
            fontWeight: currentTool === 5 ? 600 : 500,
            color: currentTool === 5 ? 'var(--on-primary-container)' : 'var(--text-muted)',
            backgroundColor: currentTool === 5 ? 'var(--primary-container)' : 'transparent',
            display: 'flex',
            alignItems: 'center',
            gap: '12px',
          }}
        >
          <Clock size={18} color={currentTool === 5 ? 'var(--primary)' : 'var(--text-muted)'} />
          Task Automations
        </button>

        <button
          onClick={() => onSelectTool(1)}
          style={{
            width: '100%',
            textAlign: 'left',
            padding: '10px 16px',
            borderRadius: '20px',
            fontSize: '13px',
            fontWeight: currentTool === 1 ? 600 : 500,
            color: currentTool === 1 ? 'var(--on-primary-container)' : 'var(--text-muted)',
            backgroundColor: currentTool === 1 ? 'var(--primary-container)' : 'transparent',
            display: 'flex',
            alignItems: 'center',
            gap: '12px',
          }}
        >
          <Grid size={18} color={currentTool === 1 ? 'var(--primary)' : 'var(--text-muted)'} />
          Tools Marketplace
        </button>

        <button
          id="btnNavArchivedProjects"
          onClick={() => onSelectTool(6)}
          style={{
            width: '100%',
            textAlign: 'left',
            padding: '10px 16px',
            borderRadius: '20px',
            fontSize: '13px',
            fontWeight: currentTool === 6 ? 600 : 500,
            color: currentTool === 6 ? 'var(--on-primary-container)' : 'var(--text-muted)',
            backgroundColor: currentTool === 6 ? 'var(--primary-container)' : 'transparent',
            display: 'flex',
            alignItems: 'center',
            gap: '12px',
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
          padding: '12px 14px',
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
          padding: '10px 16px',
          borderRadius: '20px',
          fontSize: '13px',
          fontWeight: currentTool === 2 ? 600 : 500,
          color: currentTool === 2 ? 'var(--on-primary-container)' : 'var(--text-muted)',
          backgroundColor: currentTool === 2 ? 'var(--primary-container)' : 'transparent',
          display: 'flex',
          alignItems: 'center',
          gap: '12px',
        }}
      >
        <Settings size={18} color={currentTool === 2 ? 'var(--primary)' : 'var(--text-muted)'} />
        System Settings
      </button>
    </aside>
  )
}
