import type { SecurityAuditReport } from '../types.ts'

export function resolveSecurityGrade(riskScore?: number | null, securityGrade?: string): string {
  if (securityGrade && securityGrade.trim()) return securityGrade.trim()
  const score = typeof riskScore === 'number' && !Number.isNaN(riskScore) ? riskScore : 0
  if (score <= 5) return 'A+'
  if (score <= 15) return 'A'
  if (score <= 30) return 'B'
  if (score <= 50) return 'C'
  if (score <= 70) return 'D'
  return 'F'
}

export function getRiskPresentation(riskLevel?: string) {
  const norm = (riskLevel || '').toLowerCase().trim()
  const isLow = norm === 'low'
  const isMed = norm === 'medium'
  const isHighOrCrit = norm === 'high' || norm === 'critical'

  const statusColor = isLow
    ? 'var(--green)'
    : isMed
    ? 'var(--yellow)'
    : isHighOrCrit
    ? 'var(--red)'
    : 'var(--text-muted)'
  const riskBadgeClass = isLow
    ? 'badge-green'
    : isMed
    ? 'badge-yellow'
    : isHighOrCrit
    ? 'badge-red'
    : 'badge-neutral'

  return {
    isLow,
    isMed,
    isHighOrCrit,
    statusColor,
    riskBadgeClass,
  }
}

export function formatSecurityReportMarkdown(report: SecurityAuditReport): string {
  const grade = resolveSecurityGrade(report.risk_score, report.security_grade)
  const probes = Array.isArray(report.probes) ? report.probes : []
  const recommendations = Array.isArray(report.recommendations) ? report.recommendations : []
  const passedCount = probes.filter((p) => p.status === 'passed').length

  const parsedDate = new Date(report.audited_at)
  const hasValidDate = Boolean(report.audited_at) && !Number.isNaN(parsedDate.getTime())
  const formattedFullDate = hasValidDate ? parsedDate.toLocaleString() : report.audited_at || '—'

  return [
    `# Security Audit Report`,
    `**Model:** ${report.model_id}`,
    `**Endpoint:** ${report.endpoint}`,
    `**Protocol:** ${report.provider_type}`,
    `**Audited:** ${formattedFullDate}`,
    `**Risk Level:** ${(report.risk_level || 'UNKNOWN').toUpperCase()} (Grade ${grade}, Score ${report.risk_score ?? 0}/100)`,
    '',
    `## Summary`,
    report.summary || 'No summary available.',
    '',
    `## Security Checks (${passedCount}/${probes.length} Passed)`,
    ...probes.map(
      (p) =>
        `- [${(p.status || 'unknown').toUpperCase()}] **${p.name}** (${p.category}): ${p.details}${
          p.evidence ? `\n  - Evidence: \`${p.evidence}\`` : ''
        }`
    ),
    '',
    `## Recommendations`,
    ...(recommendations.length > 0
      ? recommendations.map((r, idx) => `${idx + 1}. ${r}`)
      : ['No specific recommendations.']),
  ].join('\n')
}
