import React, { useState, useEffect } from 'react'
import { createPortal } from 'react-dom'
import {
  ShieldCheck,
  ShieldAlert,
  Shield,
  AlertTriangle,
  CheckCircle2,
  AlertCircle,
  Copy,
  Check,
  X,
  Info,
} from 'lucide-react'
import type { SecurityAuditReport } from '../types'

interface SecurityReportModalProps {
  report: SecurityAuditReport
  onClose: () => void
}

export {
  resolveSecurityGrade,
  getRiskPresentation,
  formatSecurityReportMarkdown,
} from '../utils/securityAuditReport'
import {
  resolveSecurityGrade,
  getRiskPresentation,
  formatSecurityReportMarkdown,
} from '../utils/securityAuditReport'

export const SecurityReportModal: React.FC<SecurityReportModalProps> = ({ report, onClose }) => {
  const [copied, setCopied] = useState(false)

  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        onClose()
      }
    }
    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [onClose])

  const { isLow, isHighOrCrit, statusColor, riskBadgeClass } = getRiskPresentation(report.risk_level)
  const grade = resolveSecurityGrade(report.risk_score, report.security_grade)

  const probes = Array.isArray(report.probes) ? report.probes : []
  const recommendations = Array.isArray(report.recommendations) ? report.recommendations : []

  const passedCount = probes.filter((p) => p.status === 'passed').length
  const warningCount = probes.filter((p) => p.status === 'warning').length
  const failedCount = probes.filter((p) => p.status === 'failed').length

  const parsedDate = new Date(report.audited_at)
  const hasValidDate = Boolean(report.audited_at) && !Number.isNaN(parsedDate.getTime())
  const formattedTime = hasValidDate
    ? parsedDate.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })
    : report.audited_at || '—'
  const formattedFullDate = hasValidDate ? parsedDate.toLocaleString() : report.audited_at || '—'

  const handleCopyMarkdown = () => {
    const md = formatSecurityReportMarkdown(report)
    if (navigator?.clipboard?.writeText) {
      navigator.clipboard.writeText(md).then(() => {
        setCopied(true)
        setTimeout(() => setCopied(false), 2000)
      }).catch(() => {})
    } else {
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    }
  }

  const modalContent = (
    <div
      style={{
        position: 'fixed',
        inset: 0,
        backgroundColor: 'rgba(0, 0, 0, 0.45)',
        backdropFilter: 'blur(3px)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        zIndex: 1100,
      }}
      onClick={onClose}
    >
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby="security-report-modal-title"
        className="google-card"
        style={{
          width: '640px',
          maxWidth: '94vw',
          maxHeight: '90vh',
          overflowY: 'auto',
          padding: '24px',
          backgroundColor: 'var(--surface, #ffffff)',
          borderRadius: '10px',
          boxShadow: 'var(--shadow-md)',
          display: 'flex',
          flexDirection: 'column',
          gap: '16px',
        }}
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: '12px' }}>
          <div style={{ minWidth: 0 }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px', flexWrap: 'wrap' }}>
              <Shield size={18} style={{ color: 'var(--primary)', flexShrink: 0 }} />
              <h2 id="security-report-modal-title" style={{ margin: 0, fontSize: '18px', fontWeight: 700, color: 'var(--text)' }}>
                Security Audit Report
              </h2>
              <span className={`badge-chip ${riskBadgeClass}`} style={{ whiteSpace: 'nowrap' }}>
                {(report.risk_level || 'UNKNOWN').toUpperCase()} RISK
              </span>
              <span className="badge-chip badge-neutral" style={{ whiteSpace: 'nowrap' }}>
                Grade {grade}
              </span>
            </div>
            <div style={{ fontSize: '12px', color: 'var(--text-muted)', marginTop: '2px' }}>
              Endpoint transport encryption, origin lineage, and response integrity checks.
            </div>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="btn-pill-tonal"
            style={{
              padding: '6px',
              borderRadius: '50%',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              flexShrink: 0,
            }}
            title="Close"
          >
            <X size={16} />
          </button>
        </div>

        {/* Overview & Target Metadata Card */}
        <div
          style={{
            backgroundColor: 'var(--canvas)',
            border: '1px solid var(--border)',
            borderRadius: '8px',
            padding: '12px 16px',
            display: 'flex',
            flexDirection: 'column',
            gap: '12px',
          }}
        >
          {/* Posture Summary & Score Row */}
          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
              gap: '12px',
              flexWrap: 'wrap',
            }}
          >
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px', flex: '1 1 260px', minWidth: 0 }}>
              {isLow ? (
                <ShieldCheck size={16} style={{ color: statusColor, flexShrink: 0 }} />
              ) : isHighOrCrit ? (
                <ShieldAlert size={16} style={{ color: statusColor, flexShrink: 0 }} />
              ) : (
                <AlertTriangle size={16} style={{ color: statusColor, flexShrink: 0 }} />
              )}
              <span style={{ fontSize: '12.5px', fontWeight: 500, color: 'var(--text)', lineHeight: 1.4 }}>
                {report.summary}
              </span>
            </div>

            <div style={{ display: 'flex', alignItems: 'center', gap: '10px', flexShrink: 0 }}>
              <span style={{ fontSize: '11.5px', fontWeight: 600, color: 'var(--text-muted)', whiteSpace: 'nowrap' }}>
                Risk Score: <strong style={{ color: statusColor }}>{report.risk_score}/100</strong>
              </span>
              <div
                style={{
                  width: '72px',
                  height: '6px',
                  borderRadius: '3px',
                  backgroundColor: 'var(--border)',
                  overflow: 'hidden',
                }}
              >
                <div
                  style={{
                    width: `${Math.max(4, Math.min(100, report.risk_score))}%`,
                    height: '100%',
                    backgroundColor: statusColor,
                    borderRadius: '3px',
                    transition: 'width 0.3s ease',
                  }}
                />
              </div>
            </div>
          </div>

          <div style={{ borderTop: '1px solid var(--border)' }} />

          {/* 3-Column Target Metadata */}
          <div
            style={{
              display: 'grid',
              gridTemplateColumns: 'minmax(0, 1.8fr) minmax(0, 1fr) auto',
              gap: '12px',
              fontSize: '12px',
            }}
          >
            <div style={{ minWidth: 0 }}>
              <div style={{ fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '2px' }}>
                Endpoint
              </div>
              <div
                style={{
                  fontFamily: 'monospace',
                  fontSize: '11.5px',
                  color: 'var(--text)',
                  overflow: 'hidden',
                  textOverflow: 'ellipsis',
                  whiteSpace: 'nowrap',
                }}
                title={report.endpoint}
              >
                {report.endpoint}
              </div>
            </div>

            <div style={{ minWidth: 0 }}>
              <div style={{ fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '2px' }}>
                Model Identifier
              </div>
              <div
                style={{
                  fontFamily: 'monospace',
                  fontSize: '11.5px',
                  color: 'var(--text)',
                  overflow: 'hidden',
                  textOverflow: 'ellipsis',
                  whiteSpace: 'nowrap',
                }}
                title={report.model_id}
              >
                {report.model_id}
              </div>
            </div>

            <div>
              <div style={{ fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '2px' }}>
                Audited
              </div>
              <div style={{ fontSize: '11.5px', color: 'var(--text)', whiteSpace: 'nowrap' }} title={formattedFullDate}>
                {formattedTime}
              </div>
            </div>
          </div>
        </div>

        {/* Security Checks Table Card */}
        <div
          style={{
            border: '1px solid var(--border)',
            borderRadius: '8px',
            overflow: 'hidden',
            backgroundColor: 'var(--surface, #ffffff)',
          }}
        >
          <div
            style={{
              padding: '10px 16px',
              backgroundColor: 'var(--canvas)',
              borderBottom: '1px solid var(--border)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
              gap: '8px',
              flexWrap: 'wrap',
            }}
          >
            <span
              style={{
                fontSize: '11px',
                fontWeight: 700,
                color: 'var(--text-muted)',
                letterSpacing: '0.6px',
                textTransform: 'uppercase',
              }}
            >
              Security Checks ({probes.length})
            </span>

            {probes.length > 0 && (
              <div style={{ display: 'flex', alignItems: 'center', gap: '6px', flexWrap: 'wrap' }}>
                {passedCount > 0 && (
                  <span className="badge-chip badge-green" style={{ padding: '2px 8px', fontSize: '10.5px', whiteSpace: 'nowrap' }}>
                    {passedCount} Passed
                  </span>
                )}
                {warningCount > 0 && (
                  <span className="badge-chip badge-yellow" style={{ padding: '2px 8px', fontSize: '10.5px', whiteSpace: 'nowrap' }}>
                    {warningCount} Warning
                  </span>
                )}
                {failedCount > 0 && (
                  <span className="badge-chip badge-red" style={{ padding: '2px 8px', fontSize: '10.5px', whiteSpace: 'nowrap' }}>
                    {failedCount} Failed
                  </span>
                )}
              </div>
            )}
          </div>

          <div style={{ display: 'flex', flexDirection: 'column' }}>
            {probes.length === 0 ? (
              <div
                style={{
                  padding: '14px 16px',
                  fontSize: '12px',
                  color: 'var(--text-muted)',
                  backgroundColor: 'var(--surface, #ffffff)',
                }}
              >
                No probe details recorded for this snapshot.
              </div>
            ) : (
              probes.map((probe, idx) => {
              const pPassed = probe.status === 'passed'
              const pWarn = probe.status === 'warning'
              const chipClass = pPassed ? 'badge-green' : pWarn ? 'badge-yellow' : 'badge-red'

              return (
                <div
                  key={probe.id}
                  style={{
                    padding: '10px 16px',
                    borderBottom: idx < probes.length - 1 ? '1px solid var(--border-subtle)' : 'none',
                    backgroundColor: 'var(--surface, #ffffff)',
                    display: 'flex',
                    flexDirection: 'column',
                    gap: '3px',
                  }}
                >
                  <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '8px' }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '8px', minWidth: 0, flexWrap: 'wrap' }}>
                      {pPassed ? (
                        <CheckCircle2 size={14} style={{ color: 'var(--green)', flexShrink: 0 }} />
                      ) : pWarn ? (
                        <AlertCircle size={14} style={{ color: 'var(--yellow)', flexShrink: 0 }} />
                      ) : (
                        <AlertTriangle size={14} style={{ color: 'var(--red)', flexShrink: 0 }} />
                      )}
                      <span style={{ fontSize: '12.5px', fontWeight: 600, color: 'var(--text)' }}>
                        {probe.name}
                      </span>
                      <span
                        style={{
                          fontSize: '10.5px',
                          fontWeight: 500,
                          color: 'var(--text-muted)',
                          backgroundColor: 'var(--tonal)',
                          padding: '1px 6px',
                          borderRadius: '4px',
                          whiteSpace: 'nowrap',
                        }}
                      >
                        {probe.category}
                      </span>
                    </div>

                    <span
                      className={`badge-chip ${chipClass}`}
                      style={{
                        padding: '2px 8px',
                        fontSize: '10px',
                        textTransform: 'uppercase',
                        whiteSpace: 'nowrap',
                        flexShrink: 0,
                      }}
                    >
                      {probe.status}
                    </span>
                  </div>

                  <div style={{ paddingLeft: '22px', fontSize: '12px', color: 'var(--text-muted)', lineHeight: 1.45 }}>
                    {probe.details}
                  </div>

                  {probe.evidence && (
                    <div
                      style={{
                        marginLeft: '22px',
                        marginTop: '2px',
                        fontSize: '11px',
                        fontFamily: 'monospace',
                        color: 'var(--text-subtle)',
                        backgroundColor: 'var(--canvas)',
                        border: '1px solid var(--border-subtle)',
                        padding: '4px 8px',
                        borderRadius: '4px',
                        wordBreak: 'break-all',
                      }}
                    >
                      {probe.evidence}
                    </div>
                  )}
                </div>
              )
            })
            )}
          </div>
        </div>

        {/* Recommendations */}
        {recommendations.length > 0 && (
          <div
            style={{
              backgroundColor: 'var(--canvas)',
              border: '1px solid var(--border)',
              borderRadius: '8px',
              padding: '12px 16px',
              fontSize: '11.5px',
              color: 'var(--text-muted)',
              lineHeight: 1.5,
            }}
          >
            <div
              style={{
                fontWeight: 600,
                color: 'var(--text)',
                marginBottom: '4px',
                display: 'flex',
                alignItems: 'center',
                gap: '6px',
                fontSize: '12px',
              }}
            >
              <Info size={13} style={{ color: 'var(--primary)', flexShrink: 0 }} />
              <span>Recommendations</span>
            </div>
            <ul
              style={{
                margin: 0,
                paddingLeft: '16px',
                display: 'flex',
                flexDirection: 'column',
                gap: '3px',
              }}
            >
              {recommendations.map((rec, i) => (
                <li key={i}>{rec}</li>
              ))}
            </ul>
          </div>
        )}

        {/* Footer Actions */}
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            borderTop: '1px solid var(--border)',
            paddingTop: '16px',
            gap: '12px',
          }}
        >
          <button
            type="button"
            onClick={handleCopyMarkdown}
            className="btn-pill-tonal"
            style={{
              padding: '7px 16px',
              fontSize: '12px',
              display: 'inline-flex',
              alignItems: 'center',
              gap: '6px',
              whiteSpace: 'nowrap',
            }}
          >
            {copied ? <Check size={13} color="var(--green)" /> : <Copy size={13} />}
            <span>{copied ? 'Copied Report' : 'Copy Report'}</span>
          </button>

          <button
            type="button"
            onClick={onClose}
            className="btn-pill-primary"
            style={{ padding: '7px 20px', fontSize: '12px', whiteSpace: 'nowrap' }}
          >
            Close
          </button>
        </div>
      </div>
    </div>
  )

  if (typeof document !== 'undefined') {
    return createPortal(modalContent, document.body)
  }
  return modalContent
}
