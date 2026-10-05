import React, { useState, useEffect } from 'react'
import {
  RefreshCw,
  Save,
  CheckCircle2,
} from 'lucide-react'
import type { AccountState, DeviceProfile } from '../types'
import { api } from '../api'

interface FingerprintsPageProps {
  accounts: AccountState[]
}

export const FingerprintsPage: React.FC<FingerprintsPageProps> = ({ accounts }) => {
  const [selectedEmail, setSelectedEmail] = useState<string>(accounts[0]?.email || '')
  const [profiles, setProfiles] = useState<DeviceProfile[]>([])
  const [machineId, setMachineId] = useState<string>('')
  const [updaterId, setUpdaterId] = useState<string>('')
  const [installationId, setInstallationId] = useState<string>('')
  const [installationUuid, setInstallationUuid] = useState<string>('')
  const [feedback, setFeedback] = useState<string | null>(null)
  const [isSaving, setIsSaving] = useState<boolean>(false)

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

  // Load selected profile
  useEffect(() => {
    if (!selectedEmail) return
    const match = Array.isArray(profiles) ? profiles.find((p) => p.account_email === selectedEmail) : null
    if (match) {
      setMachineId(match.machine_id)
      setUpdaterId(match.updater_id)
      setInstallationId(match.installation_id)
      setInstallationUuid(match.installation_uuid)
    } else {
      generateRandomProfile()
    }
  }, [selectedEmail, profiles])

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
    setInstallationId(randomHex(32))
    setInstallationUuid(randomUuid())
    setFeedback('Generated fresh randomized device identifiers.')
  }

  const handleSave = async () => {
    if (!selectedEmail) return
    setIsSaving(true)
    setFeedback(null)
    try {
      await api.saveFingerprint({
        account_email: selectedEmail,
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
      {/* Header Info Card */}
      <div className="google-card" style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
        <div>
          <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.8px', textTransform: 'uppercase' }}>
            Device Fingerprint Virtualizer
          </div>
          <div style={{ fontSize: '13px', color: 'var(--text)', marginTop: '4px' }}>
            Isolates hardware and installation IDs per account to prevent sybil cross-correlation.
          </div>
        </div>

        <div className="badge-chip badge-green" style={{ padding: '6px 14px', fontSize: '12px', gap: '6px' }}>
          <CheckCircle2 size={15} />
          <span>Virtualization Ready</span>
        </div>
      </div>

      {/* Active Profile Inspector Card */}
      <div className="google-card">
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '16px' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
            <span style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)' }}>
              Target Account:
            </span>
            <select
              value={selectedEmail}
              onChange={(e) => setSelectedEmail(e.target.value)}
              style={{ minWidth: '220px' }}
            >
              {accounts.map((acc) => (
                <option key={acc.email} value={acc.email}>
                  {acc.email}
                </option>
              ))}
            </select>
          </div>

          <div style={{ display: 'flex', gap: '8px' }}>
            <button onClick={generateRandomProfile} className="btn-pill-tonal">
              <RefreshCw size={14} /> Generate Fresh Profile
            </button>
            <button onClick={handleSave} disabled={isSaving} className="btn-pill-primary">
              <Save size={14} /> Save Profile
            </button>
          </div>
        </div>

        {feedback && (
          <div style={{ fontSize: '12px', color: 'var(--primary)', marginBottom: '16px', fontWeight: 500 }}>
            {feedback}
          </div>
        )}

        {/* Input Fields Grid */}
        <div style={{ display: 'flex', flexDirection: 'column', gap: '14px' }}>
          <div>
            <label style={{ fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)', display: 'block', marginBottom: '4px' }}>
              Machine ID (~/.config/Antigravity/machineid)
            </label>
            <input
              type="text"
              value={machineId}
              onChange={(e) => setMachineId(e.target.value)}
              style={{ width: '100%', fontFamily: 'monospace' }}
            />
          </div>

          <div>
            <label style={{ fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)', display: 'block', marginBottom: '4px' }}>
              Updater ID (~/.config/Antigravity/.updaterId)
            </label>
            <input
              type="text"
              value={updaterId}
              onChange={(e) => setUpdaterId(e.target.value)}
              style={{ width: '100%', fontFamily: 'monospace' }}
            />
          </div>

          <div>
            <label style={{ fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)', display: 'block', marginBottom: '4px' }}>
              Installation ID (~/.gemini/antigravity/installation_id)
            </label>
            <input
              type="text"
              value={installationId}
              onChange={(e) => setInstallationId(e.target.value)}
              style={{ width: '100%', fontFamily: 'monospace' }}
            />
          </div>

          <div>
            <label style={{ fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)', display: 'block', marginBottom: '4px' }}>
              State UUID (~/.gemini/antigravity/antigravity_state.pbtxt)
            </label>
            <input
              type="text"
              value={installationUuid}
              onChange={(e) => setInstallationUuid(e.target.value)}
              style={{ width: '100%', fontFamily: 'monospace' }}
            />
          </div>
        </div>
      </div>

      {/* Profile Mappings Inventory Table */}
      <div className="google-card" style={{ padding: '0px', overflow: 'hidden' }}>
        <div style={{ padding: '16px 20px', borderBottom: '1px solid var(--border)' }}>
          <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.8px', textTransform: 'uppercase' }}>
            Profile Mappings Inventory
          </div>
        </div>

        <table style={{ width: '100%', borderCollapse: 'collapse' }}>
          <thead>
            <tr>
              <th style={{ padding: '12px 20px', textAlign: 'left', fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)', borderBottom: '1px solid var(--border)', backgroundColor: 'var(--canvas)', textTransform: 'uppercase' }}>
                Account Email
              </th>
              <th style={{ padding: '12px 14px', textAlign: 'left', fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)', borderBottom: '1px solid var(--border)', backgroundColor: 'var(--canvas)', textTransform: 'uppercase' }}>
                Machine ID (Truncated)
              </th>
              <th style={{ padding: '12px 20px', textAlign: 'right', fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)', borderBottom: '1px solid var(--border)', backgroundColor: 'var(--canvas)', textTransform: 'uppercase' }}>
                Isolation Status
              </th>
            </tr>
          </thead>
          <tbody>
            {accounts.map((acc) => {
              const prof = profiles.find((p) => p.account_email === acc.email)
              return (
                <tr key={acc.email} style={{ borderBottom: '1px solid var(--border-subtle)' }}>
                  <td style={{ padding: '12px 20px', fontWeight: 600, color: 'var(--text)' }}>
                    {acc.email}
                  </td>
                  <td style={{ padding: '12px 14px', fontFamily: 'monospace', fontSize: '12px', color: 'var(--text-muted)' }}>
                    {prof ? `${prof.machine_id.slice(0, 16)}...` : 'Virtual Default'}
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
    </div>
  )
}
