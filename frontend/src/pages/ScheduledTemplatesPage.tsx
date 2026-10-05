import React, { useEffect, useState } from 'react'
import { Clock } from 'lucide-react'
import { api } from '../api'
import type { DeployTaskRequest, ScheduledTemplate, SidecarTaskInfo } from '../types'
import { formatSchedule } from '../utils/schedule'

const CATEGORIES = [
  'All',
  'Personal Assistant',
  'CI/CD & Development',
  'Security & Quality',
  'Research & Market',
]

export const ScheduledTemplatesPage: React.FC = () => {
  const [templates, setTemplates] = useState<ScheduledTemplate[]>([])
  const [sidecars, setSidecars] = useState<SidecarTaskInfo[]>([])
  const [selectedCategory, setSelectedCategory] = useState('All')
  const [searchQuery, setSearchQuery] = useState('')
  const [activeTab, setActiveTab] = useState<'catalog' | 'active'>('catalog')
  const [loading, setLoading] = useState(true)

  // Modal state
  const [selectedTemplate, setSelectedTemplate] = useState<ScheduledTemplate | null>(null)
  const [displayName, setDisplayName] = useState('')
  const [targetProject, setTargetProject] = useState('Antigravity Swiss Knife')
  const [cronExpression, setCronExpression] = useState('0 8 * * *')
  const [customPrompt, setCustomPrompt] = useState('')
  const [paramValues, setParamValues] = useState<Record<string, string>>({})
  const [deploying, setDeploying] = useState(false)
  const [modalMsg, setModalMsg] = useState<{ text: string; type: 'success' | 'error' } | null>(null)

  useEffect(() => {
    loadData()
  }, [])

  const loadData = async () => {
    try {
      setLoading(true)
      const [tList, sList] = await Promise.all([api.getTemplates(), api.getSidecars()])
      setTemplates(tList)
      setSidecars(sList)
    } catch (err: any) {
      console.error('Failed to load templates data:', err)
    } finally {
      setLoading(false)
    }
  }

  const openDeployModal = (t: ScheduledTemplate) => {
    setSelectedTemplate(t)
    setDisplayName(t.title)
    setCronExpression(t.default_schedule.cron_expression)
    setCustomPrompt(t.prompt_template)
    const initParams: Record<string, string> = {}
    t.parameters.forEach((p) => {
      initParams[p.key] = p.default_value
    })
    setParamValues(initParams)
    setModalMsg(null)
  }

  const handleDeploy = async () => {
    if (!selectedTemplate) return
    try {
      setDeploying(true)
      const req: DeployTaskRequest = {
        template_id: selectedTemplate.id,
        display_name: displayName,
        cron_expression: cronExpression,
        target_project: targetProject,
        custom_prompt: customPrompt,
        parameters: paramValues,
      }
      const res = await api.deployTemplate(req)
      setModalMsg({
        text: `Task successfully deployed to Antigravity as "${res.task.display_name}"!`,
        type: 'success',
      })
      await loadData()
      setTimeout(() => {
        setSelectedTemplate(null)
      }, 1500)
    } catch (err: any) {
      setModalMsg({ text: 'Deploy failed: ' + err.message, type: 'error' })
    } finally {
      setDeploying(false)
    }
  }

  const handleDeleteSidecar = async (id: string) => {
    if (!window.confirm(`Delete scheduled task "${id}"?`)) return
    try {
      await api.deleteSidecar(id)
      await loadData()
    } catch (err: any) {
      alert('Delete failed: ' + err.message)
    }
  }

  const filteredTemplates = templates.filter((t) => {
    const matchesCat = selectedCategory === 'All' || t.category === selectedCategory
    const q = searchQuery.toLowerCase()
    const matchesSearch =
      !q ||
      t.title.toLowerCase().includes(q) ||
      t.subtitle.toLowerCase().includes(q) ||
      t.description.toLowerCase().includes(q)
    return matchesCat && matchesSearch
  })

  if (loading) {
    return (
      <div style={{ padding: '32px', textAlign: 'center', color: '#64748b' }}>
        <div style={{ fontSize: '14px', fontWeight: 500 }}>Loading Scheduled Templates & Tasks...</div>
      </div>
    )
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
      {/* Top Header Card */}
      <div className="google-card" style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', flexWrap: 'wrap', gap: '16px' }}>
        <div>
          <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.8px', textTransform: 'uppercase' }}>
            Scheduled Task Templates
          </div>
          <div style={{ fontSize: '13px', color: 'var(--text)', marginTop: '4px' }}>
            Devin & Codex style automations for daily life, project health, market research, and CI/CD in Antigravity 2.0.
          </div>
        </div>

        {/* View toggle */}
        <div style={{ display: 'flex', background: 'var(--tab-inactive-bg)', borderRadius: '20px', padding: '3px', border: '1px solid var(--border)' }}>
          <button
            onClick={() => setActiveTab('catalog')}
            style={{
              padding: '6px 16px',
              borderRadius: '18px',
              border: 'none',
              background: activeTab === 'catalog' ? 'var(--surface)' : 'transparent',
              color: activeTab === 'catalog' ? 'var(--blue)' : 'var(--text-muted)',
              fontWeight: 600,
              fontSize: '12px',
              cursor: 'pointer',
              boxShadow: activeTab === 'catalog' ? '0 1px 3px rgba(0,0,0,0.08)' : 'none',
              transition: 'all 0.15s ease',
            }}
          >
            Template Catalog ({templates.length})
          </button>
          <button
            onClick={() => setActiveTab('active')}
            style={{
              padding: '6px 16px',
              borderRadius: '18px',
              border: 'none',
              background: activeTab === 'active' ? 'var(--surface)' : 'transparent',
              color: activeTab === 'active' ? 'var(--blue)' : 'var(--text-muted)',
              fontWeight: 600,
              fontSize: '12px',
              cursor: 'pointer',
              boxShadow: activeTab === 'active' ? '0 1px 3px rgba(0,0,0,0.08)' : 'none',
              transition: 'all 0.15s ease',
            }}
          >
            Active Tasks ({sidecars.length})
          </button>
        </div>
      </div>

      {activeTab === 'catalog' ? (
        <>
          {/* Category Filter Pills & Search */}
          <div
            style={{
              display: 'flex',
              justifyContent: 'space-between',
              alignItems: 'center',
              gap: '16px',
              marginBottom: '20px',
              flexWrap: 'wrap',
            }}
          >
            <div style={{ display: 'flex', gap: '8px', flexWrap: 'wrap' }}>
              {CATEGORIES.map((cat) => (
                <button
                  key={cat}
                  onClick={() => setSelectedCategory(cat)}
                  style={{
                    padding: '6px 14px',
                    borderRadius: '20px',
                    fontSize: '12px',
                    fontWeight: 600,
                    cursor: 'pointer',
                    border: selectedCategory === cat ? '1px solid #0b57d0' : '1px solid #e2e8f0',
                    background: selectedCategory === cat ? '#0b57d0' : '#ffffff',
                    color: selectedCategory === cat ? '#ffffff' : '#475569',
                    transition: 'all 0.15s ease',
                  }}
                >
                  {cat}
                </button>
              ))}
            </div>

            <input
              type="text"
              placeholder="Search templates..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              style={{
                padding: '7px 14px',
                borderRadius: '8px',
                border: '1px solid #cbd5e1',
                fontSize: '13px',
                width: '220px',
                outline: 'none',
              }}
            />
          </div>

          {/* Templates Grid */}
          <div
            style={{
              display: 'grid',
              gridTemplateColumns: 'repeat(auto-fill, minmax(320px, 1fr))',
              gap: '18px',
            }}
          >
            {filteredTemplates.map((t) => (
              <div
                key={t.id}
                onClick={() => openDeployModal(t)}
                style={{
                  background: '#ffffff',
                  borderRadius: '12px',
                  border: '1px solid #e2e8f0',
                  padding: '20px',
                  cursor: 'pointer',
                  display: 'flex',
                  flexDirection: 'column',
                  justifyContent: 'space-between',
                  boxShadow: '0 1px 3px rgba(0,0,0,0.03)',
                  transition: 'transform 0.15s, box-shadow 0.15s, border-color 0.15s',
                }}
                onMouseEnter={(e) => {
                  e.currentTarget.style.transform = 'translateY(-2px)'
                  e.currentTarget.style.boxShadow = '0 6px 18px rgba(0,0,0,0.08)'
                  e.currentTarget.style.borderColor = '#93c5fd'
                }}
                onMouseLeave={(e) => {
                  e.currentTarget.style.transform = 'translateY(0)'
                  e.currentTarget.style.boxShadow = '0 1px 3px rgba(0,0,0,0.03)'
                  e.currentTarget.style.borderColor = '#e2e8f0'
                }}
              >
                <div>
                  <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: '8px' }}>
                    <span
                      style={{
                        fontSize: '11px',
                        fontWeight: 700,
                        textTransform: 'uppercase',
                        padding: '2px 8px',
                        borderRadius: '6px',
                        background:
                          t.category === 'Personal Assistant'
                            ? '#ecfdf5'
                            : t.category === 'CI/CD & Development'
                            ? '#eff6ff'
                            : t.category === 'Security & Quality'
                            ? '#fef2f2'
                            : '#fefce8',
                        color:
                          t.category === 'Personal Assistant'
                            ? '#065f46'
                            : t.category === 'CI/CD & Development'
                            ? '#1e40af'
                            : t.category === 'Security & Quality'
                            ? '#991b1b'
                            : '#854d0e',
                      }}
                    >
                      {t.category}
                    </span>
                    <span
                      title={`Cron: ${t.default_schedule.cron_expression}`}
                      style={{ fontSize: '11px', color: 'var(--text-muted)', fontWeight: 600, display: 'flex', alignItems: 'center', gap: '4px' }}
                    >
                      <Clock size={12} />
                      <span>{formatSchedule(t.default_schedule)}</span>
                    </span>
                  </div>

                  <h3 style={{ margin: '0 0 4px', fontSize: '15px', fontWeight: 700, color: 'var(--text)' }}>
                    {t.title}
                  </h3>
                  <div style={{ fontSize: '12px', color: 'var(--blue)', fontWeight: 500, marginBottom: '8px' }}>
                    {t.subtitle}
                  </div>
                  <p style={{ margin: 0, fontSize: '12px', color: 'var(--text-muted)', lineHeight: 1.45 }}>
                    {t.description}
                  </p>
                </div>

                <div style={{ marginTop: '16px', borderTop: '1px solid var(--border)', paddingTop: '12px' }}>
                  <div style={{ display: 'flex', gap: '6px', flexWrap: 'wrap', marginBottom: '12px' }}>
                    {t.required_tools.map((tool) => (
                      <span
                        key={tool}
                        style={{
                          fontSize: '10px',
                          fontWeight: 600,
                          padding: '2px 7px',
                          borderRadius: '10px',
                          background: 'var(--tab-inactive-bg)',
                          color: 'var(--text-muted)',
                        }}
                      >
                        MCP: {tool}
                      </span>
                    ))}
                    {t.required_skills.map((skill) => (
                      <span
                        key={skill}
                        style={{
                          fontSize: '10px',
                          fontWeight: 600,
                          padding: '2px 7px',
                          borderRadius: '10px',
                          background: '#faf5ff',
                          color: '#7e22ce',
                        }}
                      >
                        Skill: {skill}
                      </span>
                    ))}
                  </div>

                  <button
                    className="btn-pill-tonal"
                    style={{
                      width: '100%',
                      padding: '7px 0',
                      fontSize: '12px',
                      justifyContent: 'center',
                    }}
                  >
                    Configure & Schedule &rarr;
                  </button>
                </div>
              </div>
            ))}
          </div>
        </>
      ) : (
        /* Active Sidecars List */
        <div className="google-card">
          <h2 style={{ margin: '0 0 16px', fontSize: '16px', fontWeight: 700, color: 'var(--text)' }}>
            Active Scheduled Tasks in Antigravity ({sidecars.length})
          </h2>
          {sidecars.length === 0 ? (
            <div style={{ textAlign: 'center', padding: '36px', color: 'var(--text-muted)', fontSize: '13px' }}>
              No scheduled sidecar tasks deployed yet. Pick a template from the catalog to schedule!
            </div>
          ) : (
            <div style={{ display: 'flex', flexDirection: 'column', gap: '12px' }}>
              {sidecars.map((sc) => (
                <div
                  key={sc.id}
                  style={{
                    display: 'flex',
                    justifyContent: 'space-between',
                    alignItems: 'center',
                    padding: '14px 18px',
                    borderRadius: '8px',
                    border: '1px solid #e2e8f0',
                    background: '#f8fafc',
                  }}
                >
                  <div>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '10px', marginBottom: '4px' }}>
                      <span style={{ fontSize: '14px', fontWeight: 700, color: '#0f172a' }}>{sc.display_name}</span>
                      <span
                        title={`Cron: ${sc.cron_expression}`}
                        style={{
                          fontSize: '11px',
                          padding: '2px 8px',
                          borderRadius: '6px',
                          background: '#e0e7ff',
                          color: '#3730a3',
                          fontWeight: 600,
                          display: 'inline-flex',
                          alignItems: 'center',
                          gap: '4px',
                        }}
                      >
                        <Clock size={11} />
                        <span>{sc.schedule_text || formatSchedule(sc.cron_expression)}</span>
                      </span>
                    </div>
                    <div style={{ fontSize: '12px', color: '#64748b', maxWidth: '640px' }}>
                      {sc.prompt_preview}
                    </div>
                  </div>

                  <button
                    onClick={() => handleDeleteSidecar(sc.id)}
                    style={{
                      padding: '6px 12px',
                      borderRadius: '6px',
                      border: '1px solid #fecaca',
                      background: '#fef2f2',
                      color: '#dc2626',
                      fontSize: '12px',
                      fontWeight: 600,
                      cursor: 'pointer',
                    }}
                  >
                    Delete Task
                  </button>
                </div>
              ))}
            </div>
          )}
        </div>
      )}

      {/* Deployment & Settings Pop-up Modal Window */}
      {selectedTemplate && (
        <div
          style={{
            position: 'fixed',
            inset: 0,
            background: 'rgba(15, 23, 42, 0.45)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            zIndex: 1000,
            padding: '20px',
          }}
        >
          <div
            style={{
              background: '#ffffff',
              borderRadius: '16px',
              maxWidth: '680px',
              width: '100%',
              maxHeight: '90vh',
              overflowY: 'auto',
              padding: '28px',
              boxShadow: '0 20px 40px rgba(0,0,0,0.2)',
            }}
          >
            {/* Modal Header */}
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: '16px' }}>
              <div>
                <span
                  style={{
                    fontSize: '11px',
                    fontWeight: 700,
                    textTransform: 'uppercase',
                    color: '#0b57d0',
                    background: '#eff6ff',
                    padding: '2px 8px',
                    borderRadius: '4px',
                  }}
                >
                  {selectedTemplate.category}
                </span>
                <h2 style={{ margin: '6px 0 2px', fontSize: '18px', fontWeight: 700, color: '#0f172a' }}>
                  {selectedTemplate.title}
                </h2>
                <div style={{ fontSize: '13px', color: '#64748b' }}>{selectedTemplate.subtitle}</div>
              </div>
              <button
                onClick={() => setSelectedTemplate(null)}
                style={{
                  border: 'none',
                  background: 'transparent',
                  fontSize: '20px',
                  color: '#94a3b8',
                  cursor: 'pointer',
                  padding: '2px 8px',
                }}
              >
                &times;
              </button>
            </div>

            {modalMsg && (
              <div
                style={{
                  padding: '10px 14px',
                  borderRadius: '6px',
                  marginBottom: '16px',
                  fontSize: '13px',
                  fontWeight: 500,
                  background: modalMsg.type === 'success' ? '#f0fdf4' : '#fef2f2',
                  color: modalMsg.type === 'success' ? '#166534' : '#991b1b',
                  border: `1px solid ${modalMsg.type === 'success' ? '#bbf7d0' : '#fecaca'}`,
                }}
              >
                {modalMsg.text}
              </div>
            )}

            {/* Modal Body Settings */}
            <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
              <div>
                <label style={{ display: 'block', fontSize: '12px', fontWeight: 700, color: '#334155', marginBottom: '4px' }}>
                  Task Display Name
                </label>
                <input
                  type="text"
                  value={displayName}
                  onChange={(e) => setDisplayName(e.target.value)}
                  style={{
                    width: '100%',
                    padding: '8px 12px',
                    borderRadius: '6px',
                    border: '1px solid #cbd5e1',
                    fontSize: '13px',
                    boxSizing: 'border-box',
                  }}
                />
              </div>

              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '14px' }}>
                <div>
                  <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '4px' }}>
                    <label style={{ fontSize: '12px', fontWeight: 700, color: '#334155' }}>
                      Schedule (Cron)
                    </label>
                    <span style={{ fontSize: '11px', color: '#2563eb', fontWeight: 600, display: 'inline-flex', alignItems: 'center', gap: '3px' }}>
                      <Clock size={11} />
                      <span>{formatSchedule(cronExpression)}</span>
                    </span>
                  </div>
                  <input
                    type="text"
                    value={cronExpression}
                    onChange={(e) => setCronExpression(e.target.value)}
                    placeholder="0 8 * * *"
                    style={{
                      width: '100%',
                      padding: '8px 12px',
                      borderRadius: '6px',
                      border: '1px solid #cbd5e1',
                      fontSize: '13px',
                      fontFamily: 'monospace',
                      boxSizing: 'border-box',
                    }}
                  />
                  <div style={{ display: 'flex', gap: '6px', marginTop: '6px', flexWrap: 'wrap' }}>
                    {[
                      { label: '8:00 AM Daily', cron: '0 8 * * *' },
                      { label: 'Hourly', cron: '0 * * * *' },
                      { label: 'Weekdays 5:00 PM', cron: '0 17 * * 1-5' },
                    ].map((preset) => (
                      <button
                        key={preset.cron}
                        type="button"
                        onClick={() => setCronExpression(preset.cron)}
                        style={{
                          fontSize: '10px',
                          fontWeight: 600,
                          padding: '2px 7px',
                          borderRadius: '4px',
                          border: '1px solid #e2e8f0',
                          background: cronExpression === preset.cron ? '#eff6ff' : '#f8fafc',
                          color: cronExpression === preset.cron ? '#1d4ed8' : '#64748b',
                          cursor: 'pointer',
                        }}
                      >
                        {preset.label}
                      </button>
                    ))}
                  </div>
                </div>

                <div>
                  <label style={{ display: 'block', fontSize: '12px', fontWeight: 700, color: '#334155', marginBottom: '4px' }}>
                    Target Project Context
                  </label>
                  <input
                    type="text"
                    value={targetProject}
                    onChange={(e) => setTargetProject(e.target.value)}
                    style={{
                      width: '100%',
                      padding: '8px 12px',
                      borderRadius: '6px',
                      border: '1px solid #cbd5e1',
                      fontSize: '13px',
                      boxSizing: 'border-box',
                    }}
                  />
                </div>
              </div>

              {/* Dynamic Template Parameters */}
              {selectedTemplate.parameters.map((p) => (
                <div key={p.key}>
                  <label style={{ display: 'block', fontSize: '12px', fontWeight: 700, color: '#334155', marginBottom: '4px' }}>
                    {p.label}
                  </label>
                  {p.type === 'textarea' ? (
                    <textarea
                      rows={3}
                      value={paramValues[p.key] || ''}
                      onChange={(e) => setParamValues({ ...paramValues, [p.key]: e.target.value })}
                      style={{
                        width: '100%',
                        padding: '8px 12px',
                        borderRadius: '6px',
                        border: '1px solid #cbd5e1',
                        fontSize: '12px',
                        boxSizing: 'border-box',
                        fontFamily: 'inherit',
                      }}
                    />
                  ) : (
                    <input
                      type="text"
                      value={paramValues[p.key] || ''}
                      onChange={(e) => setParamValues({ ...paramValues, [p.key]: e.target.value })}
                      style={{
                        width: '100%',
                        padding: '8px 12px',
                        borderRadius: '6px',
                        border: '1px solid #cbd5e1',
                        fontSize: '12px',
                        boxSizing: 'border-box',
                      }}
                    />
                  )}
                  <span style={{ fontSize: '11px', color: '#64748b' }}>{p.description}</span>
                </div>
              ))}

              {/* Instructions / Prompt Template Editor */}
              <div>
                <label style={{ display: 'block', fontSize: '12px', fontWeight: 700, color: '#334155', marginBottom: '4px' }}>
                  Execution Instructions & Prompt (Editable)
                </label>
                <textarea
                  rows={8}
                  value={customPrompt}
                  onChange={(e) => setCustomPrompt(e.target.value)}
                  style={{
                    width: '100%',
                    padding: '8px 12px',
                    borderRadius: '6px',
                    border: '1px solid #cbd5e1',
                    fontSize: '12px',
                    fontFamily: 'monospace',
                    boxSizing: 'border-box',
                    lineHeight: 1.4,
                  }}
                />
              </div>
            </div>

            {/* Modal Footer Buttons */}
            <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '10px', marginTop: '24px' }}>
              <button
                onClick={() => setSelectedTemplate(null)}
                style={{
                  padding: '8px 16px',
                  borderRadius: '6px',
                  border: '1px solid #cbd5e1',
                  background: '#ffffff',
                  color: '#475569',
                  fontSize: '13px',
                  fontWeight: 600,
                  cursor: 'pointer',
                }}
              >
                Cancel
              </button>
              <button
                onClick={handleDeploy}
                disabled={deploying}
                style={{
                  padding: '8px 20px',
                  borderRadius: '6px',
                  border: 'none',
                  background: '#0b57d0',
                  color: '#ffffff',
                  fontSize: '13px',
                  fontWeight: 600,
                  cursor: 'pointer',
                  boxShadow: '0 2px 4px rgba(11,87,208,0.25)',
                }}
              >
                {deploying ? 'Deploying...' : 'Deploy to Antigravity'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
