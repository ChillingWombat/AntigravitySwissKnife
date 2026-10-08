import React, { useState, useEffect, useMemo } from 'react'
import {
  RefreshCw,
  Save,
  X,
  ShieldCheck,
} from 'lucide-react'
import type { AccountState, DeviceProfile } from '../types'
import { api } from '../api'
import { getAccountTableDisplay } from '../utils/accountPresentation'

interface FingerprintsPageProps {
  accounts: AccountState[]
}

export const FingerprintsPage: React.FC<FingerprintsPageProps> = ({ accounts }) => {
  const [profiles, setProfiles] = useState<DeviceProfile[]>([])
  const [editingAccount, setEditingAccount] = useState<AccountState | null>(null)
  const [selectedEmail, setSelectedEmail] = useState<string>('')
  const [machineId, setMachineId] = useState<string>('')
  const [updaterId, setUpdaterId] = useState<string>('')
  const [installationId, setInstallationId] = useState<string>('')
  const [installationUuid, setInstallationUuid] = useState<string>('')
  const [feedback, setFeedback] = useState<string | null>(null)
  const [isSaving, setIsSaving] = useState<boolean>(false)

  // Stable sort accounts by email to ensure deterministic table ordering
  const sortedAccounts = useMemo(() => {
    return [...accounts].sort((a, b) =>
      a.email.toLowerCase().localeCompare(b.email.toLowerCase())
    )
  }, [accounts])

  // Fetch profiles
  const loadProfiles = async () => {
    try {
      const data: any = await api.listFingerprints()
      if (Array.isArray(data)) {
        setProfiles(data)
      } else if (data && typeof data === 'object') {
        setProfiles([data])
      } else {
        setProfiles([])
      }
    } catch (err) {
      console.error(err)
      setProfiles([])
    }
  }

  useEffect(() => {
    loadProfiles()
  }, [])

  const generateRandomProfile = () => {
    const randomHex = (len: number) =>
      Array.from({ length: len }, () => Math.floor(Math.random() * 16).toString(16)).join('')
    const randomUuid = () =>
      'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, (c) => {
        const r = (Math.random() * 16) | 0
        const v = c === 'x' ? r : (r & 0x3) | 0x8
        return v.toString(16)
      })

    setMachineId(randomHex(64))
    setUpdaterId(randomUuid())
    setInstallationId(randomUuid())
    setInstallationUuid(randomUuid())
    setFeedback('Generated fresh randomized device identifiers.')
  }

  const handleOpenModal = (acc: AccountState) => {
    setEditingAccount(acc)
    setSelectedEmail(acc.email)
    setFeedback(null)
    const match = Array.isArray(profiles)
      ? profiles.find((p) => p.account_email?.toLowerCase() === acc.email.toLowerCase())
      : null
    if (match) {
      setMachineId(match.machine_id)
      setUpdaterId(match.updater_id)
      setInstallationId(match.installation_id)
      setInstallationUuid(match.installation_uuid)
    } else {
      generateRandomProfile()
    }
  }

  const handleCloseModal = () => {
    setEditingAccount(null)
    setFeedback(null)
  }

  const handleSave = async () => {
    const targetEmail = editingAccount?.email || selectedEmail
    if (!targetEmail) return
    setIsSaving(true)
    setFeedback(null)
    try {
      await api.saveFingerprint({
        account_email: targetEmail,
        machine_id: machineId,
        updater_id: updaterId,
        installation_id: installationId,
        installation_uuid: installationUuid,
      })
      setFeedback('Device profile saved and isolated successfully.')
      await loadProfiles()
    } catch (err: any) {
      setFeedback(`Save error: ${err.message}`)
    } finally {
      setIsSaving(false)
    }
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
      {/* Profile Mappings Inventory Table */}
      <div className="google-card" style={{ padding: '0px', overflow: 'hidden' }}>
        <div
          style={{
            padding: '16px 20px',
            borderBottom: '1px solid var(--border)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
          }}
        >
          <div>
            <div
              style={{
                fontSize: '11px',
                fontWeight: 700,
                color: 'var(--text-muted)',
                letterSpacing: '0.8px',
                textTransform: 'uppercase',
              }}
            >
              Profile Mappings Inventory
            </div>
            <div style={{ fontSize: '12px', color: 'var(--text-muted)', marginTop: '2px' }}>
              Click an account row to view and customize its isolated hardware profile
            </div>
          </div>
          <span className="badge-chip badge-neutral" style={{ fontSize: '11px' }}>
            {sortedAccounts.length} Accounts
          </span>
        </div>

        <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '13px' }}>
          <thead>
            <tr>
              <th
                style={{
                  padding: '12px 20px',
                  textAlign: 'left',
                  fontSize: '11px',
                  fontWeight: 600,
                  color: 'var(--text-muted)',
                  borderBottom: '1px solid var(--border)',
                  backgroundColor: 'var(--canvas)',
                  textTransform: 'uppercase',
                }}
              >
                Account Email
              </th>
              <th
                style={{
                  padding: '12px 14px',
                  textAlign: 'left',
                  fontSize: '11px',
                  fontWeight: 600,
                  color: 'var(--text-muted)',
                  borderBottom: '1px solid var(--border)',
                  backgroundColor: 'var(--canvas)',
                  textTransform: 'uppercase',
                }}
              >
                Machine ID (Truncated)
              </th>
              <th
                style={{
                  padding: '12px 14px',
                  textAlign: 'left',
                  fontSize: '11px',
                  fontWeight: 600,
                  color: 'var(--text-muted)',
                  borderBottom: '1px solid var(--border)',
                  backgroundColor: 'var(--canvas)',
                  textTransform: 'uppercase',
                }}
              >
                Installation ID
              </th>
              <th
                style={{
                  padding: '12px 20px',
                  textAlign: 'right',
                  fontSize: '11px',
                  fontWeight: 600,
                  color: 'var(--text-muted)',
                  borderBottom: '1px solid var(--border)',
                  backgroundColor: 'var(--canvas)',
                  textTransform: 'uppercase',
                }}
              >
                Isolation Status
              </th>
            </tr>
          </thead>
          <tbody>
            {sortedAccounts.map((acc) => {
              const prof = profiles.find(
                (p) => p.account_email?.toLowerCase() === acc.email.toLowerCase()
              )
              const display = getAccountTableDisplay(acc.label, acc.email)
              return (
                <tr
                  key={acc.email}
                  onClick={() => handleOpenModal(acc)}
                  style={{
                    borderBottom: '1px solid var(--border-subtle)',
                    cursor: 'pointer',
                    transition: 'background-color 0.15s ease',
                  }}
                  onMouseEnter={(e) => (e.currentTarget.style.backgroundColor = 'var(--canvas)')}
                  onMouseLeave={(e) => (e.currentTarget.style.backgroundColor = 'transparent')}
                >
                  <td style={{ padding: '12px 20px', overflow: 'hidden' }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                      <span
                        style={{
                          fontWeight: 600,
                          color: 'var(--text)',
                          fontSize: '13px',
                          whiteSpace: 'nowrap',
                          overflow: 'hidden',
                          textOverflow: 'ellipsis',
                        }}
                      >
                        {display.primaryText}
                      </span>
                    </div>
                    {display.secondaryText && (
                      <div
                        style={{
                          fontSize: '11px',
                          color: 'var(--text-muted)',
                          marginTop: '2px',
                          whiteSpace: 'nowrap',
                          overflow: 'hidden',
                          textOverflow: 'ellipsis',
                        }}
                      >
                        {display.secondaryText}
                      </div>
                    )}
                  </td>
                  <td
                    style={{
                      padding: '12px 14px',
                      fontFamily: 'monospace',
                      fontSize: '12px',
                      color: 'var(--text-muted)',
                    }}
                  >
                    {prof ? `${prof.machine_id.slice(0, 16)}...` : 'Virtual Default'}
                  </td>
                  <td
                    style={{
                      padding: '12px 14px',
                      fontFamily: 'monospace',
                      fontSize: '12px',
                      color: 'var(--text-muted)',
                    }}
                  >
                    {prof ? `${prof.installation_id.slice(0, 13)}...` : 'Virtual Default'}
                  </td>
                  <td style={{ padding: '12px 20px', textAlign: 'right' }}>
                    <span className="badge-chip badge-green">ISOLATED</span>
                  </td>
                </tr>
              )
            })}
          </tbody>
        </table>
      </div>

      {/* Device Fingerprint Edit Modal Window */}
      {editingAccount && (
        <div
          style={{
            position: 'fixed',
            inset: 0,
            backgroundColor: 'rgba(0, 0, 0, 0.45)',
            backdropFilter: 'blur(3px)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            zIndex: 1000,
          }}
          onClick={handleCloseModal}
        >
          <div
            className="google-card"
            style={{
              width: '640px',
              maxWidth: '94vw',
              maxHeight: '92vh',
              overflowY: 'auto',
              padding: '28px',
              backgroundColor: '#ffffff',
              borderRadius: '10px',
              boxShadow: 'var(--shadow-md)',
            }}
            onClick={(e) => e.stopPropagation()}
          >
            {/* Modal Header */}
            <div
              style={{
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'space-between',
                marginBottom: '20px',
              }}
            >
              <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
                <ShieldCheck size={22} style={{ color: 'var(--primary)' }} />
                <div>
                  <h3 style={{ margin: 0, fontSize: '16px', fontWeight: 700, color: 'var(--text)' }}>
                    Device Hardware Profile
                  </h3>
                  <div style={{ fontSize: '12px', color: 'var(--text-muted)', marginTop: '2px' }}>
                    {editingAccount.email}
                  </div>
                </div>
              </div>
              <button
                onClick={handleCloseModal}
                className="btn-pill-tonal"
                style={{
                  padding: '6px',
                  borderRadius: '50%',
                  minWidth: '32px',
                  height: '32px',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  cursor: 'pointer',
                }}
                title="Close"
              >
                <X size={16} />
              </button>
            </div>

            {/* Actions Bar */}
            <div
              style={{
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'space-between',
                marginBottom: '16px',
                padding: '12px 16px',
                backgroundColor: 'var(--canvas)',
                borderRadius: '8px',
                border: '1px solid var(--border)',
              }}
            >
              <div style={{ fontSize: '12px', color: 'var(--text-muted)' }}>
                Hardware isolation prevents sybil cross-correlation.
              </div>
              <div style={{ display: 'flex', gap: '8px' }}>
                <button
                  onClick={generateRandomProfile}
                  className="btn-pill-tonal"
                  style={{ fontSize: '12px', padding: '6px 12px' }}
                >
                  <RefreshCw size={13} /> Generate Fresh
                </button>
                <button
                  onClick={handleSave}
                  disabled={isSaving}
                  className="btn-pill-primary"
                  style={{ fontSize: '12px', padding: '6px 14px' }}
                >
                  <Save size={13} /> {isSaving ? 'Saving...' : 'Save Profile'}
                </button>
              </div>
            </div>

            {feedback && (
              <div
                style={{
                  fontSize: '12px',
                  color: feedback.includes('error') || feedback.includes('Error')
                    ? 'var(--danger, #d93025)'
                    : 'var(--primary)',
                  marginBottom: '16px',
                  fontWeight: 500,
                  padding: '8px 12px',
                  backgroundColor: feedback.includes('error') || feedback.includes('Error')
                    ? '#fce8e6'
                    : 'rgba(26, 115, 232, 0.08)',
                  borderRadius: '6px',
                }}
              >
                {feedback}
              </div>
            )}

            {/* Input Fields Grid */}
            <div style={{ display: 'flex', flexDirection: 'column', gap: '14px' }}>
              <div>
                <label
                  style={{
                    fontSize: '11px',
                    fontWeight: 600,
                    color: 'var(--text-muted)',
                    display: 'block',
                    marginBottom: '4px',
                  }}
                >
                  Machine ID (~/.config/Antigravity/machineid)
                </label>
                <input
                  type="text"
                  value={machineId}
                  onChange={(e) => setMachineId(e.target.value)}
                  style={{ width: '100%', fontFamily: 'monospace', fontSize: '12px' }}
                />
              </div>

              <div>
                <label
                  style={{
                    fontSize: '11px',
                    fontWeight: 600,
                    color: 'var(--text-muted)',
                    display: 'block',
                    marginBottom: '4px',
                  }}
                >
                  Updater ID (~/.config/Antigravity/.updaterId)
                </label>
                <input
                  type="text"
                  value={updaterId}
                  onChange={(e) => setUpdaterId(e.target.value)}
                  style={{ width: '100%', fontFamily: 'monospace', fontSize: '12px' }}
                />
              </div>

              <div>
                <label
                  style={{
                    fontSize: '11px',
                    fontWeight: 600,
                    color: 'var(--text-muted)',
                    display: 'block',
                    marginBottom: '4px',
                  }}
                >
                  Installation ID (~/.gemini/antigravity/installation_id)
                </label>
                <input
                  type="text"
                  value={installationId}
                  onChange={(e) => setInstallationId(e.target.value)}
                  style={{ width: '100%', fontFamily: 'monospace', fontSize: '12px' }}
                />
              </div>

              <div>
                <label
                  style={{
                    fontSize: '11px',
                    fontWeight: 600,
                    color: 'var(--text-muted)',
                    display: 'block',
                    marginBottom: '4px',
                  }}
                >
                  State UUID (~/.gemini/antigravity/antigravity_state.pbtxt)
                </label>
                <input
                  type="text"
                  value={installationUuid}
                  onChange={(e) => setInstallationUuid(e.target.value)}
                  style={{ width: '100%', fontFamily: 'monospace', fontSize: '12px' }}
                />
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
