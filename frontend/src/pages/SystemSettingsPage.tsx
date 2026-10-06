import React, { useState, useEffect } from 'react'
import {
  ShieldCheck,
  Copy,
  Check,
  RefreshCw,
  Monitor,
  Code,
  Terminal,
  Lock,
  Eye,
  EyeOff,
  KeyRound,
  HardDrive,
  Layers,
  Send,
  ExternalLink,
  Info,
  Bug,
  FolderOpen,
  Trash2,
  RotateCcw,
  Star,
  AlertTriangle,
  Globe,
  Folder,
  FileText,
  Mic,
  Bookmark,
  Filter,
} from 'lucide-react'
import type {
  SystemStatus,
  SystemInstallations,
  StorageInfo,
  PrivacySettings,
  DiagnosticResult,
} from '../types'
import { ToggleSwitch } from '../components/ToggleSwitch'
import { api } from '../api'
import soloCanImg from '../assets/solo_can.png'

interface SystemSettingsPageProps {
  status: SystemStatus | null
  onRefresh: () => void
  activeTab?: number
  onTabChange?: (tab: number) => void
}

export const SystemSettingsPage: React.FC<SystemSettingsPageProps> = ({
  status,
  onRefresh,
  activeTab = 0,
  onTabChange: _onTabChange,
}) => {
  const [internalTab] = useState<number>(0)
  const currentTab = typeof activeTab === 'number' ? activeTab : internalTab

  const [copiedKey, setCopiedKey] = useState<string | null>(null)
  const [installations, setInstallations] = useState<SystemInstallations | null>(null)
  const [isCheckingUpdates, setIsCheckingUpdates] = useState(false)
  const [updateFeedback, setUpdateFeedback] = useState<string | null>(null)

  // App Access Password state
  const [isPasswordEnabled, setIsPasswordEnabled] = useState(false)
  const [currentPassword, setCurrentPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [showCurrentPassword, setShowCurrentPassword] = useState(false)
  const [showNewPassword, setShowNewPassword] = useState(false)
  const [showConfirmPassword, setShowConfirmPassword] = useState(false)
  const [passwordFeedback, setPasswordFeedback] = useState<{ text: string; isError: boolean } | null>(null)
  const [isSubmittingPassword, setIsSubmittingPassword] = useState(false)

  // Desktop System Startup state
  const [startupEnabled, setStartupEnabled] = useState<boolean>(() => {
    return localStorage.getItem('antigravity_startup_enabled') === 'true'
  })
  const [closeToTrayEnabled, setCloseToTrayEnabled] = useState<boolean>(() => {
    return localStorage.getItem('antigravity_close_to_tray_enabled') === 'true'
  })

  // Preferred IDE state
  const [preferredIDE, setPreferredIDE] = useState<string>(() => {
    return localStorage.getItem('antigravity_preferred_ide') || 'code'
  })
  const [ideFeedback, setIdeFeedback] = useState<string | null>(null)
  const [isSavingIDE, setIsSavingIDE] = useState<boolean>(false)

  const handleSaveIDE = async (newIDE: string) => {
    setIsSavingIDE(true)
    setPreferredIDE(newIDE)
    localStorage.setItem('antigravity_preferred_ide', newIDE)
    try {
      await api.setPreferredIDE(newIDE)
      setIdeFeedback('Saved preferred IDE to configuration.')
    } catch {
      setIdeFeedback('Saved preferred IDE locally.')
    } finally {
      setIsSavingIDE(false)
      setTimeout(() => setIdeFeedback(null), 3000)
    }
  }

  // Storage and Path state
  const [storageInfo, setStorageInfo] = useState<StorageInfo | null>(null)
  const [selectedStorageMode, setSelectedStorageMode] = useState<'system_default' | 'app_portable'>('system_default')
  const [migrateData, setMigrateData] = useState<boolean>(true)
  const [isSavingStorage, setIsSavingStorage] = useState<boolean>(false)
  const [storageFeedback, setStorageFeedback] = useState<{ text: string; isError: boolean } | null>(null)

  // Quick Memos Storage & Scope Settings state
  const [memoStorageLocation, setMemoStorageLocation] = useState<'global' | 'project'>(() => {
    return (localStorage.getItem('antigravity_memo_storage_location') as 'global' | 'project') || 'global'
  })
  const [memoViewScope, setMemoViewScope] = useState<'all' | 'current'>(() => {
    return (localStorage.getItem('antigravity_memo_view_scope') as 'all' | 'current') || 'all'
  })
  const [memoSearchScope, setMemoSearchScope] = useState<'text' | 'all'>(() => {
    return (localStorage.getItem('antigravity_memo_search_scope') as 'text' | 'all') || 'text'
  })
  const [memoConfigFeedback, setMemoConfigFeedback] = useState<{ text: string; isError: boolean } | null>(null)
  const [isSavingMemoConfig, setIsSavingMemoConfig] = useState<boolean>(false)

  // 3 App Zones, Custom Paths & Per-Account Overrides state
  const [accounts, setAccounts] = useState<any[]>([])
  const [customPaths, setCustomPaths] = useState<Record<string, string>>({
    desktop: '',
    agy: '',
    vscode: '',
  })
  const [accountOverrideDrafts, setAccountOverrideDrafts] = useState<Record<string, { email: string; path: string }>>({
    desktop: { email: '', path: '' },
    agy: { email: '', path: '' },
    vscode: { email: '', path: '' },
  })
  const [pathFeedback, setPathFeedback] = useState<Record<string, { text: string; isError: boolean }>>({})
  const [cacheClearFeedback, setCacheClearFeedback] = useState<Record<string, { text: string; isError: boolean; isClearing?: boolean }>>({})

  // Clean State Restore / Factory Reset state
  const [showResetConfirm, setShowResetConfirm] = useState(false)
  const [isResetting, setIsResetting] = useState(false)
  const [resetFeedback, setResetFeedback] = useState<{ text: string; isError: boolean } | null>(null)

  // Privacy & Telemetry state
  const [, setPrivacySettings] = useState<PrivacySettings | null>(null)
  const [anonymousErrorReports, setAnonymousErrorReports] = useState<boolean>(true)
  const [anonymousTelemetry, setAnonymousTelemetry] = useState<boolean>(false)
  const [privacyFeedback, setPrivacyFeedback] = useState<{ text: string; isError: boolean } | null>(null)

  // Issue Diagnostic Gadget state
  const [issueDescription, setIssueDescription] = useState<string>('')
  const [includeSystemInfo, setIncludeSystemInfo] = useState<boolean>(true)
  const [includeLogs, setIncludeLogs] = useState<boolean>(true)
  const [isDiagnosing, setIsDiagnosing] = useState<boolean>(false)
  const [diagResult, setDiagResult] = useState<DiagnosticResult | null>(null)
  const [diagError, setDiagError] = useState<string | null>(null)
  const [copiedReport, setCopiedReport] = useState<boolean>(false)

  useEffect(() => {
    const electronAPI = (window as any).electronAPI
    if (electronAPI?.getStartupSetting) {
      electronAPI
        .getStartupSetting()
        .then((res: any) => {
          if (res && typeof res.openAtLogin === 'boolean') {
            setStartupEnabled(res.openAtLogin)
            localStorage.setItem('antigravity_startup_enabled', String(res.openAtLogin))
          }
        })
        .catch((err: any) => console.warn('Could not read startup setting:', err))
    }
    if (electronAPI?.getCloseToTraySetting) {
      electronAPI
        .getCloseToTraySetting()
        .then((res: any) => {
          if (res && typeof res.closeToTray === 'boolean') {
            setCloseToTrayEnabled(res.closeToTray)
            localStorage.setItem('antigravity_close_to_tray_enabled', String(res.closeToTray))
          }
        })
        .catch((err: any) => console.warn('Could not read close to tray setting:', err))
    }
  }, [])

  const handleToggleStartup = async (enabled: boolean) => {
    setStartupEnabled(enabled)
    localStorage.setItem('antigravity_startup_enabled', String(enabled))
    const electronAPI = (window as any).electronAPI
    if (electronAPI?.setStartupSetting) {
      try {
        await electronAPI.setStartupSetting(enabled)
      } catch (err: any) {
        console.warn('Failed to update startup setting:', err)
      }
    }
  }

  const handleToggleCloseToTray = async (enabled: boolean) => {
    setCloseToTrayEnabled(enabled)
    localStorage.setItem('antigravity_close_to_tray_enabled', String(enabled))
    const electronAPI = (window as any).electronAPI
    if (electronAPI?.setCloseToTraySetting) {
      try {
        await electronAPI.setCloseToTraySetting(enabled)
      } catch (err: any) {
        console.warn('Failed to update close to tray setting:', err)
      }
    }
  }

  const loadInstallations = async () => {
    try {
      const data = await api.getInstallations()
      setInstallations(data)
    } catch (err: any) {
      console.error('Failed to load installations:', err)
    }
  }

  const loadPasswordSettings = async () => {
    try {
      const res = await api.getPasswordSettings()
      setIsPasswordEnabled(!!res.enabled)
    } catch (err: any) {
      console.error('Failed to load password settings:', err)
    }
  }

  const loadStorageSettings = async () => {
    try {
      const data = await api.getStorageSettings()
      setStorageInfo(data)
      if (data && data.storage_mode) {
        setSelectedStorageMode(data.storage_mode)
      }
      if (data?.app_zones) {
        setCustomPaths({
          desktop: data.app_zones.desktop?.custom_path || '',
          agy: data.app_zones.agy?.custom_path || '',
          vscode: data.app_zones.vscode?.custom_path || '',
        })
      }
    } catch (err: any) {
      console.error('Failed to load storage settings:', err)
    }

    try {
      const accList = await api.getAccounts()
      if (Array.isArray(accList)) {
        setAccounts(accList)
      }
    } catch (err: any) {
      console.error('Failed to load accounts for path overrides:', err)
    }
  }

  const loadPrivacySettings = async () => {
    try {
      const data = await api.getPrivacySettings()
      setPrivacySettings(data)
      if (data) {
        setAnonymousErrorReports(data.anonymous_error_reports)
        setAnonymousTelemetry(data.anonymous_telemetry)
      }
    } catch (err: any) {
      console.error('Failed to load privacy settings:', err)
    }
  }

  const loadMemoSettings = async () => {
    try {
      const res = await api.getMemoConfig()
      if (res && res.success && res.config) {
        const cfg = res.config
        if (cfg.storage_location === 'global' || cfg.storage_location === 'project') {
          setMemoStorageLocation(cfg.storage_location)
          localStorage.setItem('antigravity_memo_storage_location', cfg.storage_location)
        }
        if (cfg.view_scope === 'all' || cfg.view_scope === 'current') {
          setMemoViewScope(cfg.view_scope)
          localStorage.setItem('antigravity_memo_view_scope', cfg.view_scope)
        }
        if (cfg.search_scope === 'text' || cfg.search_scope === 'all') {
          setMemoSearchScope(cfg.search_scope)
          localStorage.setItem('antigravity_memo_search_scope', cfg.search_scope)
        }
      }
    } catch (err: any) {
      console.error('Failed to load memo settings:', err)
    }
  }

  const handleUpdateMemoConfig = async (
    newStorage: 'global' | 'project',
    newView: 'all' | 'current',
    newSearch: 'text' | 'all'
  ) => {
    setMemoStorageLocation(newStorage)
    setMemoViewScope(newView)
    setMemoSearchScope(newSearch)
    localStorage.setItem('antigravity_memo_storage_location', newStorage)
    localStorage.setItem('antigravity_memo_view_scope', newView)
    localStorage.setItem('antigravity_memo_search_scope', newSearch)
    setIsSavingMemoConfig(true)
    try {
      const res = await api.updateMemoConfig({
        storage_location: newStorage,
        view_scope: newView,
        search_scope: newSearch,
      })
      if (res && res.success) {
        setMemoConfigFeedback({ text: 'Quick Memos settings updated and synchronized with backend.', isError: false })
      } else {
        setMemoConfigFeedback({ text: 'Settings updated locally.', isError: false })
      }
    } catch {
      setMemoConfigFeedback({ text: 'Saved settings locally (backend unreachable).', isError: false })
    } finally {
      setIsSavingMemoConfig(false)
      setTimeout(() => setMemoConfigFeedback(null), 3500)
    }
  }

  useEffect(() => {
    loadInstallations()
    loadPasswordSettings()
    loadStorageSettings()
    loadPrivacySettings()
    loadMemoSettings()
  }, [])

  const handleSetPassword = async () => {
    setPasswordFeedback(null)
    if (newPassword.length < 6) {
      setPasswordFeedback({ text: 'Password must be at least 6 characters long.', isError: true })
      return
    }
    if (newPassword !== confirmPassword) {
      setPasswordFeedback({ text: 'New passwords do not match.', isError: true })
      return
    }

    setIsSubmittingPassword(true)
    try {
      const res = await api.setPasswordSettings({
        password: newPassword,
        current_password: currentPassword,
      })
      if (res.success) {
        setIsPasswordEnabled(true)
        setNewPassword('')
        setConfirmPassword('')
        setCurrentPassword('')
        setPasswordFeedback({ text: 'Application access password updated successfully.', isError: false })
      } else {
        setPasswordFeedback({ text: res.error || 'Failed to set password.', isError: true })
      }
    } catch (err: any) {
      setPasswordFeedback({ text: err.message || 'Failed to update password.', isError: true })
    } finally {
      setIsSubmittingPassword(false)
    }
  }

  const handleRemovePassword = async () => {
    setPasswordFeedback(null)
    if (!currentPassword) {
      setPasswordFeedback({ text: 'Please enter your current password to remove protection.', isError: true })
      return
    }

    setIsSubmittingPassword(true)
    try {
      const res = await api.setPasswordSettings({
        remove: true,
        current_password: currentPassword,
      })
      if (res.success) {
        setIsPasswordEnabled(false)
        setNewPassword('')
        setConfirmPassword('')
        setCurrentPassword('')
        setPasswordFeedback({ text: 'Application access password removed.', isError: false })
      } else {
        setPasswordFeedback({ text: res.error || 'Failed to remove password.', isError: true })
      }
    } catch (err: any) {
      setPasswordFeedback({ text: err.message || 'Failed to remove password.', isError: true })
    } finally {
      setIsSubmittingPassword(false)
    }
  }

  const handleCheckUpdates = async () => {
    setIsCheckingUpdates(true)
    setUpdateFeedback(null)
    try {
      const data = await api.checkUpdates()
      setInstallations(data)
      setUpdateFeedback('Update check completed successfully.')
    } catch (err: any) {
      setUpdateFeedback(`Update check failed: ${err.message}`)
    } finally {
      setIsCheckingUpdates(false)
    }
  }

  const handleSaveStorageMode = async () => {
    setIsSavingStorage(true)
    setStorageFeedback(null)
    try {
      const res = await api.setStorageSettings({
        storage_mode: selectedStorageMode,
        migrate_data: migrateData,
      })
      if (res.success) {
        setStorageInfo(res.storage)
        setStorageFeedback({
          text: `Storage mode switched to ${selectedStorageMode === 'app_portable' ? 'Portable (Store with App)' : 'System Standard Folders'}${migrateData ? ' with data migration completed.' : '.'}`,
          isError: false,
        })
      } else {
        setStorageFeedback({ text: res.error || 'Failed to update storage mode.', isError: true })
      }
    } catch (err: any) {
      setStorageFeedback({ text: err.message || 'Failed to update storage mode.', isError: true })
    } finally {
      setIsSavingStorage(false)
    }
  }

  const handleSavePrivacyToggle = async (errorReports: boolean, telemetry: boolean) => {
    setAnonymousErrorReports(errorReports)
    setAnonymousTelemetry(telemetry)
    setPrivacyFeedback(null)
    try {
      const res = await api.setPrivacySettings({
        anonymous_error_reports: errorReports,
        anonymous_telemetry: telemetry,
      })
      if (res.success) {
        setPrivacyFeedback({ text: 'Privacy preferences saved.', isError: false })
        setTimeout(() => setPrivacyFeedback(null), 2500)
      } else {
        setPrivacyFeedback({ text: res.error || 'Failed to save privacy preferences.', isError: true })
      }
    } catch (err: any) {
      setPrivacyFeedback({ text: err.message || 'Failed to save privacy preferences.', isError: true })
    }
  }

  const handleRunDiagnostics = async () => {
    if (!issueDescription.trim()) {
      setDiagError('Please enter a brief description of the issue before diagnosing.')
      return
    }
    setIsDiagnosing(true)
    setDiagError(null)
    setDiagResult(null)
    try {
      const res = await api.runIssueDiagnosis({
        description: issueDescription.trim(),
        include_system_info: includeSystemInfo,
        include_logs: includeLogs,
      })
      if (res.success) {
        setDiagResult(res)
      } else {
        setDiagError(res.error || 'Diagnosis failed.')
      }
    } catch (err: any) {
      setDiagError(err.message || 'Diagnostics execution failed.')
    } finally {
      setIsDiagnosing(false)
    }
  }

  const handleOpenGitHubIssue = () => {
    if (diagResult?.issue_url) {
      const electronAPI = (window as any).electronAPI
      if (electronAPI?.openExternal) {
        electronAPI.openExternal(diagResult.issue_url)
      } else {
        window.open(diagResult.issue_url, '_blank', 'noopener,noreferrer')
      }
    }
  }

  const copyPath = (key: string, val: string) => {
    navigator.clipboard.writeText(val)
    setCopiedKey(key)
    setTimeout(() => setCopiedKey(null), 1500)
  }

  const copyReportText = () => {
    if (diagResult?.sanitized_report) {
      navigator.clipboard.writeText(diagResult.sanitized_report)
      setCopiedReport(true)
      setTimeout(() => setCopiedReport(false), 1500)
    }
  }

  const handleRefreshAll = () => {
    onRefresh()
    loadInstallations()
    loadStorageSettings()
    loadPrivacySettings()
  }

  const handleOpenExternal = (url: string) => {
    const electronAPI = (window as any).electronAPI
    if (electronAPI?.openExternal) {
      electronAPI.openExternal(url)
    } else {
      window.open(url, '_blank', 'noopener,noreferrer')
    }
  }

  const handleBrowsePath = async (appType: 'desktop' | 'agy' | 'vscode', isAccountOverride: boolean = false) => {
    const electronAPI = (window as any).electronAPI
    if (electronAPI?.selectPath) {
      const isDir = appType === 'vscode'
      const selected = await electronAPI.selectPath({
        directory: isDir,
        title: isDir ? `Select ${appType.toUpperCase()} Extension Directory` : `Select ${appType.toUpperCase()} Executable`,
      })
      if (selected) {
        if (isAccountOverride) {
          setAccountOverrideDrafts((prev) => ({
            ...prev,
            [appType]: { ...prev[appType], path: selected },
          }))
        } else {
          setCustomPaths((prev) => ({ ...prev, [appType]: selected }))
        }
      }
    } else {
      const currentVal = isAccountOverride
        ? accountOverrideDrafts[appType]?.path || ''
        : customPaths[appType] || ''
      const promptVal = window.prompt(`Enter full path for ${appType}:`, currentVal)
      if (promptVal !== null) {
        if (isAccountOverride) {
          setAccountOverrideDrafts((prev) => ({
            ...prev,
            [appType]: { ...prev[appType], path: promptVal.trim() },
          }))
        } else {
          setCustomPaths((prev) => ({ ...prev, [appType]: promptVal.trim() }))
        }
      }
    }
  }

  const handleSaveAppPath = async (appType: string) => {
    const path = customPaths[appType] || ''
    setPathFeedback((prev) => ({ ...prev, [appType]: { text: 'Saving path...', isError: false } }))
    try {
      const res = await api.saveAppPath(appType, path)
      if (res.success && res.storage) {
        setStorageInfo(res.storage)
        setPathFeedback((prev) => ({
          ...prev,
          [appType]: { text: path ? 'Custom executable path saved.' : 'Path reset to auto-detected default.', isError: false },
        }))
        setTimeout(() => setPathFeedback((prev) => ({ ...prev, [appType]: { text: '', isError: false } })), 3000)
      } else {
        setPathFeedback((prev) => ({
          ...prev,
          [appType]: { text: res.error || 'Failed to save path.', isError: true },
        }))
      }
    } catch (err: any) {
      setPathFeedback((prev) => ({
        ...prev,
        [appType]: { text: err.message || 'Failed to save path.', isError: true },
      }))
    }
  }

  const handleResetAppPath = async (appType: string) => {
    setCustomPaths((prev) => ({ ...prev, [appType]: '' }))
    try {
      const res = await api.saveAppPath(appType, '')
      if (res.success && res.storage) {
        setStorageInfo(res.storage)
        setPathFeedback((prev) => ({
          ...prev,
          [appType]: { text: 'Path reset to auto-detected default.', isError: false },
        }))
        setTimeout(() => setPathFeedback((prev) => ({ ...prev, [appType]: { text: '', isError: false } })), 3000)
      }
    } catch (err: any) {
      setPathFeedback((prev) => ({
        ...prev,
        [appType]: { text: err.message || 'Failed to reset path.', isError: true },
      }))
    }
  }

  const handleSaveAccountOverride = async (appType: string) => {
    const draft = accountOverrideDrafts[appType]
    if (!draft?.email || !draft?.path) {
      setPathFeedback((prev) => ({
        ...prev,
        [appType]: { text: 'Select an account and specify an executable path.', isError: true },
      }))
      return
    }
    try {
      const res = await api.saveAccountOverride(appType, draft.email, draft.path)
      if (res.success && res.storage) {
        setStorageInfo(res.storage)
        setAccountOverrideDrafts((prev) => ({
          ...prev,
          [appType]: { email: '', path: '' },
        }))
        setPathFeedback((prev) => ({
          ...prev,
          [appType]: { text: `Override applied for ${draft.email}.`, isError: false },
        }))
        setTimeout(() => setPathFeedback((prev) => ({ ...prev, [appType]: { text: '', isError: false } })), 3000)
      } else {
        setPathFeedback((prev) => ({
          ...prev,
          [appType]: { text: res.error || 'Failed to set account override.', isError: true },
        }))
      }
    } catch (err: any) {
      setPathFeedback((prev) => ({
        ...prev,
        [appType]: { text: err.message || 'Failed to set account override.', isError: true },
      }))
    }
  }

  const handleRemoveAccountOverride = async (appType: string, email: string) => {
    try {
      const res = await api.saveAccountOverride(appType, email, '')
      if (res.success && res.storage) {
        setStorageInfo(res.storage)
        setPathFeedback((prev) => ({
          ...prev,
          [appType]: { text: `Override removed for ${email}.`, isError: false },
        }))
        setTimeout(() => setPathFeedback((prev) => ({ ...prev, [appType]: { text: '', isError: false } })), 3000)
      }
    } catch (err: any) {
      setPathFeedback((prev) => ({
        ...prev,
        [appType]: { text: err.message || 'Failed to remove override.', isError: true },
      }))
    }
  }

  const handleClearAppCache = async (appType: string) => {
    setCacheClearFeedback((prev) => ({
      ...prev,
      [appType]: { text: 'Clearing application cache...', isError: false, isClearing: true },
    }))
    try {
      const res = await api.clearAppCache(appType)
      if (res.success) {
        const freedMb = (res.freed_bytes / (1024 * 1024)).toFixed(1)
        const msg =
          res.freed_bytes > 0
            ? `Cache cleared: freed ${freedMb} MB (${res.deleted_files} files removed).`
            : 'Cache already clean (no temporary files found).'
        setCacheClearFeedback((prev) => ({
          ...prev,
          [appType]: { text: msg, isError: false, isClearing: false },
        }))
      } else {
        setCacheClearFeedback((prev) => ({
          ...prev,
          [appType]: { text: res.error || 'Failed to clear cache.', isError: true, isClearing: false },
        }))
      }
    } catch (err: any) {
      setCacheClearFeedback((prev) => ({
        ...prev,
        [appType]: { text: err.message || 'Failed to clear cache.', isError: true, isClearing: false },
      }))
    }
  }

  const handleFactoryReset = async () => {
    setIsResetting(true)
    setResetFeedback(null)
    try {
      const res = await api.factoryReset()
      if (res.success) {
        setResetFeedback({ text: res.message, isError: false })
        setShowResetConfirm(false)
        loadStorageSettings()
        loadInstallations()
        onRefresh()
      } else {
        setResetFeedback({ text: res.error || 'Failed to restore clean unmodified status.', isError: true })
      }
    } catch (err: any) {
      setResetFeedback({ text: err.message || 'Failed to restore clean unmodified status.', isError: true })
    } finally {
      setIsResetting(false)
    }
  }

  const envPaths = [
    {
      key: 'config_dir',
      label: 'Configuration Directory:',
      val: storageInfo?.current_paths?.config_dir || '~/.config/antigravity-swiss',
    },
    {
      key: 'credentials',
      label: 'Credentials / Accounts File:',
      val: storageInfo?.current_paths?.credentials_path || '~/.config/antigravity-swiss/accounts.json',
    },
    {
      key: 'temp_dir',
      label: 'Temp & Cache Directory:',
      val: storageInfo?.current_paths?.temp_dir || '/tmp/antigravity-swiss',
    },
    {
      key: 'socket',
      label: 'Daemon IPC Socket:',
      val: storageInfo?.current_paths?.socket_path || '/run/user/1000/antigravity-swiss/daemon.sock',
    },
    {
      key: 'antigravity_bin',
      label: 'Antigravity Binary:',
      val: installations?.desktop_app?.path || '/opt/Antigravity/antigravity',
    },
    {
      key: 'antigravity_config',
      label: 'Antigravity Host Config:',
      val: '~/.config/Antigravity',
    },
  ]

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
      {/* Tab 0: General */}
      {currentTab === 0 && (
        <>
          {/* Header Info Card */}
          <div className="google-card" style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
            <div>
              <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.8px', textTransform: 'uppercase' }}>
                System & Process Overview
              </div>
              <div style={{ fontSize: '13px', color: 'var(--text)', marginTop: '4px' }}>
                Runtime diagnostics, IPC Unix domain sockets, and Antigravity process safety shield.
              </div>
              <div style={{ display: 'flex', gap: '16px', marginTop: '10px', fontSize: '12px', color: 'var(--text-muted)' }}>
                <span>
                  <strong>Daemon:</strong> {status?.daemon_running ? 'Online' : 'Stopped'}
                </span>
                <span>
                  <strong>Host Process:</strong> {status?.antigravity_running ? `Running (PID: ${status.antigravity_pid})` : 'Not running'}
                </span>
                <span>
                  <strong>Active Account:</strong> {status?.active_account || 'None'}
                </span>
              </div>
            </div>

            <button onClick={handleRefreshAll} className="btn-pill-tonal">
              <RefreshCw size={14} /> Refresh Diagnostics
            </button>
          </div>

          {/* Safety Shield Card */}
          <div className="google-card">
            <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.8px', textTransform: 'uppercase', marginBottom: '12px' }}>
              Process Safety Shield & Protection
            </div>

            <div
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: '12px',
                backgroundColor: 'var(--green-bg)',
                border: '1px solid #ceead6',
                borderRadius: '12px',
                padding: '16px 20px',
              }}
            >
              <ShieldCheck size={28} color="var(--green)" />
              <div>
                <div style={{ fontSize: '13px', fontWeight: 700, color: 'var(--green)' }}>
                  Host Process Safety Shield: ACTIVE
                </div>
                <div style={{ fontSize: '12px', color: 'var(--text-muted)', marginTop: '2px' }}>
                  Host Antigravity 2.0 PID {status?.antigravity_pid ? `(${status.antigravity_pid})` : ''} is protected against accidental termination signals.
                </div>
              </div>
            </div>
          </div>

          {/* System Startup & Desktop Integration Card */}
          <div className="google-card">
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '16px' }}>
              <div>
                <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.8px', textTransform: 'uppercase' }}>
                  System Startup & Desktop Integration
                </div>
                <div style={{ fontSize: '13px', color: 'var(--text)', marginTop: '4px' }}>
                  Configure automatic background startup and minimize-to-tray behavior on system login.
                </div>
              </div>
              <div className={`badge-chip ${startupEnabled ? 'badge-green' : 'badge-tonal'}`} style={{ fontSize: '12px', padding: '6px 14px' }}>
                <Monitor size={14} />
                <span>{startupEnabled ? 'Launch at Startup Active' : 'Manual Launch Only'}</span>
              </div>
            </div>

            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '14px 18px', backgroundColor: 'var(--canvas)', borderRadius: '10px' }}>
              <div>
                <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)' }}>
                  Launch at System Startup (Minimized to Tray)
                </div>
                <div style={{ fontSize: '12px', color: 'var(--text-muted)', marginTop: '2px' }}>
                  Automatically starts the Antigravity companion silently in your system tray when you log into Windows, macOS, or Linux.
                </div>
              </div>

              <ToggleSwitch
                size="md"
                checked={startupEnabled}
                onChange={handleToggleStartup}
              />
            </div>

            <div style={{ height: '1px', backgroundColor: 'var(--border, #e2e8f0)', margin: '12px 0' }} />

            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '14px 18px', backgroundColor: 'var(--canvas)', borderRadius: '10px' }}>
              <div>
                <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)' }}>
                  Keep Running in Background When Closed
                </div>
                <div style={{ fontSize: '12px', color: 'var(--text-muted)', marginTop: '2px' }}>
                  Minimizes to system tray and keeps the background daemon running when the window is closed. When disabled (default), closing the window completely terminates the desktop app and daemon.
                </div>
              </div>

              <ToggleSwitch
                size="md"
                checked={closeToTrayEnabled}
                onChange={handleToggleCloseToTray}
              />
            </div>
          </div>

          {/* App Access Password Protection Card */}
          <div className="google-card">
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '14px' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
                <div
                  style={{
                    width: '32px',
                    height: '32px',
                    borderRadius: '8px',
                    backgroundColor: isPasswordEnabled ? 'var(--primary-light)' : 'var(--canvas)',
                    border: '1px solid var(--border)',
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    color: isPasswordEnabled ? 'var(--primary)' : 'var(--text-muted)',
                  }}
                >
                  <Lock size={18} />
                </div>
                <div>
                  <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.8px', textTransform: 'uppercase' }}>
                    App Access Password Protection
                  </div>
                  <div style={{ fontSize: '12px', color: 'var(--text-muted)', marginTop: '2px' }}>
                    Require an entry password to unlock and use Antigravity Swiss Knife. Minimum 6 characters (numbers, letters, symbols).
                  </div>
                </div>
              </div>

              <span className={`badge-chip ${isPasswordEnabled ? 'badge-green' : 'badge-neutral'}`}>
                {isPasswordEnabled ? 'ENABLED / LOCKED' : 'OPTIONAL / DISABLED'}
              </span>
            </div>

            {passwordFeedback && (
              <div
                style={{
                  backgroundColor: passwordFeedback.isError ? 'var(--red-bg)' : 'var(--green-bg)',
                  color: passwordFeedback.isError ? 'var(--red)' : 'var(--green)',
                  padding: '10px 14px',
                  borderRadius: '8px',
                  fontSize: '12px',
                  marginBottom: '16px',
                }}
              >
                {passwordFeedback.text}
              </div>
            )}

            <div
              style={{
                backgroundColor: 'var(--canvas)',
                border: '1px solid var(--border)',
                borderRadius: '12px',
                padding: '16px',
                display: 'flex',
                flexDirection: 'column',
                gap: '12px',
              }}
            >
              {isPasswordEnabled && (
                <div>
                  <label style={{ display: 'flex', alignItems: 'center', gap: '6px', fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '6px' }}>
                    <KeyRound size={14} /> Current Password:
                  </label>
                  <div style={{ position: 'relative', display: 'flex', alignItems: 'center', maxWidth: '400px' }}>
                    <input
                      type={showCurrentPassword ? 'text' : 'password'}
                      placeholder="Enter current password to verify"
                      value={currentPassword}
                      onChange={(e) => setCurrentPassword(e.target.value)}
                      style={{ width: '100%', paddingRight: '40px' }}
                    />
                    <button
                      type="button"
                      onClick={() => setShowCurrentPassword(!showCurrentPassword)}
                      style={{
                        position: 'absolute',
                        right: '8px',
                        background: 'none',
                        border: 'none',
                        color: 'var(--text-muted)',
                        cursor: 'pointer',
                        display: 'flex',
                        alignItems: 'center',
                        padding: '4px',
                      }}
                      title={showCurrentPassword ? 'Hide password' : 'Show password'}
                    >
                      {showCurrentPassword ? <EyeOff size={16} /> : <Eye size={16} />}
                    </button>
                  </div>
                </div>
              )}

              <div>
                <label style={{ display: 'flex', alignItems: 'center', gap: '6px', fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '6px' }}>
                  <Lock size={14} /> {isPasswordEnabled ? 'New Password (Optional):' : 'Set App Password:'}
                </label>
                <div style={{ position: 'relative', display: 'flex', alignItems: 'center', maxWidth: '400px' }}>
                  <input
                    type={showNewPassword ? 'text' : 'password'}
                    placeholder="Min 6 characters (numbers, letters, symbols)"
                    value={newPassword}
                    onChange={(e) => setNewPassword(e.target.value)}
                    style={{ width: '100%', paddingRight: '40px' }}
                  />
                  <button
                    type="button"
                    onClick={() => setShowNewPassword(!showNewPassword)}
                    style={{
                      position: 'absolute',
                      right: '8px',
                      background: 'none',
                      border: 'none',
                      color: 'var(--text-muted)',
                      cursor: 'pointer',
                      display: 'flex',
                      alignItems: 'center',
                      padding: '4px',
                    }}
                    title={showNewPassword ? 'Hide password' : 'Show password'}
                  >
                    {showNewPassword ? <EyeOff size={16} /> : <Eye size={16} />}
                  </button>
                </div>
              </div>

              <div>
                <label style={{ display: 'flex', alignItems: 'center', gap: '6px', fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '6px' }}>
                  <Check size={14} /> Confirm Password:
                </label>
                <div style={{ position: 'relative', display: 'flex', alignItems: 'center', maxWidth: '400px' }}>
                  <input
                    type={showConfirmPassword ? 'text' : 'password'}
                    placeholder="Re-enter password to confirm"
                    value={confirmPassword}
                    onChange={(e) => setConfirmPassword(e.target.value)}
                    style={{ width: '100%', paddingRight: '40px' }}
                  />
                  <button
                    type="button"
                    onClick={() => setShowConfirmPassword(!showConfirmPassword)}
                    style={{
                      position: 'absolute',
                      right: '8px',
                      background: 'none',
                      border: 'none',
                      color: 'var(--text-muted)',
                      cursor: 'pointer',
                      display: 'flex',
                      alignItems: 'center',
                      padding: '4px',
                    }}
                    title={showConfirmPassword ? 'Hide password' : 'Show password'}
                  >
                    {showConfirmPassword ? <EyeOff size={16} /> : <Eye size={16} />}
                  </button>
                </div>
              </div>

              <div style={{ display: 'flex', gap: '10px', marginTop: '6px' }}>
                <button
                  onClick={handleSetPassword}
                  disabled={isSubmittingPassword || !newPassword}
                  className="btn-pill-primary"
                >
                  {isSubmittingPassword ? 'Saving...' : isPasswordEnabled ? 'Update Password' : 'Enable App Password'}
                </button>

                {isPasswordEnabled && (
                  <button
                    onClick={handleRemovePassword}
                    disabled={isSubmittingPassword}
                    className="btn-pill-danger"
                  >
                    Remove Password
                  </button>
                )}
              </div>
            </div>
          </div>

          {/* Preferred IDE Workspace Integration Card */}
          <div className="google-card">
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '14px' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
                <div
                  style={{
                    width: '32px',
                    height: '32px',
                    borderRadius: '8px',
                    backgroundColor: 'var(--primary-light)',
                    border: '1px solid var(--border)',
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    color: 'var(--primary)',
                  }}
                >
                  <Code size={18} />
                </div>
                <div>
                  <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.8px', textTransform: 'uppercase' }}>
                    IDE Workspace Integration
                  </div>
                  <div style={{ fontSize: '12px', color: 'var(--text-muted)', marginTop: '2px' }}>
                    Configure your preferred code editor or IDE when launching folders as workspaces from the File Explorer toolbar.
                  </div>
                </div>
              </div>

              <span className="badge-chip badge-green">
                ACTIVE: {preferredIDE.toUpperCase()}
              </span>
            </div>

            {ideFeedback && (
              <div
                style={{
                  backgroundColor: 'var(--green-bg)',
                  color: 'var(--green)',
                  padding: '10px 14px',
                  borderRadius: '8px',
                  fontSize: '12px',
                  marginBottom: '16px',
                }}
              >
                {ideFeedback}
              </div>
            )}

            <div
              style={{
                backgroundColor: 'var(--canvas)',
                border: '1px solid var(--border)',
                borderRadius: '12px',
                padding: '16px',
                display: 'flex',
                flexDirection: 'column',
                gap: '12px',
              }}
            >
              <div>
                <label style={{ display: 'block', fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '6px' }}>
                  Preferred IDE / Editor:
                </label>
                <div style={{ display: 'flex', gap: '10px', alignItems: 'center', maxWidth: '450px' }}>
                  <select
                    value={['code', 'cursor', 'windsurf', 'codium', 'zed'].includes(preferredIDE.toLowerCase()) ? preferredIDE.toLowerCase() : 'custom'}
                    onChange={(e) => {
                      if (e.target.value !== 'custom') {
                        handleSaveIDE(e.target.value)
                      }
                    }}
                    style={{ flex: 1, padding: '8px 12px', borderRadius: '8px', border: '1px solid var(--border)', backgroundColor: 'var(--canvas)', color: 'var(--text)', fontSize: '12px' }}
                  >
                    <option value="code">VS Code (`code`)</option>
                    <option value="cursor">Cursor (`cursor`)</option>
                    <option value="windsurf">Windsurf (`windsurf`)</option>
                    <option value="codium">VSCodium (`codium`)</option>
                    <option value="zed">Zed (`zed`)</option>
                    <option value="custom">Custom Command / Binary</option>
                  </select>
                </div>
              </div>

              <div>
                <label style={{ display: 'block', fontSize: '12px', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '6px' }}>
                  Command Binary / Executable:
                </label>
                <div style={{ display: 'flex', gap: '10px', maxWidth: '450px' }}>
                  <input
                    type="text"
                    value={preferredIDE}
                    onChange={(e) => setPreferredIDE(e.target.value)}
                    placeholder="e.g. code, cursor, windsurf, codium, zed"
                    style={{ flex: 1 }}
                  />
                  <button
                    onClick={() => handleSaveIDE(preferredIDE)}
                    disabled={isSavingIDE}
                    className="btn-pill-primary"
                    style={{ fontSize: '11px', padding: '6px 14px' }}
                  >
                    {isSavingIDE ? 'Saving...' : 'Save IDE'}
                  </button>
                </div>
              </div>
            </div>
          </div>
        </>
      )}

      {/* Tab 1: Path & Storage */}
      {currentTab === 1 && (
        <>
          {/* Storage Mode Selector Card */}
          <div className="google-card">
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '16px' }}>
              <div>
                <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.8px', textTransform: 'uppercase' }}>
                  Data & Storage Location
                </div>
                <div style={{ fontSize: '13px', color: 'var(--text)', marginTop: '4px' }}>
                  Choose where to store application configuration, keyring credentials, and temporary cache.
                </div>
              </div>

              <div className="badge-chip badge-tonal" style={{ fontSize: '11.5px', padding: '5px 12px' }}>
                <Layers size={13} />
                <span>
                  {storageInfo?.app_execution_type === 'unzipped_folder'
                    ? 'Unzipped Directory Runner'
                    : storageInfo?.app_execution_type === 'standalone_binary'
                    ? 'Standalone Binary File'
                    : 'System Package'}
                </span>
              </div>
            </div>

            {/* Execution Detail Banner */}
            <div
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: '10px',
                padding: '12px 16px',
                backgroundColor: 'var(--canvas)',
                borderRadius: '8px',
                border: '1px solid var(--border)',
                marginBottom: '18px',
                fontSize: '12px',
                color: 'var(--text-muted)',
              }}
            >
              <Info size={16} color="var(--primary)" style={{ flexShrink: 0 }} />
              <div>
                <strong>Detected Runtime Format:</strong> {storageInfo?.app_execution_detail || 'Standard executable runner.'}
                <div style={{ fontSize: '11.5px', marginTop: '2px', color: 'var(--text-subtle)' }}>
                  In portable mode, configuration and accounts are saved directly with the application in <code>./data/</code> without touching host system user directories.
                </div>
              </div>
            </div>

            {storageFeedback && (
              <div
                style={{
                  backgroundColor: storageFeedback.isError ? 'var(--red-bg)' : 'var(--green-bg)',
                  color: storageFeedback.isError ? 'var(--red)' : 'var(--green)',
                  padding: '10px 14px',
                  borderRadius: '8px',
                  fontSize: '12px',
                  marginBottom: '16px',
                }}
              >
                {storageFeedback.text}
              </div>
            )}

            {/* Options Grid */}
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '16px', marginBottom: '18px' }}>
              {/* Option 1: System Default */}
              <div
                onClick={() => setSelectedStorageMode('system_default')}
                style={{
                  border: selectedStorageMode === 'system_default' ? '2px solid var(--primary)' : '1px solid var(--border)',
                  backgroundColor: selectedStorageMode === 'system_default' ? 'rgba(26, 115, 232, 0.04)' : 'var(--canvas)',
                  borderRadius: '12px',
                  padding: '16px',
                  cursor: 'pointer',
                  display: 'flex',
                  flexDirection: 'column',
                  gap: '10px',
                  transition: 'all 0.15s ease',
                }}
              >
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                    <HardDrive size={18} color={selectedStorageMode === 'system_default' ? 'var(--primary)' : 'var(--text-muted)'} />
                    <span style={{ fontSize: '13.5px', fontWeight: 700, color: 'var(--text)' }}>
                      System Relevant Default Folders
                    </span>
                  </div>
                  <input
                    type="radio"
                    name="storage_mode"
                    checked={selectedStorageMode === 'system_default'}
                    onChange={() => setSelectedStorageMode('system_default')}
                    style={{ cursor: 'pointer' }}
                  />
                </div>

                <p style={{ margin: 0, fontSize: '12px', color: 'var(--text-muted)', lineHeight: 1.5 }}>
                  Store configuration, credentials, and temp files in OS-standard user directories (e.g., <code>~/.config/antigravity-swiss</code> on Linux, <code>%APPDATA%</code> on Windows).
                </p>

                <div style={{ borderTop: '1px solid var(--border)', paddingTop: '10px', fontSize: '11px', color: 'var(--text-subtle)' }}>
                  <div><strong>Config:</strong> {storageInfo?.system_default_paths?.config_dir || '~/.config/antigravity-swiss'}</div>
                  <div style={{ marginTop: '2px' }}><strong>Temp:</strong> {storageInfo?.system_default_paths?.temp_dir || '/tmp/antigravity-swiss'}</div>
                </div>
              </div>

              {/* Option 2: Store with App */}
              <div
                onClick={() => setSelectedStorageMode('app_portable')}
                style={{
                  border: selectedStorageMode === 'app_portable' ? '2px solid var(--primary)' : '1px solid var(--border)',
                  backgroundColor: selectedStorageMode === 'app_portable' ? 'rgba(26, 115, 232, 0.04)' : 'var(--canvas)',
                  borderRadius: '12px',
                  padding: '16px',
                  cursor: 'pointer',
                  display: 'flex',
                  flexDirection: 'column',
                  gap: '10px',
                  transition: 'all 0.15s ease',
                }}
              >
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                    <FolderOpen size={18} color={selectedStorageMode === 'app_portable' ? 'var(--primary)' : 'var(--text-muted)'} />
                    <span style={{ fontSize: '13.5px', fontWeight: 700, color: 'var(--text)' }}>
                      Store with the App (Portable Mode)
                    </span>
                  </div>
                  <input
                    type="radio"
                    name="storage_mode"
                    checked={selectedStorageMode === 'app_portable'}
                    onChange={() => setSelectedStorageMode('app_portable')}
                    style={{ cursor: 'pointer' }}
                  />
                </div>

                <p style={{ margin: 0, fontSize: '12px', color: 'var(--text-muted)', lineHeight: 1.5 }}>
                  Store data directly alongside the application files in <code>./data/</code>. Ideal for unzipped runner folders, portable USBs, or standalone binaries without touching host OS directories.
                </p>

                <div style={{ borderTop: '1px solid var(--border)', paddingTop: '10px', fontSize: '11px', color: 'var(--text-subtle)' }}>
                  <div><strong>Config:</strong> {storageInfo?.app_portable_paths?.config_dir || '<app_dir>/data'}</div>
                  <div style={{ marginTop: '2px' }}><strong>Temp:</strong> {storageInfo?.app_portable_paths?.temp_dir || '<app_dir>/data/temp'}</div>
                </div>
              </div>
            </div>

            {/* Migration Toggle and Action */}
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', borderTop: '1px solid var(--border)', paddingTop: '16px' }}>
              <label style={{ display: 'flex', alignItems: 'center', gap: '8px', fontSize: '12.5px', color: 'var(--text)', cursor: 'pointer' }}>
                <input
                  type="checkbox"
                  checked={migrateData}
                  onChange={(e) => setMigrateData(e.target.checked)}
                />
                <span>Migrate existing configuration and accounts (<code>accounts.json</code>) to newly selected location</span>
              </label>

              <button
                onClick={handleSaveStorageMode}
                disabled={isSavingStorage || selectedStorageMode === storageInfo?.storage_mode}
                className="btn-pill-primary"
                style={{ padding: '7px 20px', fontSize: '12.5px' }}
              >
                {isSavingStorage ? 'Applying...' : 'Apply Storage Setting'}
              </button>
            </div>
          </div>

          {/* Quick Memos Storage & Scope Settings Card */}
          <div className="google-card">
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '16px' }}>
              <div>
                <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.8px', textTransform: 'uppercase' }}>
                  Quick Memos Storage & Scope Settings
                </div>
                <div style={{ fontSize: '13px', color: 'var(--text)', marginTop: '4px' }}>
                  Configure workspace-specific vs. global storage persistence, view boundaries, and search scopes for Quick Memos.
                </div>
              </div>

              <div className="badge-chip badge-tonal" style={{ fontSize: '11.5px', padding: '5px 12px' }}>
                <Bookmark size={13} />
                <span>{memoStorageLocation === 'project' ? 'Per-Project Storage' : 'Global Shared'}</span>
              </div>
            </div>

            {memoConfigFeedback && (
              <div
                style={{
                  backgroundColor: memoConfigFeedback.isError ? 'var(--red-bg)' : 'var(--green-bg)',
                  color: memoConfigFeedback.isError ? 'var(--red)' : 'var(--green)',
                  padding: '10px 14px',
                  borderRadius: '8px',
                  fontSize: '12px',
                  marginBottom: '16px',
                  display: 'flex',
                  alignItems: 'center',
                  gap: '8px',
                }}
              >
                <Check size={14} />
                <span>{memoConfigFeedback.text}</span>
              </div>
            )}

            {/* 1. Storage Location */}
            <div style={{ marginBottom: '20px' }}>
              <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.8px', textTransform: 'uppercase', marginBottom: '10px' }}>
                Storage Location
              </div>
              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '16px' }}>
                {/* Global Shared Storage */}
                <div
                  onClick={() => handleUpdateMemoConfig('global', memoViewScope, memoSearchScope)}
                  style={{
                    border: memoStorageLocation === 'global' ? '2px solid var(--primary)' : '1px solid var(--border)',
                    backgroundColor: memoStorageLocation === 'global' ? 'rgba(26, 115, 232, 0.04)' : 'var(--canvas)',
                    borderRadius: '12px',
                    padding: '16px',
                    cursor: 'pointer',
                    display: 'flex',
                    flexDirection: 'column',
                    gap: '10px',
                    transition: 'all 0.15s ease',
                  }}
                >
                  <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                      <Globe size={18} color={memoStorageLocation === 'global' ? 'var(--primary)' : 'var(--text-muted)'} />
                      <span style={{ fontSize: '13.5px', fontWeight: 700, color: 'var(--text)' }}>
                        Global Shared Storage
                      </span>
                    </div>
                    <input
                      type="radio"
                      name="memo_storage_location"
                      checked={memoStorageLocation === 'global'}
                      onChange={() => handleUpdateMemoConfig('global', memoViewScope, memoSearchScope)}
                      style={{ cursor: 'pointer' }}
                    />
                  </div>
                  <p style={{ margin: 0, fontSize: '12px', color: 'var(--text-muted)', lineHeight: 1.5 }}>
                    Shared across all workspaces in central app configuration directory (<code>~/.config/antigravity-swiss/memos.json</code>).
                  </p>
                </div>

                {/* Per-Project Storage */}
                <div
                  onClick={() => handleUpdateMemoConfig('project', memoViewScope, memoSearchScope)}
                  style={{
                    border: memoStorageLocation === 'project' ? '2px solid var(--primary)' : '1px solid var(--border)',
                    backgroundColor: memoStorageLocation === 'project' ? 'rgba(26, 115, 232, 0.04)' : 'var(--canvas)',
                    borderRadius: '12px',
                    padding: '16px',
                    cursor: 'pointer',
                    display: 'flex',
                    flexDirection: 'column',
                    gap: '10px',
                    transition: 'all 0.15s ease',
                  }}
                >
                  <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                      <Folder size={18} color={memoStorageLocation === 'project' ? 'var(--primary)' : 'var(--text-muted)'} />
                      <span style={{ fontSize: '13.5px', fontWeight: 700, color: 'var(--text)' }}>
                        Per-Project Storage
                      </span>
                    </div>
                    <input
                      type="radio"
                      name="memo_storage_location"
                      checked={memoStorageLocation === 'project'}
                      onChange={() => handleUpdateMemoConfig('project', memoViewScope, memoSearchScope)}
                      style={{ cursor: 'pointer' }}
                    />
                  </div>
                  <p style={{ margin: 0, fontSize: '12px', color: 'var(--text-muted)', lineHeight: 1.5 }}>
                    Stored in <code>.antigravity/memos.json</code> within each project's workspace directory.
                  </p>
                </div>
              </div>
            </div>

            {/* 2. View Scope */}
            <div style={{ marginBottom: '20px' }}>
              <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.8px', textTransform: 'uppercase', marginBottom: '10px' }}>
                View Scope
              </div>
              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '16px' }}>
                {/* All Projects */}
                <div
                  onClick={() => handleUpdateMemoConfig(memoStorageLocation, 'all', memoSearchScope)}
                  style={{
                    border: memoViewScope === 'all' ? '2px solid var(--primary)' : '1px solid var(--border)',
                    backgroundColor: memoViewScope === 'all' ? 'rgba(26, 115, 232, 0.04)' : 'var(--canvas)',
                    borderRadius: '12px',
                    padding: '16px',
                    cursor: 'pointer',
                    display: 'flex',
                    flexDirection: 'column',
                    gap: '10px',
                    transition: 'all 0.15s ease',
                  }}
                >
                  <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                      <Layers size={18} color={memoViewScope === 'all' ? 'var(--primary)' : 'var(--text-muted)'} />
                      <span style={{ fontSize: '13.5px', fontWeight: 700, color: 'var(--text)' }}>
                        All Projects
                      </span>
                    </div>
                    <input
                      type="radio"
                      name="memo_view_scope"
                      checked={memoViewScope === 'all'}
                      onChange={() => handleUpdateMemoConfig(memoStorageLocation, 'all', memoSearchScope)}
                      style={{ cursor: 'pointer' }}
                    />
                  </div>
                  <p style={{ margin: 0, fontSize: '12px', color: 'var(--text-muted)', lineHeight: 1.5 }}>
                    Display all memos across all projects and global storage.
                  </p>
                </div>

                {/* Current Project Only */}
                <div
                  onClick={() => handleUpdateMemoConfig(memoStorageLocation, 'current', memoSearchScope)}
                  style={{
                    border: memoViewScope === 'current' ? '2px solid var(--primary)' : '1px solid var(--border)',
                    backgroundColor: memoViewScope === 'current' ? 'rgba(26, 115, 232, 0.04)' : 'var(--canvas)',
                    borderRadius: '12px',
                    padding: '16px',
                    cursor: 'pointer',
                    display: 'flex',
                    flexDirection: 'column',
                    gap: '10px',
                    transition: 'all 0.15s ease',
                  }}
                >
                  <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                      <Filter size={18} color={memoViewScope === 'current' ? 'var(--primary)' : 'var(--text-muted)'} />
                      <span style={{ fontSize: '13.5px', fontWeight: 700, color: 'var(--text)' }}>
                        Current Project Only
                      </span>
                    </div>
                    <input
                      type="radio"
                      name="memo_view_scope"
                      checked={memoViewScope === 'current'}
                      onChange={() => handleUpdateMemoConfig(memoStorageLocation, 'current', memoSearchScope)}
                      style={{ cursor: 'pointer' }}
                    />
                  </div>
                  <p style={{ margin: 0, fontSize: '12px', color: 'var(--text-muted)', lineHeight: 1.5 }}>
                    Restrict the memo view to the currently active project workspace.
                  </p>
                </div>
              </div>
            </div>

            {/* 3. Default Search Scope */}
            <div style={{ marginBottom: '16px' }}>
              <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.8px', textTransform: 'uppercase', marginBottom: '10px' }}>
                Default Search Scope
              </div>
              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '16px' }}>
                {/* Text Memos Only (Default) */}
                <div
                  onClick={() => handleUpdateMemoConfig(memoStorageLocation, memoViewScope, 'text')}
                  style={{
                    border: memoSearchScope === 'text' ? '2px solid var(--primary)' : '1px solid var(--border)',
                    backgroundColor: memoSearchScope === 'text' ? 'rgba(26, 115, 232, 0.04)' : 'var(--canvas)',
                    borderRadius: '12px',
                    padding: '16px',
                    cursor: 'pointer',
                    display: 'flex',
                    flexDirection: 'column',
                    gap: '10px',
                    transition: 'all 0.15s ease',
                  }}
                >
                  <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                      <FileText size={18} color={memoSearchScope === 'text' ? 'var(--primary)' : 'var(--text-muted)'} />
                      <span style={{ fontSize: '13.5px', fontWeight: 700, color: 'var(--text)' }}>
                        Text Memos Only (Default)
                      </span>
                    </div>
                    <input
                      type="radio"
                      name="memo_search_scope"
                      checked={memoSearchScope === 'text'}
                      onChange={() => handleUpdateMemoConfig(memoStorageLocation, memoViewScope, 'text')}
                      style={{ cursor: 'pointer' }}
                    />
                  </div>
                  <p style={{ margin: 0, fontSize: '12px', color: 'var(--text-muted)', lineHeight: 1.5 }}>
                    Search queries only match text memos.
                  </p>
                </div>

                {/* Text and Voice Memos */}
                <div
                  onClick={() => handleUpdateMemoConfig(memoStorageLocation, memoViewScope, 'all')}
                  style={{
                    border: memoSearchScope === 'all' ? '2px solid var(--primary)' : '1px solid var(--border)',
                    backgroundColor: memoSearchScope === 'all' ? 'rgba(26, 115, 232, 0.04)' : 'var(--canvas)',
                    borderRadius: '12px',
                    padding: '16px',
                    cursor: 'pointer',
                    display: 'flex',
                    flexDirection: 'column',
                    gap: '10px',
                    transition: 'all 0.15s ease',
                  }}
                >
                  <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                      <Mic size={18} color={memoSearchScope === 'all' ? 'var(--primary)' : 'var(--text-muted)'} />
                      <span style={{ fontSize: '13.5px', fontWeight: 700, color: 'var(--text)' }}>
                        Text and Voice Memos
                      </span>
                    </div>
                    <input
                      type="radio"
                      name="memo_search_scope"
                      checked={memoSearchScope === 'all'}
                      onChange={() => handleUpdateMemoConfig(memoStorageLocation, memoViewScope, 'all')}
                      style={{ cursor: 'pointer' }}
                    />
                  </div>
                  <p style={{ margin: 0, fontSize: '12px', color: 'var(--text-muted)', lineHeight: 1.5 }}>
                    Search queries match both text memos and transcribed voice memos.
                  </p>
                </div>
              </div>
            </div>

            {/* Card Footer Info */}
            <div style={{ borderTop: '1px solid var(--border)', paddingTop: '12px', fontSize: '11px', color: 'var(--text-subtle)', display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
              <div>
                Active storage: <code>{memoStorageLocation === 'project' ? '<workspace>/.antigravity/memos.json' : '~/.config/antigravity-swiss/memos.json'}</code>
              </div>
              <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                {isSavingMemoConfig && <RefreshCw size={12} className="animate-spin" />}
                <span>{isSavingMemoConfig ? 'Syncing...' : 'Synchronized with backend'}</span>
              </div>
            </div>
          </div>

          {/* 3 Application Zones Card */}
          <div className="google-card">
            <div style={{ marginBottom: '16px' }}>
              <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.8px', textTransform: 'uppercase' }}>
                Application Executable Paths & Cache Management (3 Zones)
              </div>
              <div style={{ fontSize: '13px', color: 'var(--text)', marginTop: '4px' }}>
                Auto-detected host binaries, custom path overrides, per-account executable assignments, and isolated cache cleanup.
              </div>
            </div>

            <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>
              {[
                {
                  appType: 'desktop' as const,
                  title: 'Antigravity 2.0 Desktop App',
                  icon: Monitor,
                  zone: storageInfo?.app_zones?.desktop,
                  placeholder: '/opt/Antigravity/antigravity or C:\\Program Files\\Antigravity\\...',
                },
                {
                  appType: 'agy' as const,
                  title: 'agy CLI',
                  icon: Terminal,
                  zone: storageInfo?.app_zones?.agy,
                  placeholder: '~/.local/bin/agy or /usr/local/bin/agy',
                },
                {
                  appType: 'vscode' as const,
                  title: 'VS Code Extension',
                  icon: Code,
                  zone: storageInfo?.app_zones?.vscode,
                  placeholder: '~/.vscode/extensions/...',
                },
              ].map(({ appType, title, icon: Icon, zone, placeholder }) => {
                const detectedPath = zone?.detected_path || 'Not detected'
                const isInstalled = zone?.installed ?? false
                const version = zone?.version || ''
                const currentVal = customPaths[appType] ?? (zone?.custom_path || '')
                const feedback = pathFeedback[appType]
                const cacheFeedback = cacheClearFeedback[appType]
                const draft = accountOverrideDrafts[appType] || { email: '', path: '' }
                const overrides = zone?.account_overrides || {}

                return (
                  <div
                    key={appType}
                    style={{
                      border: '1px solid var(--border)',
                      borderRadius: '12px',
                      padding: '18px',
                      backgroundColor: 'var(--canvas)',
                    }}
                  >
                    {/* Zone Header */}
                    <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '14px' }}>
                      <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
                        <div
                          style={{
                            width: '32px',
                            height: '32px',
                            borderRadius: '8px',
                            backgroundColor: 'rgba(26, 115, 232, 0.1)',
                            display: 'flex',
                            alignItems: 'center',
                            justifyContent: 'center',
                          }}
                        >
                          <Icon size={18} color="var(--primary)" />
                        </div>
                        <div>
                          <span style={{ fontSize: '14px', fontWeight: 700, color: 'var(--text)' }}>
                            {title}
                          </span>
                          <span style={{ fontSize: '11px', color: 'var(--text-muted)', marginLeft: '8px' }}>
                            ({appType.toUpperCase()})
                          </span>
                        </div>
                      </div>

                      <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                        {isInstalled ? (
                          <span className="badge-chip badge-green">
                            Installed {version ? `v${version}` : ''}
                          </span>
                        ) : (
                          <span className="badge-chip badge-neutral">Not Detected</span>
                        )}

                        <button
                          type="button"
                          onClick={() => handleClearAppCache(appType)}
                          disabled={cacheFeedback?.isClearing}
                          className="btn-pill-tonal"
                          style={{ padding: '5px 12px', fontSize: '11.5px', display: 'flex', alignItems: 'center', gap: '5px' }}
                          title={`Clear cache for ${title}`}
                        >
                          <Trash2 size={12} />
                          {cacheFeedback?.isClearing ? 'Clearing...' : 'Clear Cache'}
                        </button>
                      </div>
                    </div>

                    {/* Cache Feedback Banner */}
                    {cacheFeedback && (
                      <div
                        style={{
                          backgroundColor: cacheFeedback.isError ? 'var(--red-bg)' : 'var(--green-bg)',
                          color: cacheFeedback.isError ? 'var(--red)' : 'var(--green)',
                          padding: '7px 12px',
                          borderRadius: '6px',
                          fontSize: '11.5px',
                          marginBottom: '12px',
                          fontWeight: 500,
                        }}
                      >
                        {cacheFeedback.text}
                      </div>
                    )}

                    {/* Auto-Detected Path */}
                    <div style={{ marginBottom: '12px' }}>
                      <div style={{ fontSize: '11px', color: 'var(--text-muted)', fontWeight: 600, marginBottom: '4px' }}>
                        Auto-Detected Path:
                      </div>
                      <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                        <input
                          type="text"
                          readOnly
                          value={detectedPath}
                          style={{
                            flex: 1,
                            backgroundColor: '#ffffff',
                            fontFamily: 'monospace',
                            fontSize: '11.5px',
                            padding: '6px 10px',
                            borderRadius: '6px',
                            border: '1px solid var(--border)',
                            color: 'var(--text)',
                          }}
                        />
                        {detectedPath !== 'Not detected' && (
                          <button
                            type="button"
                            onClick={() => copyPath(`${appType}_detected`, detectedPath)}
                            className="btn-pill-tonal"
                            style={{ padding: '6px 10px', fontSize: '11px' }}
                            title="Copy path"
                          >
                            {copiedKey === `${appType}_detected` ? <Check size={12} /> : <Copy size={12} />}
                          </button>
                        )}
                      </div>
                    </div>

                    {/* Manual Executable Path Input */}
                    <div style={{ marginBottom: '14px' }}>
                      <div style={{ fontSize: '11px', color: 'var(--text-muted)', fontWeight: 600, marginBottom: '4px' }}>
                        Manual Custom Executable / Path Override:
                      </div>
                      <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                        <input
                          type="text"
                          value={currentVal}
                          onChange={(e) => setCustomPaths((prev) => ({ ...prev, [appType]: e.target.value }))}
                          placeholder={placeholder}
                          style={{
                            flex: 1,
                            backgroundColor: '#ffffff',
                            fontFamily: 'monospace',
                            fontSize: '11.5px',
                            padding: '6px 10px',
                            borderRadius: '6px',
                            border: '1px solid var(--border)',
                            color: 'var(--text)',
                          }}
                        />
                        <button
                          type="button"
                          onClick={() => handleBrowsePath(appType)}
                          className="btn-pill-tonal"
                          style={{ padding: '6px 12px', fontSize: '11.5px', display: 'flex', alignItems: 'center', gap: '5px' }}
                          title="Open file/folder picker"
                        >
                          <FolderOpen size={13} />
                          Browse
                        </button>
                        <button
                          type="button"
                          onClick={() => handleSaveAppPath(appType)}
                          className="btn-pill-primary"
                          style={{ padding: '6px 14px', fontSize: '11.5px' }}
                        >
                          Save Path
                        </button>
                        {Boolean(zone?.custom_path) && (
                          <button
                            type="button"
                            onClick={() => handleResetAppPath(appType)}
                            className="btn-pill-tonal"
                            style={{ padding: '6px 10px', fontSize: '11px', color: 'var(--text-muted)' }}
                            title="Reset to auto-detected default"
                          >
                            Reset
                          </button>
                        )}
                      </div>
                    </div>

                    {/* Path Feedback */}
                    {feedback && feedback.text && (
                      <div
                        style={{
                          fontSize: '11.5px',
                          color: feedback.isError ? 'var(--red)' : 'var(--green)',
                          marginBottom: '12px',
                          fontWeight: 500,
                        }}
                      >
                        {feedback.text}
                      </div>
                    )}

                    {/* Per-Account Executable Override Section */}
                    <div
                      style={{
                        borderTop: '1px solid var(--border)',
                        paddingTop: '12px',
                        marginTop: '12px',
                      }}
                    >
                      <div style={{ fontSize: '11.5px', fontWeight: 600, color: 'var(--text)', marginBottom: '4px' }}>
                        Optional Executable Override Per Account
                      </div>
                      <div style={{ fontSize: '11px', color: 'var(--text-subtle)', marginBottom: '8px' }}>
                        Run this application using a specific binary or directory when switching to a selected account.
                      </div>

                      <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '10px' }}>
                        <select
                          value={draft.email}
                          onChange={(e) =>
                            setAccountOverrideDrafts((prev) => ({
                              ...prev,
                              [appType]: { ...draft, email: e.target.value },
                            }))
                          }
                          style={{
                            minWidth: '180px',
                            padding: '6px 10px',
                            borderRadius: '6px',
                            border: '1px solid var(--border)',
                            backgroundColor: '#ffffff',
                            fontSize: '11.5px',
                            color: 'var(--text)',
                          }}
                        >
                          <option value="">Select Account...</option>
                          {accounts.map((acc: any) => {
                            const email = acc.email || acc
                            const label = acc.label ? ` (${acc.label})` : ''
                            return (
                              <option key={email} value={email}>
                                {email}{label}
                              </option>
                            )
                          })}
                        </select>

                        <input
                          type="text"
                          value={draft.path}
                          onChange={(e) =>
                            setAccountOverrideDrafts((prev) => ({
                              ...prev,
                              [appType]: { ...draft, path: e.target.value },
                            }))
                          }
                          placeholder="Account executable path..."
                          style={{
                            flex: 1,
                            backgroundColor: '#ffffff',
                            fontFamily: 'monospace',
                            fontSize: '11.5px',
                            padding: '6px 10px',
                            borderRadius: '6px',
                            border: '1px solid var(--border)',
                            color: 'var(--text)',
                          }}
                        />

                        <button
                          type="button"
                          onClick={() => handleBrowsePath(appType, true)}
                          className="btn-pill-tonal"
                          style={{ padding: '6px 10px', fontSize: '11px' }}
                          title="Browse for override binary"
                        >
                          <FolderOpen size={12} />
                        </button>

                        <button
                          type="button"
                          onClick={() => handleSaveAccountOverride(appType)}
                          disabled={!draft.email || !draft.path}
                          className="btn-pill-tonal"
                          style={{ padding: '6px 12px', fontSize: '11.5px', fontWeight: 600 }}
                        >
                          Set Override
                        </button>
                      </div>

                      {/* Active Overrides Table/List */}
                      {Object.keys(overrides).length > 0 && (
                        <div
                          style={{
                            backgroundColor: '#ffffff',
                            borderRadius: '6px',
                            border: '1px solid var(--border)',
                            padding: '8px 12px',
                            display: 'flex',
                            flexDirection: 'column',
                            gap: '6px',
                          }}
                        >
                          <div style={{ fontSize: '10.5px', fontWeight: 700, color: 'var(--text-subtle)', textTransform: 'uppercase' }}>
                            Configured Account Overrides:
                          </div>
                          {Object.entries(overrides).map(([accEmail, accPath]) => (
                            <div
                              key={accEmail}
                              style={{
                                display: 'flex',
                                alignItems: 'center',
                                justifyContent: 'space-between',
                                fontSize: '11px',
                                padding: '4px 0',
                                borderBottom: '1px solid var(--border-subtle)',
                              }}
                            >
                              <div style={{ display: 'flex', alignItems: 'center', gap: '8px', overflow: 'hidden' }}>
                                <span style={{ fontWeight: 600, color: 'var(--primary)' }}>{accEmail}</span>
                                <span style={{ color: 'var(--text-subtle)' }}>→</span>
                                <span style={{ fontFamily: 'monospace', color: 'var(--text)', textOverflow: 'ellipsis', overflow: 'hidden' }}>
                                  {accPath}
                                </span>
                              </div>
                              <button
                                type="button"
                                onClick={() => handleRemoveAccountOverride(appType, accEmail)}
                                className="btn-pill-danger"
                                style={{ padding: '3px 8px', fontSize: '10.5px' }}
                                title="Remove override"
                              >
                                <Trash2 size={11} />
                              </button>
                            </div>
                          ))}
                        </div>
                      )}
                    </div>
                  </div>
                )
              })}
            </div>
          </div>

          {/* Active Runtime Paths Card */}
          <div className="google-card">
            <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.8px', textTransform: 'uppercase', marginBottom: '16px' }}>
              Current Active Runtime Paths
            </div>

            <div style={{ display: 'flex', flexDirection: 'column', gap: '12px' }}>
              {envPaths.map((item) => (
                <div key={item.key} style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
                  <span
                    style={{
                      width: '200px',
                      fontSize: '12px',
                      fontWeight: 600,
                      color: 'var(--text-muted)',
                      flexShrink: 0,
                    }}
                  >
                    {item.label}
                  </span>

                  <input
                    type="text"
                    readOnly
                    value={item.val}
                    style={{
                      flex: 1,
                      backgroundColor: 'var(--canvas)',
                      fontFamily: 'monospace',
                      fontSize: '11.5px',
                      color: 'var(--text)',
                      padding: '7px 10px',
                    }}
                  />

                  <button
                    onClick={() => copyPath(item.key, item.val)}
                    className="btn-pill-tonal"
                    style={{ padding: '6px 12px', fontSize: '11px', flexShrink: 0 }}
                  >
                    {copiedKey === item.key ? <Check size={14} /> : <Copy size={14} />}
                    {copiedKey === item.key ? 'Copied' : 'Copy'}
                  </button>
                </div>
              ))}
            </div>
          </div>
        </>
      )}

      {/* Tab 2: Error & Privacy */}
      {currentTab === 2 && (
        <>
          {/* Privacy & Telemetry Settings Card */}
          <div className="google-card">
            <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.8px', textTransform: 'uppercase', marginBottom: '4px' }}>
              Privacy & Error Reporting
            </div>
            <div style={{ fontSize: '13px', color: 'var(--text)', marginBottom: '16px' }}>
              Configure anonymous error reporting and telemetry data collection preferences.
            </div>

            {privacyFeedback && (
              <div
                style={{
                  backgroundColor: privacyFeedback.isError ? 'var(--red-bg)' : 'var(--green-bg)',
                  color: privacyFeedback.isError ? 'var(--red)' : 'var(--green)',
                  padding: '10px 14px',
                  borderRadius: '8px',
                  fontSize: '12px',
                  marginBottom: '16px',
                }}
              >
                {privacyFeedback.text}
              </div>
            )}

            <div style={{ display: 'flex', flexDirection: 'column', gap: '14px' }}>
              {/* Error reporting toggle */}
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '14px 18px', backgroundColor: 'var(--canvas)', borderRadius: '10px' }}>
                <div>
                  <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)' }}>
                    Auto-Send Anonymous Non-Sensitive Error Reports
                  </div>
                  <div style={{ fontSize: '12px', color: 'var(--text-muted)', marginTop: '2px' }}>
                    Helps us fix crashes and unexpected runtime exceptions. All emails, tokens, and credentials are automatically scrubbed.
                  </div>
                </div>

                <ToggleSwitch
                  size="md"
                  checked={anonymousErrorReports}
                  onChange={(val) => handleSavePrivacyToggle(val, anonymousTelemetry)}
                />
              </div>

              {/* Telemetry toggle */}
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '14px 18px', backgroundColor: 'var(--canvas)', borderRadius: '10px' }}>
                <div>
                  <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)' }}>
                    Auto-Send Anonymous Telemetry & Usage Analytics
                  </div>
                  <div style={{ fontSize: '12px', color: 'var(--text-muted)', marginTop: '2px' }}>
                    Shares anonymous aggregated metrics (such as quota rotation counts and model usage frequencies) to optimize performance.
                  </div>
                </div>

                <ToggleSwitch
                  size="md"
                  checked={anonymousTelemetry}
                  onChange={(val) => handleSavePrivacyToggle(anonymousErrorReports, val)}
                />
              </div>
            </div>

            {/* Privacy Shield Assurance Banner */}
            <div
              style={{
                marginTop: '16px',
                display: 'flex',
                alignItems: 'center',
                gap: '10px',
                padding: '12px 16px',
                backgroundColor: 'var(--green-bg)',
                border: '1px solid #ceead6',
                borderRadius: '8px',
                fontSize: '12px',
                color: 'var(--green)',
              }}
            >
              <ShieldCheck size={18} style={{ flexShrink: 0 }} />
              <div>
                <strong>Zero-Sensitive-Data Guarantee:</strong> Passwords, session tokens, refresh tokens, emails, and prompt contents are strictly redacted and never transmitted.
              </div>
            </div>
          </div>

          {/* Issue Diagnosis & GitHub Reporter Gadget */}
          <div className="google-card">
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '14px' }}>
              <div>
                <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.8px', textTransform: 'uppercase' }}>
                  Agent Diagnostics & GitHub Issue Reporter
                </div>
                <div style={{ fontSize: '13px', color: 'var(--text)', marginTop: '4px' }}>
                  Describe an issue and let your Antigravity agent diagnose runtime state and prepare an issue on our public GitHub repository.
                </div>
              </div>

              <div className="badge-chip badge-tonal" style={{ fontSize: '11.5px', padding: '5px 12px' }}>
                <Bug size={13} />
                <span>Public GitHub Bug Reporter</span>
              </div>
            </div>

            {/* Pipeline Hierarchy Badges */}
            <div
              style={{
                display: 'flex',
                flexDirection: 'column',
                gap: '8px',
                padding: '14px 16px',
                backgroundColor: 'var(--canvas)',
                borderRadius: '10px',
                border: '1px solid var(--border)',
                marginBottom: '16px',
              }}
            >
              <div style={{ display: 'flex', alignItems: 'center', gap: '8px', fontSize: '11.5px', color: 'var(--text-muted)' }}>
                <strong style={{ minWidth: '130px', color: 'var(--text)' }}>Agent Priority:</strong>
                <span className="badge-chip badge-tonal">1. Antigravity 2.0</span>
                <span>→</span>
                <span className="badge-chip badge-tonal">2. agy CLI</span>
                <span>→</span>
                <span className="badge-chip badge-tonal">3. VS Code Extension</span>
              </div>

              <div style={{ display: 'flex', alignItems: 'center', gap: '8px', fontSize: '11.5px', color: 'var(--text-muted)' }}>
                <strong style={{ minWidth: '130px', color: 'var(--text)' }}>Account Resolution:</strong>
                <span className="badge-chip badge-tonal">Active Account</span>
                <span>→</span>
                <span className="badge-chip badge-tonal">Enabled Keyring Account</span>
                <span>→</span>
                <span className="badge-chip badge-tonal">Enabled Custom Model</span>
                <span>→</span>
                <span className="badge-chip badge-tonal">Built-in Engine</span>
              </div>
            </div>

            {/* Description Input */}
            <div style={{ display: 'flex', flexDirection: 'column', gap: '8px', marginBottom: '14px' }}>
              <label style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text)' }}>
                Short Description of Issue:
              </label>
              <textarea
                value={issueDescription}
                onChange={(e) => setIssueDescription(e.target.value)}
                placeholder="e.g., Encountered daemon socket timeout when switching quota accounts, or custom model response delayed..."
                rows={3}
                style={{
                  width: '100%',
                  padding: '10px 12px',
                  borderRadius: '8px',
                  border: '1px solid var(--border)',
                  fontSize: '12.5px',
                  fontFamily: 'inherit',
                  resize: 'vertical',
                  boxSizing: 'border-box',
                }}
              />
            </div>

            {/* Diagnostic Options & Action */}
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '16px' }}>
              <div style={{ display: 'flex', gap: '18px' }}>
                <label style={{ display: 'flex', alignItems: 'center', gap: '6px', fontSize: '12px', color: 'var(--text-muted)', cursor: 'pointer' }}>
                  <input
                    type="checkbox"
                    checked={includeSystemInfo}
                    onChange={(e) => setIncludeSystemInfo(e.target.checked)}
                  />
                  <span>Include sanitized system info</span>
                </label>
                <label style={{ display: 'flex', alignItems: 'center', gap: '6px', fontSize: '12px', color: 'var(--text-muted)', cursor: 'pointer' }}>
                  <input
                    type="checkbox"
                    checked={includeLogs}
                    onChange={(e) => setIncludeLogs(e.target.checked)}
                  />
                  <span>Include sanitized runtime logs</span>
                </label>
              </div>

              <button
                onClick={handleRunDiagnostics}
                disabled={isDiagnosing || !issueDescription.trim()}
                className="btn-pill-primary"
                style={{ padding: '7px 20px', fontSize: '12px', display: 'flex', alignItems: 'center', gap: '6px' }}
              >
                {isDiagnosing ? <RefreshCw size={13} className="animate-spin" /> : <Send size={13} />}
                {isDiagnosing ? 'Running Diagnostics...' : 'Diagnose & Prepare Issue'}
              </button>
            </div>

            {diagError && (
              <div
                style={{
                  backgroundColor: 'var(--red-bg)',
                  color: 'var(--red)',
                  padding: '10px 14px',
                  borderRadius: '8px',
                  fontSize: '12px',
                  marginBottom: '16px',
                }}
              >
                {diagError}
              </div>
            )}

            {/* Diagnosed Results Box */}
            {diagResult && (
              <div
                style={{
                  marginTop: '16px',
                  border: '1px solid var(--border)',
                  borderRadius: '12px',
                  backgroundColor: 'var(--canvas)',
                  padding: '16px',
                  display: 'flex',
                  flexDirection: 'column',
                  gap: '12px',
                }}
              >
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                    <span className="badge-chip badge-green">
                      <ShieldCheck size={13} /> Sanitization Passed ({diagResult.redacted_token_count} scrubbed)
                    </span>
                    <span className="badge-chip badge-tonal">
                      Agent: {diagResult.agent_selected}
                    </span>
                    <span className="badge-chip badge-tonal">
                      Runner: {diagResult.account_or_model_used}
                    </span>
                  </div>

                  <div style={{ display: 'flex', gap: '8px' }}>
                    <button
                      onClick={copyReportText}
                      className="btn-pill-tonal"
                      style={{ padding: '5px 12px', fontSize: '11px', display: 'flex', alignItems: 'center', gap: '4px' }}
                    >
                      {copiedReport ? <Check size={12} /> : <Copy size={12} />}
                      {copiedReport ? 'Copied' : 'Copy Issue Markdown'}
                    </button>
                    <button
                      onClick={handleOpenGitHubIssue}
                      className="btn-pill-primary"
                      style={{ padding: '5px 14px', fontSize: '11px', display: 'flex', alignItems: 'center', gap: '5px' }}
                    >
                      <ExternalLink size={12} /> Open Issue on GitHub
                    </button>
                  </div>
                </div>

                <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--text)' }}>
                  {diagResult.issue_title}
                </div>

                <pre
                  style={{
                    backgroundColor: '#ffffff',
                    border: '1px solid var(--border)',
                    borderRadius: '8px',
                    padding: '12px',
                    fontSize: '11.5px',
                    fontFamily: 'monospace',
                    color: 'var(--text)',
                    maxHeight: '260px',
                    overflowY: 'auto',
                    margin: 0,
                    whiteSpace: 'pre-wrap',
                    wordBreak: 'break-word',
                  }}
                >
                  {diagResult.sanitized_report}
                </pre>
              </div>
            )}
          </div>
        </>
      )}

      {/* Tab 3: About */}
      {currentTab === 3 && (
        <>
          {/* App Info Card */}
          <div className="google-card" style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '14px' }}>
              <div
                style={{
                  width: '44px',
                  height: '44px',
                  borderRadius: '12px',
                  backgroundColor: 'rgba(26, 115, 232, 0.1)',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  color: 'var(--primary)',
                  fontWeight: 800,
                  fontSize: '18px',
                }}
              >
                SK
              </div>
              <div>
                <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                  <h2 style={{ margin: 0, fontSize: '17px', fontWeight: 700, color: 'var(--text)' }}>
                    Antigravity Swiss Knife
                  </h2>
                  <span className="badge-chip badge-green">v2.0.0</span>
                </div>
                <div style={{ fontSize: '12.5px', color: 'var(--text-muted)', marginTop: '3px' }}>
                  Native standalone desktop companion and quota manager for Google Antigravity 2.0.
                </div>
              </div>
            </div>

            <a
              href="https://github.com/ChillingWombat/AntigravitySwissKnife"
              target="_blank"
              rel="noopener noreferrer"
              className="btn-pill-tonal"
              style={{ textDecoration: 'none', display: 'flex', alignItems: 'center', gap: '6px', fontSize: '12px' }}
            >
              <ExternalLink size={13} /> View on GitHub
            </a>
          </div>

          {/* Support & Community Appreciation Card */}
          <div className="google-card" style={{ backgroundColor: 'var(--surface)' }}>
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '12px' }}>
              <div>
                <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.8px', textTransform: 'uppercase' }}>
                  Support & Appreciation
                </div>
                <div style={{ fontSize: '14px', fontWeight: 600, color: 'var(--text)', marginTop: '2px' }}>
                  Enjoying Antigravity Swiss Knife? Support the Project!
                </div>
              </div>
            </div>

            <p style={{ margin: '0 0 16px 0', fontSize: '12.5px', color: 'var(--text-muted)', lineHeight: 1.5 }}>
              If Antigravity Swiss Knife streamlines your workflow, saves quota, and unlocks deeper agent orchestrations,
              consider starring our GitHub repository or buying me a can of SOLO!
            </p>

            <div style={{ display: 'flex', alignItems: 'center', gap: '14px', flexWrap: 'wrap' }}>
              {/* Native Ko-fi "Buy me a SOLO (A$1)" Widget */}
              <button
                type="button"
                onClick={() => handleOpenExternal('https://ko-fi.com/L3S628AE5Q')}
                title="Support on Ko-fi"
                style={{
                  display: 'inline-flex',
                  alignItems: 'center',
                  gap: '8px',
                  backgroundColor: '#ffde21',
                  color: '#000000',
                  border: 'none',
                  borderRadius: '7px',
                  padding: '4px 18px',
                  height: '42px',
                  fontWeight: 700,
                  fontSize: '14px',
                  cursor: 'pointer',
                  boxShadow: '1px 1px 0px rgba(0, 0, 0, 0.2)',
                  textDecoration: 'none',
                  transition: 'opacity 0.15s ease, transform 0.15s ease',
                }}
                onMouseEnter={(e) => (e.currentTarget.style.opacity = '0.9')}
                onMouseLeave={(e) => (e.currentTarget.style.opacity = '1')}
              >
                <img
                  src={soloCanImg}
                  alt="SOLO Can"
                  style={{
                    height: '24px',
                    width: 'auto',
                    verticalAlign: 'middle',
                    animation: 'kofi-wiggle 3s infinite',
                  }}
                />
                <span style={{ letterSpacing: '-0.15px' }}>Buy me a SOLO (A$1)</span>
              </button>

              {/* Star on GitHub */}
              <button
                type="button"
                onClick={() => handleOpenExternal('https://github.com/ChillingWombat/AntigravitySwissKnife')}
                className="btn-pill-tonal"
                style={{
                  height: '42px',
                  padding: '0 20px',
                  display: 'inline-flex',
                  alignItems: 'center',
                  gap: '8px',
                  fontSize: '13px',
                  fontWeight: 600,
                  borderRadius: '8px',
                  cursor: 'pointer',
                }}
              >
                <Star size={16} color="#f59e0b" fill="#f59e0b" />
                <span>Star on GitHub</span>
              </button>
            </div>
          </div>

          {/* Clean State Restore / Factory Reset Gadget */}
          <div className="google-card" style={{ border: '1px solid #fce8e6' }}>
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '10px' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                <RotateCcw size={18} color="var(--red)" />
                <div style={{ fontSize: '13.5px', fontWeight: 700, color: 'var(--text)' }}>
                  Restore Antigravity Apps to Clean Unmodified State
                </div>
              </div>

              <span className="badge-chip badge-red">Factory Restore</span>
            </div>

            <p style={{ margin: '0 0 14px 0', fontSize: '12px', color: 'var(--text-muted)', lineHeight: 1.5 }}>
              Restore all Antigravity apps to an unmodified status by turning off all features and restoring backed-up files and code.
              This removes custom executable overrides, disables injected customizations, and resets state safely.
            </p>

            {resetFeedback && (
              <div
                style={{
                  backgroundColor: resetFeedback.isError ? 'var(--red-bg)' : 'var(--green-bg)',
                  color: resetFeedback.isError ? 'var(--red)' : 'var(--green)',
                  padding: '10px 14px',
                  borderRadius: '8px',
                  fontSize: '12px',
                  marginBottom: '14px',
                  fontWeight: 500,
                }}
              >
                {resetFeedback.text}
              </div>
            )}

            <button
              type="button"
              onClick={() => setShowResetConfirm(true)}
              disabled={isResetting}
              className="btn-pill-danger"
              style={{ fontSize: '12.5px', padding: '8px 20px', display: 'inline-flex', alignItems: 'center', gap: '6px' }}
            >
              <RotateCcw size={14} />
              {isResetting ? 'Restoring...' : 'Restore All to Factory / Unmodified State'}
            </button>
          </div>

          {/* Reset Confirmation Modal */}
          {showResetConfirm && (
            <div
              style={{
                position: 'fixed',
                inset: 0,
                backgroundColor: 'rgba(0, 0, 0, 0.45)',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                zIndex: 9999,
                backdropFilter: 'blur(2px)',
              }}
            >
              <div
                style={{
                  backgroundColor: 'var(--surface)',
                  borderRadius: '16px',
                  padding: '24px',
                  maxWidth: '480px',
                  width: '90%',
                  boxShadow: '0 20px 25px -5px rgba(0, 0, 0, 0.2)',
                  border: '1px solid var(--border)',
                }}
              >
                <div style={{ display: 'flex', alignItems: 'center', gap: '10px', marginBottom: '14px' }}>
                  <AlertTriangle size={24} color="var(--red)" />
                  <h3 style={{ margin: 0, fontSize: '16px', fontWeight: 700, color: 'var(--text)' }}>
                    Confirm Clean State Restore
                  </h3>
                </div>

                <p style={{ margin: '0 0 18px 0', fontSize: '13px', color: 'var(--text-muted)', lineHeight: 1.5 }}>
                  Are you sure you want to restore all Antigravity applications to their unmodified status?
                  All active features will be turned off, custom executable paths and account overrides will be reset,
                  and original backed-up code/files will be restored.
                </p>

                <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '10px' }}>
                  <button
                    type="button"
                    onClick={() => setShowResetConfirm(false)}
                    disabled={isResetting}
                    className="btn-pill-tonal"
                    style={{ padding: '8px 16px', fontSize: '12.5px' }}
                  >
                    Cancel
                  </button>
                  <button
                    type="button"
                    onClick={handleFactoryReset}
                    disabled={isResetting}
                    className="btn-pill-danger"
                    style={{ padding: '8px 20px', fontSize: '12.5px' }}
                  >
                    {isResetting ? 'Restoring...' : 'Yes, Restore to Clean State'}
                  </button>
                </div>
              </div>
            </div>
          )}

          {/* Antigravity Installations & Updates Card */}
          <div className="google-card">
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '16px' }}>
              <div>
                <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.8px', textTransform: 'uppercase' }}>
                  Antigravity Installations & Updates
                </div>
                <div style={{ fontSize: '12px', color: 'var(--text-muted)', marginTop: '2px' }}>
                  Cross-platform host detection ({installations?.platform || 'linux'} / {installations?.arch || 'amd64'})
                </div>
              </div>

              <button
                onClick={handleCheckUpdates}
                disabled={isCheckingUpdates}
                className="btn-pill-primary"
                style={{ padding: '6px 14px', fontSize: '12px', display: 'flex', alignItems: 'center', gap: '6px' }}
              >
                <RefreshCw size={13} />
                {isCheckingUpdates ? 'Checking...' : 'Check for Updates'}
              </button>
            </div>

            {updateFeedback && (
              <div style={{ fontSize: '12px', color: 'var(--primary)', marginBottom: '14px', fontWeight: 500 }}>
                {updateFeedback}
              </div>
            )}

            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: '16px' }}>
              {/* Desktop App */}
              <div
                style={{
                  border: '1px solid var(--border)',
                  borderRadius: '12px',
                  padding: '16px',
                  backgroundColor: 'var(--canvas)',
                  display: 'flex',
                  flexDirection: 'column',
                  justifyContent: 'space-between',
                }}
              >
                <div>
                  <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '10px' }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                      <Monitor size={18} color="var(--primary)" />
                      <span style={{ fontSize: '14px', fontWeight: 700, color: 'var(--text)' }}>
                        Desktop App (2.0)
                      </span>
                    </div>
                    {installations?.desktop_app?.installed ? (
                      <span className={`badge-chip ${installations.desktop_app.up_to_date ? 'badge-green' : 'badge-yellow'}`}>
                        {installations.desktop_app.up_to_date
                          ? `v${installations.desktop_app.version} (Up to Date)`
                          : `v${installations.desktop_app.version} → v${installations.desktop_app.latest_version} (Update Available)`}
                      </span>
                    ) : (
                      <span className="badge-chip badge-neutral">Not Detected</span>
                    )}
                  </div>

                  <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginBottom: '6px' }}>
                    <strong>Binary / Package Path:</strong>
                  </div>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '6px', marginBottom: '12px' }}>
                    <input
                      type="text"
                      readOnly
                      value={installations?.desktop_app?.path || 'Not installed'}
                      style={{
                        flex: 1,
                        backgroundColor: '#ffffff',
                        fontFamily: 'monospace',
                        fontSize: '11px',
                        padding: '6px 10px',
                        borderRadius: '6px',
                        border: '1px solid var(--border)',
                        color: 'var(--text)',
                      }}
                    />
                    {installations?.desktop_app?.path && (
                      <button
                        onClick={() => copyPath('desktop_path', installations.desktop_app.path)}
                        className="btn-pill-tonal"
                        style={{ padding: '5px 10px', fontSize: '11px' }}
                        title="Copy path"
                      >
                        {copiedKey === 'desktop_path' ? <Check size={12} /> : <Copy size={12} />}
                      </button>
                    )}
                  </div>
                </div>

                <div style={{ fontSize: '11px', color: 'var(--text-subtle)', borderTop: '1px solid var(--border)', paddingTop: '10px' }}>
                  <strong>Process State:</strong> {installations?.desktop_app?.process_state || (status?.antigravity_running ? `Running (PID: ${status.antigravity_pid})` : 'Not running')}
                </div>
              </div>

              {/* agy CLI */}
              <div
                style={{
                  border: '1px solid var(--border)',
                  borderRadius: '12px',
                  padding: '16px',
                  backgroundColor: 'var(--canvas)',
                  display: 'flex',
                  flexDirection: 'column',
                  justifyContent: 'space-between',
                }}
              >
                <div>
                  <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '10px' }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                      <Terminal size={18} color="var(--primary)" />
                      <span style={{ fontSize: '14px', fontWeight: 700, color: 'var(--text)' }}>
                        agy CLI
                      </span>
                    </div>
                    {storageInfo?.app_zones?.agy?.installed ? (
                      <span className="badge-chip badge-green">
                        Installed {storageInfo.app_zones.agy.version ? `v${storageInfo.app_zones.agy.version}` : ''}
                      </span>
                    ) : (
                      <span className="badge-chip badge-neutral">Not Detected</span>
                    )}
                  </div>

                  <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginBottom: '6px' }}>
                    <strong>CLI Binary Path:</strong>
                  </div>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '6px', marginBottom: '12px' }}>
                    <input
                      type="text"
                      readOnly
                      value={storageInfo?.app_zones?.agy?.active_path || storageInfo?.app_zones?.agy?.detected_path || 'Not installed'}
                      style={{
                        flex: 1,
                        backgroundColor: '#ffffff',
                        fontFamily: 'monospace',
                        fontSize: '11px',
                        padding: '6px 10px',
                        borderRadius: '6px',
                        border: '1px solid var(--border)',
                        color: 'var(--text)',
                      }}
                    />
                    {(storageInfo?.app_zones?.agy?.active_path || storageInfo?.app_zones?.agy?.detected_path) && (
                      <button
                        onClick={() => copyPath('agy_path', storageInfo?.app_zones?.agy?.active_path || storageInfo?.app_zones?.agy?.detected_path || '')}
                        className="btn-pill-tonal"
                        style={{ padding: '5px 10px', fontSize: '11px' }}
                        title="Copy path"
                      >
                        {copiedKey === 'agy_path' ? <Check size={12} /> : <Copy size={12} />}
                      </button>
                    )}
                  </div>
                </div>

                <div style={{ fontSize: '11px', color: 'var(--text-subtle)', borderTop: '1px solid var(--border)', paddingTop: '10px' }}>
                  <strong>Execution Target:</strong> {storageInfo?.app_zones?.agy?.installed ? 'Autonomous terminal runner' : 'Binary not found in PATH'}
                </div>
              </div>

              {/* VS Code Extension */}
              <div
                style={{
                  border: '1px solid var(--border)',
                  borderRadius: '12px',
                  padding: '16px',
                  backgroundColor: 'var(--canvas)',
                  display: 'flex',
                  flexDirection: 'column',
                  justifyContent: 'space-between',
                }}
              >
                <div>
                  <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '10px' }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                      <Code size={18} color="var(--primary)" />
                      <span style={{ fontSize: '14px', fontWeight: 700, color: 'var(--text)' }}>
                        VS Code Extension
                      </span>
                    </div>
                    {installations?.vscode_extension?.installed ? (
                      <span className={`badge-chip ${installations.vscode_extension.up_to_date ? 'badge-green' : 'badge-yellow'}`}>
                        {installations.vscode_extension.up_to_date
                          ? `v${installations.vscode_extension.version} (Up to Date)`
                          : `v${installations.vscode_extension.version} → v${installations.vscode_extension.latest_version} (Update Available)`}
                      </span>
                    ) : (
                      <span className="badge-chip badge-neutral">Not Detected</span>
                    )}
                  </div>

                  <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginBottom: '6px' }}>
                    <strong>Extension Directory:</strong>
                  </div>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '6px', marginBottom: '12px' }}>
                    <input
                      type="text"
                      readOnly
                      value={installations?.vscode_extension?.path || 'Not installed'}
                      style={{
                        flex: 1,
                        backgroundColor: '#ffffff',
                        fontFamily: 'monospace',
                        fontSize: '11px',
                        padding: '6px 10px',
                        borderRadius: '6px',
                        border: '1px solid var(--border)',
                        color: 'var(--text)',
                      }}
                    />
                    {installations?.vscode_extension?.path && (
                      <button
                        onClick={() => copyPath('vscode_path', installations.vscode_extension.path)}
                        className="btn-pill-tonal"
                        style={{ padding: '5px 10px', fontSize: '11px' }}
                        title="Copy path"
                      >
                        {copiedKey === 'vscode_path' ? <Check size={12} /> : <Copy size={12} />}
                      </button>
                    )}
                  </div>
                </div>

                <div style={{ fontSize: '11px', color: 'var(--text-subtle)', borderTop: '1px solid var(--border)', paddingTop: '10px' }}>
                  <strong>Integration Target:</strong> {installations?.vscode_extension?.installed ? 'Webview injection ready' : 'Extension not found'}
                </div>
              </div>
            </div>
          </div>

          {/* System Environment Details */}
          <div className="google-card">
            <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', letterSpacing: '0.8px', textTransform: 'uppercase', marginBottom: '12px' }}>
              System Environment & Runtime Architecture
            </div>

            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(4, 1fr)', gap: '14px' }}>
              <div style={{ padding: '12px', backgroundColor: 'var(--canvas)', borderRadius: '8px', border: '1px solid var(--border)' }}>
                <div style={{ fontSize: '11px', color: 'var(--text-muted)', fontWeight: 600 }}>PLATFORM</div>
                <div style={{ fontSize: '14px', fontWeight: 700, color: 'var(--text)', marginTop: '4px' }}>
                  {installations?.platform || 'linux'}
                </div>
              </div>

              <div style={{ padding: '12px', backgroundColor: 'var(--canvas)', borderRadius: '8px', border: '1px solid var(--border)' }}>
                <div style={{ fontSize: '11px', color: 'var(--text-muted)', fontWeight: 600 }}>ARCHITECTURE</div>
                <div style={{ fontSize: '14px', fontWeight: 700, color: 'var(--text)', marginTop: '4px' }}>
                  {installations?.arch || 'amd64'}
                </div>
              </div>

              <div style={{ padding: '12px', backgroundColor: 'var(--canvas)', borderRadius: '8px', border: '1px solid var(--border)' }}>
                <div style={{ fontSize: '11px', color: 'var(--text-muted)', fontWeight: 600 }}>DAEMON PROTOCOL</div>
                <div style={{ fontSize: '14px', fontWeight: 700, color: 'var(--text)', marginTop: '4px' }}>
                  JSON-RPC 2.0 / IPC
                </div>
              </div>

              <div style={{ padding: '12px', backgroundColor: 'var(--canvas)', borderRadius: '8px', border: '1px solid var(--border)' }}>
                <div style={{ fontSize: '11px', color: 'var(--text-muted)', fontWeight: 600 }}>LICENSE</div>
                <div style={{ fontSize: '14px', fontWeight: 700, color: 'var(--text)', marginTop: '4px' }}>
                  MIT License
                </div>
              </div>
            </div>
          </div>
        </>
      )}
    </div>
  )
}
