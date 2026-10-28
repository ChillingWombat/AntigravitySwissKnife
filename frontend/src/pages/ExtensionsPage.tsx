import React, { useEffect, useState } from 'react'
import {
  Blocks,
  Globe,
  Code,
  Terminal,
  Database,
  Bot,
  Wrench,
  BookOpen,
  Layers,
  Cpu,
  Plus,
  Upload,
  Download,
  Trash2,
  Edit3,
  X,
  CheckCircle2,
  AlertTriangle,
  PanelLeft,
  Folder,
  FileText,
} from 'lucide-react'
import { ToggleSwitch } from '../components/ToggleSwitch'
import { GithubIcon } from '../components/GithubIcon'
import { api } from '../api'
import type { EnhancementsConfig, CommunityExtension, CommunityExtensionIcon } from '../types'

const IDE_PRESETS = ['code', 'cursor', 'windsurf', 'codium', 'zed']

// Transcription / OCR language choices shared by both memo language selectors.
const MEMO_LANGUAGES: Array<{ code: string; label: string }> = [
  { code: 'auto', label: 'Auto detect' },
  { code: 'en', label: 'English' },
  { code: 'zh-Hans', label: '简体中文' },
  { code: 'zh-Hant', label: '繁體中文' },
  { code: 'ja', label: '日本語' },
  { code: 'ko', label: '한국어' },
  { code: 'es', label: 'Español' },
  { code: 'fr', label: 'Français' },
  { code: 'de', label: 'Deutsch' },
  { code: 'pt', label: 'Português' },
  { code: 'ru', label: 'Русский' },
  { code: 'ar', label: 'العربية' },
  { code: 'hi', label: 'हिन्दी' },
  { code: 'it', label: 'Italiano' },
  { code: 'nl', label: 'Nederlands' },
  { code: 'tr', label: 'Türkçe' },
  { code: 'vi', label: 'Tiếng Việt' },
  { code: 'th', label: 'ไทย' },
  { code: 'id', label: 'Bahasa Indonesia' },
]

const COMMUNITY_ICONS: Array<{ id: CommunityExtensionIcon; label: string }> = [
  { id: 'blocks', label: 'Blocks' },
  { id: 'globe', label: 'Globe' },
  { id: 'code', label: 'Code' },
  { id: 'terminal', label: 'Terminal' },
  { id: 'database', label: 'Database' },
  { id: 'bot', label: 'Bot' },
  { id: 'wrench', label: 'Wrench' },
  { id: 'book-open', label: 'Book' },
  { id: 'layers', label: 'Layers' },
  { id: 'cpu', label: 'CPU' },
]

const slugify = (text: string) =>
  text
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9_-]+/g, '-')
    .replace(/^-+|-+$/g, '')

const renderCommunityIcon = (icon?: string, size = 16, color = 'var(--text-muted)') => {
  switch (icon) {
    case 'globe':
      return <Globe size={size} color={color} />
    case 'code':
      return <Code size={size} color={color} />
    case 'terminal':
      return <Terminal size={size} color={color} />
    case 'database':
      return <Database size={size} color={color} />
    case 'bot':
      return <Bot size={size} color={color} />
    case 'wrench':
      return <Wrench size={size} color={color} />
    case 'book-open':
      return <BookOpen size={size} color={color} />
    case 'layers':
      return <Layers size={size} color={color} />
    case 'cpu':
      return <Cpu size={size} color={color} />
    case 'blocks':
    default:
      return <Blocks size={size} color={color} />
  }
}

type ExtensionVisibility = { aux_panel: boolean; main_page: boolean }
const DEFAULT_EXT_VIS: ExtensionVisibility = { aux_panel: true, main_page: false }

export interface ExtensionsPageProps {
  activeTab?: number
  onTabChange?: (tab: number) => void
  scope?: string
  onScopeChange?: (scope: string) => void
  fallbackProject?: string
}

export const ExtensionsPage: React.FC<ExtensionsPageProps> = ({ activeTab = 0 }) => {
  // --- 1. Extension Visibility Switches (daemon-persisted EnhancementsConfig) ---
  // aux_panel  = available in the IDE's right auxiliary panel
  // main_page  = openable on the main stage via a left-sidebar tab button
  const [enhConfig, setEnhConfig] = useState<EnhancementsConfig | null>(null)
  const [extVisLocal, setExtVisLocal] = useState<Record<string, ExtensionVisibility>>({})
  const [enhError, setEnhError] = useState<string | null>(null)

  // --- 1b. Community Extensions State ---
  const [communityExtensions, setCommunityExtensions] = useState<CommunityExtension[]>([])
  const [isEditorOpen, setIsEditorOpen] = useState<boolean>(false)
  const [editingId, setEditingId] = useState<string | null>(null)
  const [isImportModalOpen, setIsImportModalOpen] = useState<boolean>(false)
  const [importJsonText, setImportJsonText] = useState<string>('')
  const [importError, setImportError] = useState<string | null>(null)

  // Form state for Create / Edit Community Extension
  const [formName, setFormName] = useState<string>('')
  const [formId, setFormId] = useState<string>('')
  const [formIdEdited, setFormIdEdited] = useState<boolean>(false)
  const [formVersion, setFormVersion] = useState<string>('1.0.0')
  const [formAuthor, setFormAuthor] = useState<string>('Community')
  const [formDescription, setFormDescription] = useState<string>('')
  const [formIcon, setFormIcon] = useState<CommunityExtensionIcon | string>('blocks')
  const [formSourceType, setFormSourceType] = useState<'url' | 'html'>('url')
  const [formUrl, setFormUrl] = useState<string>('')
  const [formHtml, setFormHtml] = useState<string>('')
  const [formCss, setFormCss] = useState<string>('')
  const [formJs, setFormJs] = useState<string>('')
  const [formAuxPanel, setFormAuxPanel] = useState<boolean>(true)
  const [formMainPage, setFormMainPage] = useState<boolean>(false)

  // --- 2. Extension Specific Settings State ---
  // GitHub Workspace
  const [auxAgentScope, setAuxAgentScope] = useState<'all' | 'focused'>(() => {
    return (localStorage.getItem('antigravity_github_aux_agent_scope') as 'all' | 'focused') || 'all'
  })
  const [auxPinFocused, setAuxPinFocused] = useState<boolean>(() => {
    return localStorage.getItem('antigravity_github_aux_pin_focused') !== 'false'
  })

  // Preview Browser
  const [previewUrl, setPreviewUrl] = useState<string>(() => {
    const stored = localStorage.getItem('antigravity_browser_default_url')
    if (!stored || stored === 'http://localhost:5173') return 'http://localhost:8765'
    return stored
  })

  // File Explorer
  const [preferredIDE, setPreferredIDE] = useState<string>(() => {
    return (
      localStorage.getItem('antigravity_swiss_preferred_ide') ||
      localStorage.getItem('antigravity_preferred_ide') ||
      'code'
    )
  })
  const [ideCustomMode, setIdeCustomMode] = useState<boolean>(false)
  const [customIDE, setCustomIDE] = useState<string>(() =>
    IDE_PRESETS.includes(preferredIDE.toLowerCase()) ? '' : preferredIDE
  )
  const ideIsCustom = ideCustomMode || !IDE_PRESETS.includes(preferredIDE.toLowerCase())

  // Quick Memos (daemon-persisted; localStorage seeds the first paint)
  const [memoStorageLocation, setMemoStorageLocation] = useState<'global' | 'project'>(() => {
    return (localStorage.getItem('antigravity_memo_storage_location') as 'global' | 'project') || 'global'
  })
  const [memoViewScope, setMemoViewScope] = useState<'all' | 'current'>(() => {
    return (localStorage.getItem('antigravity_memo_view_scope') as 'all' | 'current') || 'all'
  })
  const [memoSearchScope, setMemoSearchScope] = useState<'text' | 'all'>(() => {
    return (localStorage.getItem('antigravity_memo_search_scope') as 'text' | 'all') || 'text'
  })
  const [memoAutoExtract, setMemoAutoExtract] = useState<boolean>(() => {
    return localStorage.getItem('antigravity_memo_auto_extract') === 'true'
  })
  const [memoVoiceLanguage, setMemoVoiceLanguage] = useState<string>(() => {
    return localStorage.getItem('antigravity_memo_voice_language') || 'auto'
  })
  const [memoOcrLanguage, setMemoOcrLanguage] = useState<string>(() => {
    return localStorage.getItem('antigravity_memo_ocr_language') || 'auto'
  })

  // Mobile Simulator
  const [selectedDevice, setSelectedDevice] = useState<'iphone16' | 'pixel9' | 'ipad'>(() => {
    return (localStorage.getItem('antigravity_mobile_default_device') as any) || 'iphone16'
  })
  const [showBezel, setShowBezel] = useState<boolean>(() => {
    return localStorage.getItem('antigravity_mobile_show_bezel') !== 'false'
  })

  // Feedback Notification
  const [feedback, setFeedback] = useState<string | null>(null)

  const showFeedback = (msg: string) => {
    setFeedback(msg)
    setTimeout(() => setFeedback(null), 3000)
  }

  // Load daemon-persisted settings (extension visibility, IDE + Quick Memos);
  // keep localStorage fallbacks if the daemon is unreachable.
  useEffect(() => {
    api
      .getEnhancements()
      .then((cfg) => {
        setEnhConfig(cfg)
        if (Array.isArray(cfg?.community_extensions) && cfg.community_extensions.length > 0) {
          setCommunityExtensions((prev) => (prev.length === 0 ? cfg.community_extensions! : prev))
        }
        setEnhError(null)
      })
      .catch(() =>
        setEnhError('Enhancements daemon unreachable — visibility switches show defaults and will not persist')
      )

    api
      .getCommunityExtensions()
      .then((res) => {
        if (Array.isArray(res?.extensions)) {
          setCommunityExtensions(res.extensions)
        }
      })
      .catch(() => {})

    api
      .getPreferredIDE()
      .then((res) => {
        if (res?.success && res.preferred_ide) {
          setPreferredIDE(res.preferred_ide)
          if (!IDE_PRESETS.includes(res.preferred_ide.toLowerCase())) {
            setCustomIDE(res.preferred_ide)
          }
          localStorage.setItem('antigravity_swiss_preferred_ide', res.preferred_ide)
          localStorage.setItem('antigravity_preferred_ide', res.preferred_ide)
        }
      })
      .catch(() => {})

    api
      .getMemoConfig()
      .then((res) => {
        const cfg = res?.config
        if (!cfg) return
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
        // Fields added by the extraction-mode / language PR; undefined means "keep the default".
        if (typeof cfg.auto_extract === 'boolean') {
          setMemoAutoExtract(cfg.auto_extract)
          localStorage.setItem('antigravity_memo_auto_extract', cfg.auto_extract ? 'true' : 'false')
        }
        if (typeof cfg.voice_language === 'string' && cfg.voice_language) {
          setMemoVoiceLanguage(cfg.voice_language)
          localStorage.setItem('antigravity_memo_voice_language', cfg.voice_language)
        }
        if (typeof cfg.ocr_language === 'string' && cfg.ocr_language) {
          setMemoOcrLanguage(cfg.ocr_language)
          localStorage.setItem('antigravity_memo_ocr_language', cfg.ocr_language)
        }
      })
      .catch(() => {})
  }, [])

  const handleSaveIDE = async (next: string) => {
    const ide = next.trim()
    if (!ide) return
    setPreferredIDE(ide)
    localStorage.setItem('antigravity_swiss_preferred_ide', ide)
    // Legacy key still read first by the injected extension tab — keep both in sync.
    localStorage.setItem('antigravity_preferred_ide', ide)
    try {
      await api.setPreferredIDE(ide)
      showFeedback(`Preferred IDE set to ${getIDEName(ide)}`)
    } catch {
      showFeedback('Saved preferred IDE locally (daemon unreachable)')
    }
  }

  const handleUpdateMemoConfig = (
    patch: Partial<{
      storage_location: 'global' | 'project'
      view_scope: 'all' | 'current'
      search_scope: 'text' | 'all'
      auto_extract: boolean
      voice_language: string
      ocr_language: string
    }>
  ) => {
    const next = {
      storage_location: patch.storage_location ?? memoStorageLocation,
      view_scope: patch.view_scope ?? memoViewScope,
      search_scope: patch.search_scope ?? memoSearchScope,
      auto_extract: patch.auto_extract ?? memoAutoExtract,
      voice_language: patch.voice_language ?? memoVoiceLanguage,
      ocr_language: patch.ocr_language ?? memoOcrLanguage,
    }
    setMemoStorageLocation(next.storage_location)
    setMemoViewScope(next.view_scope)
    setMemoSearchScope(next.search_scope)
    setMemoAutoExtract(next.auto_extract)
    setMemoVoiceLanguage(next.voice_language)
    setMemoOcrLanguage(next.ocr_language)
    localStorage.setItem('antigravity_memo_storage_location', next.storage_location)
    localStorage.setItem('antigravity_memo_view_scope', next.view_scope)
    localStorage.setItem('antigravity_memo_search_scope', next.search_scope)
    localStorage.setItem('antigravity_memo_auto_extract', next.auto_extract ? 'true' : 'false')
    localStorage.setItem('antigravity_memo_voice_language', next.voice_language)
    localStorage.setItem('antigravity_memo_ocr_language', next.ocr_language)
    api
      .updateMemoConfig(next)
      .then((res) => showFeedback(res?.success ? 'Quick Memos settings saved' : 'Quick Memos settings saved locally'))
      .catch(() => showFeedback('Quick Memos settings saved locally (daemon unreachable)'))
  }

  const segBtnStyle = (active: boolean): React.CSSProperties => ({
    border: 'none',
    padding: '4px 8px',
    borderRadius: '6px',
    fontSize: '12px',
    fontWeight: active ? 600 : 500,
    backgroundColor: active ? '#ffffff' : 'transparent',
    color: active ? 'var(--primary)' : 'var(--text-muted)',
    boxShadow: active ? '0 1px 2px rgba(0,0,0,0.06)' : 'none',
    cursor: 'pointer',
    whiteSpace: 'nowrap',
  })

  // Per-extension visibility: the EnhancementsConfig.extensions map is the
  // source of truth (dashboard localStorage has no effect on the IDE).
  const visFor = (id: string): ExtensionVisibility => {
    const local = extVisLocal[id]
    if (local) return local
    const v = enhConfig?.extensions?.[id]
    if (v) return { aux_panel: v.aux_panel !== false, main_page: v.main_page === true }
    return DEFAULT_EXT_VIS
  }

  const handleExtVisChange = async (id: string, field: 'aux_panel' | 'main_page', value: boolean) => {
    const optimistic: ExtensionVisibility = { ...visFor(id), [field]: value }
    setExtVisLocal((prev) => {
      const cfgVis = enhConfig?.extensions?.[id]
      const baseVis =
        prev[id] ||
        (cfgVis
          ? { aux_panel: cfgVis.aux_panel !== false, main_page: cfgVis.main_page === true }
          : DEFAULT_EXT_VIS)
      return { ...prev, [id]: { ...baseVis, [field]: value } }
    })
    try {
      let base = enhConfig
      if (!base) {
        base = await api.getEnhancements()
        setEnhConfig(base)
      }
      const cur = base.extensions?.[id]
      const next: ExtensionVisibility = {
        aux_panel: field === 'aux_panel' ? value : cur ? cur.aux_panel !== false : optimistic.aux_panel,
        main_page: field === 'main_page' ? value : cur ? cur.main_page === true : optimistic.main_page,
      }
      const updated: EnhancementsConfig = {
        ...base,
        extensions: { ...(base.extensions || {}), [id]: next },
      }
      const saved = await api.updateEnhancements(updated)
      setEnhConfig(saved)
      api.applyEnhancements().catch(() => {})
      setEnhError(null)
      setExtVisLocal((prev) => {
        const copy = { ...prev }
        delete copy[id]
        return copy
      })
      showFeedback('Extension visibility updated')
    } catch {
      setEnhError('Daemon unreachable — extension visibility change was not saved')
    }
  }

  const extSwitchLabelStyle: React.CSSProperties = {
    display: 'inline-flex',
    alignItems: 'center',
    gap: '8px',
    fontSize: '12px',
    fontWeight: 600,
    color: 'var(--text-muted)',
    whiteSpace: 'nowrap',
  }

  const handleLeftPanelModeChange = async (mode: 'single' | 'individual') => {
    try {
      let base = enhConfig
      if (!base) {
        base = await api.getEnhancements()
      }
      const updated: EnhancementsConfig = {
        ...base,
        left_panel_extensions_mode: mode,
      }
      try {
        localStorage.setItem('antigravity_swiss_left_panel_mode', mode)
        window.dispatchEvent(new CustomEvent('swiss-left-nav-config-updated', { detail: { mode } }))
      } catch {}
      const saved = await api.updateEnhancements(updated)
      setEnhConfig(saved)
      api.applyEnhancements().catch(() => {})
      showFeedback(`Left panel mode set to ${mode === 'individual' ? 'Individual Buttons' : 'Single Button'}`)
    } catch {
      setEnhError('Daemon unreachable — left panel mode change was not saved')
    }
  }

  const renderExtSwitches = (id: string, name: string) => {
    const vis = visFor(id)
    return (
      <div style={{ display: 'flex', flexDirection: 'row', alignItems: 'center', gap: '16px', flexShrink: 0 }}>
        <span style={extSwitchLabelStyle} title="Available in the IDE's right auxiliary panel">
          Auxiliary Panel
          <ToggleSwitch
            size="sm"
            checked={vis.aux_panel}
            onChange={(v) => handleExtVisChange(id, 'aux_panel', v)}
            ariaLabel={`${name} available in auxiliary panel`}
          />
        </span>
        <span style={extSwitchLabelStyle} title="Openable on the main stage via a left-sidebar tab button">
          Main Page
          <ToggleSwitch
            size="sm"
            checked={vis.main_page}
            onChange={(v) => handleExtVisChange(id, 'main_page', v)}
            ariaLabel={`${name} available in main page`}
          />
        </span>
      </div>
    )
  }

  const githubVis = visFor('github')
  const browserVis = visFor('browser')
  const filesVis = visFor('files')
  const memosVis = visFor('memos')

  // Community Extensions handlers
  const openCreateForm = () => {
    setEditingId(null)
    setFormName('')
    setFormId('')
    setFormIdEdited(false)
    setFormVersion('1.0.0')
    setFormAuthor('Community')
    setFormDescription('')
    setFormIcon('blocks')
    setFormSourceType('url')
    setFormUrl('')
    setFormHtml('')
    setFormCss('')
    setFormJs('')
    setFormAuxPanel(true)
    setFormMainPage(false)
    setIsEditorOpen(true)
  }

  const openEditForm = (ext: CommunityExtension) => {
    setEditingId(ext.id)
    setFormName(ext.name)
    setFormId(ext.id)
    setFormIdEdited(true)
    setFormVersion(ext.version || '1.0.0')
    setFormAuthor(ext.author || 'Community')
    setFormDescription(ext.description || '')
    setFormIcon((ext.icon as CommunityExtensionIcon) || 'blocks')
    setFormSourceType(ext.source_type || 'url')
    setFormUrl(ext.url || '')
    setFormHtml(ext.html || '')
    setFormCss(ext.css || '')
    setFormJs(ext.js || '')
    setFormAuxPanel(ext.aux_panel !== false)
    setFormMainPage(ext.main_page === true)
    setIsEditorOpen(true)
  }

  const handleSaveExtension = async () => {
    const trimmedName = formName.trim()
    if (!trimmedName) {
      showFeedback('Extension name is required')
      return
    }
    const targetId = formId.trim() || slugify(trimmedName)
    if (!targetId) {
      showFeedback('Extension ID is required')
      return
    }

    const payload: Partial<CommunityExtension> = {
      id: targetId,
      name: trimmedName,
      description: formDescription.trim(),
      author: formAuthor.trim() || 'Community',
      version: formVersion.trim() || '1.0.0',
      icon: formIcon,
      source_type: formSourceType,
      url: formSourceType === 'url' ? formUrl.trim() : undefined,
      html: formSourceType === 'html' ? formHtml : undefined,
      css: formSourceType === 'html' ? formCss : undefined,
      js: formSourceType === 'html' ? formJs : undefined,
      enabled: true,
      aux_panel: formAuxPanel,
      main_page: formMainPage,
    }

    try {
      const res = await api.saveCommunityExtension(payload)
      if (Array.isArray(res?.extensions)) {
        setCommunityExtensions(res.extensions)
      } else if (res?.extension) {
        setCommunityExtensions((prev) => {
          const idx = prev.findIndex((e) => e.id === res.extension.id)
          if (idx >= 0) {
            const next = [...prev]
            next[idx] = res.extension
            return next
          }
          return [...prev, res.extension]
        })
      } else {
        setCommunityExtensions((prev) => {
          const saved = payload as CommunityExtension
          const idx = prev.findIndex((e) => e.id === saved.id)
          if (idx >= 0) {
            const next = [...prev]
            next[idx] = saved
            return next
          }
          return [...prev, saved]
        })
      }
      api.applyEnhancements().catch(() => {})
      setIsEditorOpen(false)
      showFeedback(editingId ? 'Extension updated' : 'Extension created')
    } catch {
      showFeedback('Failed to save extension')
    }
  }

  const handleDeleteCommunityExtension = async (id: string) => {
    try {
      const res = await api.deleteCommunityExtension(id)
      if (Array.isArray(res?.extensions)) {
        setCommunityExtensions(res.extensions)
      } else {
        setCommunityExtensions((prev) => prev.filter((e) => e.id !== id))
      }
      api.applyEnhancements().catch(() => {})
      showFeedback('Community extension deleted')
    } catch {
      showFeedback('Failed to delete community extension')
    }
  }

  const handleCommunityVisChange = async (
    ext: CommunityExtension,
    field: 'aux_panel' | 'main_page',
    value: boolean
  ) => {
    const updatedExt: CommunityExtension = {
      ...ext,
      [field]: value,
    }
    setCommunityExtensions((prev) =>
      prev.map((e) => (e.id === ext.id ? updatedExt : e))
    )
    try {
      const res = await api.saveCommunityExtension(updatedExt)
      if (Array.isArray(res?.extensions)) {
        setCommunityExtensions(res.extensions)
      }
      api.applyEnhancements().catch(() => {})
      showFeedback('Community extension visibility updated')
    } catch {
      showFeedback('Failed to update community extension visibility')
    }
  }

  const handleExportJson = (ext: CommunityExtension) => {
    try {
      if (typeof URL !== 'undefined' && typeof URL.createObjectURL === 'function') {
        const blob = new Blob([JSON.stringify(ext, null, 2)], { type: 'application/json' })
        const url = URL.createObjectURL(blob)
        const a = document.createElement('a')
        a.href = url
        a.download = `${ext.id || 'extension'}.json`
        a.click()
        URL.revokeObjectURL(url)
      }
      showFeedback(`Exported ${ext.name} to JSON`)
    } catch {
      showFeedback(`Exported ${ext.name}`)
    }
  }

  const handleImportSubmit = async () => {
    try {
      const parsed = JSON.parse(importJsonText)
      let items: Partial<CommunityExtension>[] = []
      if (Array.isArray(parsed)) {
        items = parsed
      } else if (parsed && Array.isArray(parsed.extensions)) {
        items = parsed.extensions
      } else if (parsed && typeof parsed === 'object') {
        items = [parsed]
      } else {
        setImportError('Invalid JSON format: expected an object or array')
        return
      }

      if (items.length === 0) {
        setImportError('No extensions found in JSON')
        return
      }

      const res = await api.importCommunityExtensions(items)
      if (Array.isArray(res?.extensions)) {
        setCommunityExtensions(res.extensions)
      } else {
        setCommunityExtensions((prev) => {
          const map = new Map(prev.map((e) => [e.id, e]))
          for (const item of items) {
            if (item.id) {
              map.set(item.id, {
                enabled: true,
                aux_panel: true,
                main_page: false,
                source_type: 'url',
                name: item.id,
                ...item,
              } as CommunityExtension)
            }
          }
          return Array.from(map.values())
        })
      }
      api.applyEnhancements().catch(() => {})
      setIsImportModalOpen(false)
      setImportJsonText('')
      setImportError(null)
      showFeedback(`Imported ${items.length} extension${items.length === 1 ? '' : 's'}`)
    } catch (err: any) {
      setImportError(`JSON parse error: ${err?.message || 'Invalid JSON'}`)
    }
  }

  const getIDEName = (id: string) => {
    const map: Record<string, string> = {
      code: 'VS Code',
      vscode: 'VS Code',
      cursor: 'Cursor',
      windsurf: 'Windsurf',
      codium: 'VSCodium',
      vscodium: 'VSCodium',
      zed: 'Zed',
    }
    return map[id.toLowerCase()] || id || 'VS Code'
  }


  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '20px', maxWidth: '1200px', margin: '0 auto', width: '100%' }}>
      {feedback && (
        <div
          style={{
            padding: '10px 16px',
            borderRadius: '8px',
            backgroundColor: 'var(--green-bg)',
            color: 'var(--green)',
            fontSize: '12px',
            display: 'flex',
            alignItems: 'center',
            gap: '8px',
            fontWeight: 600,
          }}
        >
          <CheckCircle2 size={15} />
          <span>{feedback}</span>
        </div>
      )}

      {enhError && (
        <div
          style={{
            padding: '8px 14px',
            borderRadius: '8px',
            backgroundColor: 'rgba(220, 38, 38, 0.08)',
            border: '1px solid rgba(220, 38, 38, 0.25)',
            color: '#dc2626',
            fontSize: '12px',
            display: 'flex',
            alignItems: 'center',
            gap: '8px',
            fontWeight: 500,
          }}
        >
          <AlertTriangle size={14} />
          <span>{enhError}</span>
        </div>
      )}

      {/* Extension Gadgets Grid (Built-in Tab) */}
      {activeTab === 0 && (
      <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>

        {/* ========================================================================= */}
        {/* TOP GADGET: Left Panel Extensions Access Mode                            */}
        {/* ========================================================================= */}
        <div
          className="google-card"
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            padding: '16px 20px',
            gap: '16px',
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
            <PanelLeft size={16} color="var(--text-muted)" />
            <h3 style={{ margin: 0, fontSize: '15px', fontWeight: 700, color: 'var(--text)' }}>
              Left Panel Extensions Access Mode
            </h3>
          </div>

          <div style={{ display: 'flex', gap: '6px', backgroundColor: 'var(--tonal)', padding: '3px', borderRadius: '8px', flexShrink: 0 }}>
            <button
              type="button"
              onClick={() => handleLeftPanelModeChange('single')}
              style={{
                border: 'none',
                padding: '6px 12px',
                borderRadius: '6px',
                fontSize: '12px',
                fontWeight: (enhConfig?.left_panel_extensions_mode ?? 'single') === 'single' ? 600 : 500,
                backgroundColor: (enhConfig?.left_panel_extensions_mode ?? 'single') === 'single' ? '#ffffff' : 'transparent',
                color: (enhConfig?.left_panel_extensions_mode ?? 'single') === 'single' ? 'var(--primary)' : 'var(--text-muted)',
                boxShadow: (enhConfig?.left_panel_extensions_mode ?? 'single') === 'single' ? '0 1px 2px rgba(0,0,0,0.06)' : 'none',
                cursor: 'pointer',
                whiteSpace: 'nowrap',
              }}
            >
              Single Swiss Knife Button
            </button>
            <button
              type="button"
              onClick={() => handleLeftPanelModeChange('individual')}
              style={{
                border: 'none',
                padding: '6px 12px',
                borderRadius: '6px',
                fontSize: '12px',
                fontWeight: (enhConfig?.left_panel_extensions_mode ?? 'single') === 'individual' ? 600 : 500,
                backgroundColor: (enhConfig?.left_panel_extensions_mode ?? 'single') === 'individual' ? '#ffffff' : 'transparent',
                color: (enhConfig?.left_panel_extensions_mode ?? 'single') === 'individual' ? 'var(--primary)' : 'var(--text-muted)',
                boxShadow: (enhConfig?.left_panel_extensions_mode ?? 'single') === 'individual' ? '0 1px 2px rgba(0,0,0,0.06)' : 'none',
                cursor: 'pointer',
                whiteSpace: 'nowrap',
              }}
            >
              Individual Buttons
            </button>
          </div>
        </div>

        {/* ========================================================================= */}
        {/* GADGET 1: GitHub Workspace                                                */}
        {/* ========================================================================= */}
        <div className="google-card" style={{ display: 'flex', flexDirection: 'column', gap: (githubVis.aux_panel || githubVis.main_page) ? '14px' : 0, padding: '16px 20px' }}>
          {/* Header & Toggle Row */}
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '16px' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
              <GithubIcon size={16} color="var(--text-muted)" />
              <h3 style={{ margin: 0, fontSize: '15px', fontWeight: 700, color: 'var(--text)' }}>
                GitHub Workspace
              </h3>
            </div>
            {renderExtSwitches('github', 'GitHub Workspace')}
          </div>

          {/* Extension Settings & Actions */}
          {(githubVis.aux_panel || githubVis.main_page) && (
            <div style={{ marginTop: '4px', paddingTop: '12px', borderTop: '1px solid var(--border-subtle)', display: 'flex', alignItems: 'center', justifyContent: 'space-between', flexWrap: 'wrap', gap: '12px' }}>
              {/* Auxiliary Panel AGY Agents Display Settings */}
              <div style={{ width: '100%', display: 'flex', alignItems: 'center', justifyContent: 'space-between', flexWrap: 'wrap', gap: '12px' }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: '10px', flexWrap: 'wrap' }}>
                  <span style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', textTransform: 'uppercase' }}>
                    Aux AGY Agents:
                  </span>
                  <div style={{ display: 'flex', gap: '4px', backgroundColor: 'var(--tonal)', padding: '2px', borderRadius: '6px' }}>
                    <button
                      onClick={() => {
                        setAuxAgentScope('all')
                        localStorage.setItem('antigravity_github_aux_agent_scope', 'all')
                      }}
                      style={{
                        border: 'none',
                        padding: '4px 8px',
                        borderRadius: '6px',
                        fontSize: '12px',
                        fontWeight: auxAgentScope === 'all' ? 600 : 500,
                        backgroundColor: auxAgentScope === 'all' ? 'var(--accent, var(--primary, #1a73e8))' : 'transparent',
                        color: auxAgentScope === 'all' ? 'var(--accent-foreground, #ffffff)' : 'var(--text-muted)',
                        boxShadow: auxAgentScope === 'all' ? '0 1px 2px rgba(0,0,0,0.12)' : 'none',
                        cursor: 'pointer',
                        whiteSpace: 'nowrap',
                      }}
                    >
                      All Active Chats
                    </button>
                    <button
                      onClick={() => {
                        setAuxAgentScope('focused')
                        localStorage.setItem('antigravity_github_aux_agent_scope', 'focused')
                      }}
                      style={{
                        border: 'none',
                        padding: '4px 8px',
                        borderRadius: '6px',
                        fontSize: '12px',
                        fontWeight: auxAgentScope === 'focused' ? 600 : 500,
                        backgroundColor: auxAgentScope === 'focused' ? 'var(--accent, var(--primary, #1a73e8))' : 'transparent',
                        color: auxAgentScope === 'focused' ? 'var(--accent-foreground, #ffffff)' : 'var(--text-muted)',
                        boxShadow: auxAgentScope === 'focused' ? '0 1px 2px rgba(0,0,0,0.12)' : 'none',
                        cursor: 'pointer',
                        whiteSpace: 'nowrap',
                      }}
                    >
                      Focused Chat Only
                    </button>
                  </div>
                </div>

                {auxAgentScope === 'all' && (
                  <label style={{ display: 'inline-flex', alignItems: 'center', gap: '8px', cursor: 'pointer', fontSize: '12px', color: 'var(--text)' }}>
                    <span>Pin focused chat to top</span>
                    <ToggleSwitch
                      size="sm"
                      checked={auxPinFocused}
                      onChange={(val) => {
                        setAuxPinFocused(val)
                        localStorage.setItem('antigravity_github_aux_pin_focused', val ? 'true' : 'false')
                      }}
                      ariaLabel="Pin focused chat to top"
                    />
                  </label>
                )}
              </div>
            </div>
          )}
        </div>

        {/* ========================================================================= */}
        {/* GADGET 2: Preview Browser                                                 */}
        {/* ========================================================================= */}
        <div className="google-card" style={{ display: 'flex', flexDirection: 'column', gap: (browserVis.aux_panel || browserVis.main_page) ? '14px' : 0, padding: '16px 20px' }}>
          {/* Header & Toggle Row */}
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '16px' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
              <Globe size={16} color="var(--text-muted)" />
              <h3 style={{ margin: 0, fontSize: '15px', fontWeight: 700, color: 'var(--text)' }}>
                Preview Browser
              </h3>
            </div>
            {renderExtSwitches('browser', 'Preview Browser')}
          </div>

          {/* Extension Settings */}
          {(browserVis.aux_panel || browserVis.main_page) && (
            <div style={{ marginTop: '4px', paddingTop: '12px', borderTop: '1px solid var(--border-subtle)', display: 'flex', flexDirection: 'column', gap: '12px' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '12px', flexWrap: 'wrap' }}>
                <span style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', textTransform: 'uppercase', minWidth: '100px' }}>
                  Default URL:
                </span>
                <input
                  type="text"
                  value={previewUrl}
                  onChange={(e) => {
                    setPreviewUrl(e.target.value)
                    localStorage.setItem('antigravity_browser_default_url', e.target.value)
                  }}
                  style={{
                    flex: 1,
                    minWidth: '220px',
                    fontSize: '12px',
                    padding: '5px 10px',
                    fontFamily: 'monospace',
                    borderRadius: '6px',
                    border: '1px solid var(--border)',
                    backgroundColor: 'var(--canvas)',
                  }}
                  placeholder="http://localhost:8765"
                />
                <div style={{ display: 'flex', gap: '4px', flexWrap: 'wrap' }}>
                  {['8765', '5173', '3000', '8080'].map((port) => (
                    <button
                      key={port}
                      onClick={() => {
                        const url = `http://localhost:${port}`
                        setPreviewUrl(url)
                        localStorage.setItem('antigravity_browser_default_url', url)
                      }}
                      className="btn-pill-tonal"
                      style={{ padding: '3px 8px', fontSize: '11px', fontWeight: 600, whiteSpace: 'nowrap' }}
                    >
                      :{port}
                    </button>
                  ))}
                </div>
              </div>

              {/* Mobile Viewport Simulation Options */}
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', flexWrap: 'wrap', gap: '12px', paddingTop: '10px', borderTop: '1px dashed var(--border-subtle)' }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: '12px', flexWrap: 'wrap' }}>
                  <span style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', textTransform: 'uppercase', minWidth: '100px' }}>
                    Default Device:
                  </span>
                  <select
                    value={selectedDevice}
                    onChange={(e) => {
                      const dev = e.target.value as any
                      setSelectedDevice(dev)
                      localStorage.setItem('antigravity_mobile_default_device', dev)
                    }}
                    style={{
                      fontSize: '12px',
                      padding: '5px 10px',
                      borderRadius: '6px',
                      border: '1px solid var(--border)',
                      backgroundColor: 'var(--canvas)',
                    }}
                  >
                    <option value="iphone16">iPhone 16 Pro (393 × 852)</option>
                    <option value="pixel9">Google Pixel 9 (412 × 924)</option>
                    <option value="ipad">iPad Air (820 × 1180)</option>
                  </select>
                </div>

                <div style={{ display: 'flex', alignItems: 'center', gap: '16px' }}>
                  <label style={{ display: 'flex', alignItems: 'center', gap: '6px', fontSize: '12px', cursor: 'pointer', whiteSpace: 'nowrap' }}>
                    <span>Show Hardware Bezel</span>
                    <ToggleSwitch
                      size="sm"
                      checked={showBezel}
                      onChange={(val) => {
                        setShowBezel(val)
                        localStorage.setItem('antigravity_mobile_show_bezel', String(val))
                      }}
                    />
                  </label>
                </div>
              </div>
            </div>
          )}
        </div>

        {/* ========================================================================= */}
        {/* GADGET 3: Auxiliary File Explorer                                         */}
        {/* ========================================================================= */}
        <div className="google-card" style={{ display: 'flex', flexDirection: 'column', gap: (filesVis.aux_panel || filesVis.main_page) ? '14px' : 0, padding: '16px 20px' }}>
          {/* Header & Toggle Row */}
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '16px' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
              <Folder size={16} color="var(--text-muted)" />
              <h3 style={{ margin: 0, fontSize: '15px', fontWeight: 700, color: 'var(--text)' }}>
                Auxiliary File Explorer
              </h3>
            </div>
            {renderExtSwitches('files', 'Auxiliary File Explorer')}
          </div>

          {/* Extension Settings */}
          {(filesVis.aux_panel || filesVis.main_page) && (
            <div style={{ marginTop: '4px', paddingTop: '12px', borderTop: '1px solid var(--border-subtle)', display: 'flex', flexDirection: 'column', gap: '10px' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '12px', flexWrap: 'wrap' }}>
                <span style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', textTransform: 'uppercase', minWidth: '100px' }}>
                  Preferred IDE:
                </span>
                <select
                  value={ideIsCustom ? 'custom' : preferredIDE.toLowerCase()}
                  onChange={(e) => {
                    if (e.target.value === 'custom') {
                      setIdeCustomMode(true)
                      if (!IDE_PRESETS.includes(preferredIDE.toLowerCase())) {
                        setCustomIDE(preferredIDE)
                      }
                    } else {
                      setIdeCustomMode(false)
                      handleSaveIDE(e.target.value)
                    }
                  }}
                  style={{
                    fontSize: '12px',
                    padding: '5px 12px',
                    borderRadius: '6px',
                    border: '1px solid var(--border)',
                    backgroundColor: 'var(--canvas)',
                    color: 'var(--text)',
                  }}
                >
                  <option value="code">VS Code (`code`)</option>
                  <option value="cursor">Cursor (`cursor`)</option>
                  <option value="windsurf">Windsurf (`windsurf`)</option>
                  <option value="codium">VSCodium (`codium`)</option>
                  <option value="zed">Zed (`zed`)</option>
                  <option value="custom">Custom Command / Binary</option>
                </select>
              </div>

              {ideIsCustom && (
                <div style={{ display: 'flex', alignItems: 'center', gap: '12px', flexWrap: 'wrap' }}>
                  <span style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', textTransform: 'uppercase', minWidth: '100px' }}>
                    Command:
                  </span>
                  <input
                    type="text"
                    value={customIDE}
                    onChange={(e) => setCustomIDE(e.target.value)}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter') handleSaveIDE(customIDE)
                    }}
                    placeholder="e.g. code-insiders, fleet"
                    style={{
                      flex: 1,
                      minWidth: '180px',
                      maxWidth: '320px',
                      fontSize: '12px',
                      padding: '5px 10px',
                      fontFamily: 'monospace',
                      borderRadius: '6px',
                      border: '1px solid var(--border)',
                      backgroundColor: 'var(--canvas)',
                    }}
                  />
                  <button
                    onClick={() => handleSaveIDE(customIDE)}
                    className="btn-pill-primary"
                    style={{ padding: '6px 12px', fontSize: '12px', whiteSpace: 'nowrap' }}
                  >
                    Save IDE
                  </button>
                </div>
              )}
            </div>
          )}
        </div>

        {/* ========================================================================= */}
        {/* GADGET 4: Quick Memos                                                     */}
        {/* ========================================================================= */}
        <div className="google-card" style={{ display: 'flex', flexDirection: 'column', gap: (memosVis.aux_panel || memosVis.main_page) ? '14px' : 0, padding: '16px 20px' }}>
          {/* Header & Toggle Row */}
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '16px' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
              <FileText size={16} color="var(--text-muted)" />
              <h3 style={{ margin: 0, fontSize: '15px', fontWeight: 700, color: 'var(--text)' }}>
                Quick Memos
              </h3>
            </div>
            {renderExtSwitches('memos', 'Quick Memos')}
          </div>

          {/* Extension Settings (persisted via daemon /api/memos/config) */}
          {(memosVis.aux_panel || memosVis.main_page) && (
            <div style={{ marginTop: '4px', paddingTop: '12px', borderTop: '1px solid var(--border-subtle)', display: 'flex', flexDirection: 'column', gap: '10px' }}>
              {/* Row 1: Storage Location */}
              <div style={{ display: 'flex', alignItems: 'center', gap: '12px', flexWrap: 'wrap' }}>
                <span style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', textTransform: 'uppercase', minWidth: '100px' }}>
                  Storage Location:
                </span>
                <div style={{ display: 'flex', gap: '4px', backgroundColor: 'var(--tonal)', padding: '2px', borderRadius: '6px' }}>
                  <button
                    onClick={() => handleUpdateMemoConfig({ storage_location: 'global' })}
                    style={segBtnStyle(memoStorageLocation === 'global')}
                  >
                    Global
                  </button>
                  <button
                    onClick={() => handleUpdateMemoConfig({ storage_location: 'project' })}
                    style={segBtnStyle(memoStorageLocation === 'project')}
                  >
                    Project
                  </button>
                </div>
              </div>

              {/* Row 2: View Scope + Search Scope side by side */}
              <div style={{ display: 'flex', alignItems: 'center', gap: '24px', flexWrap: 'wrap' }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                  <span style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', textTransform: 'uppercase', minWidth: '100px' }}>
                    View Scope:
                  </span>
                  <div style={{ display: 'flex', gap: '4px', backgroundColor: 'var(--tonal)', padding: '2px', borderRadius: '6px' }}>
                    <button
                      onClick={() => handleUpdateMemoConfig({ view_scope: 'all' })}
                      style={segBtnStyle(memoViewScope === 'all')}
                    >
                      All Projects
                    </button>
                    <button
                      onClick={() => handleUpdateMemoConfig({ view_scope: 'current' })}
                      style={segBtnStyle(memoViewScope === 'current')}
                    >
                      Current Project
                    </button>
                  </div>
                </div>

                <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                  <span style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', textTransform: 'uppercase', minWidth: '100px' }}>
                    Search Scope:
                  </span>
                  <div style={{ display: 'flex', gap: '4px', backgroundColor: 'var(--tonal)', padding: '2px', borderRadius: '6px' }}>
                    <button
                      onClick={() => handleUpdateMemoConfig({ search_scope: 'text' })}
                      style={segBtnStyle(memoSearchScope === 'text')}
                    >
                      Text
                    </button>
                    <button
                      onClick={() => handleUpdateMemoConfig({ search_scope: 'all' })}
                      style={segBtnStyle(memoSearchScope === 'all')}
                    >
                      Semantic
                    </button>
                  </div>
                </div>
              </div>

              {/* Row 3: Text Extraction Mode */}
              <div style={{ display: 'flex', alignItems: 'center', gap: '12px', flexWrap: 'wrap' }}>
                <span style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', textTransform: 'uppercase', minWidth: '100px' }}>
                  Extraction:
                </span>
                <div style={{ display: 'flex', gap: '4px', backgroundColor: 'var(--tonal)', padding: '2px', borderRadius: '6px' }}>
                  <button
                    onClick={() => handleUpdateMemoConfig({ auto_extract: false })}
                    style={segBtnStyle(!memoAutoExtract)}
                    aria-pressed={!memoAutoExtract}
                  >
                    Manual
                  </button>
                  <button
                    onClick={() => handleUpdateMemoConfig({ auto_extract: true })}
                    style={segBtnStyle(memoAutoExtract)}
                    aria-pressed={memoAutoExtract}
                  >
                    Automatic
                  </button>
                </div>
              </div>

              {/* Row 4: Voice + OCR language */}
              <div style={{ display: 'flex', alignItems: 'center', gap: '24px', flexWrap: 'wrap' }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                  <span style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', textTransform: 'uppercase', minWidth: '100px' }}>
                    Voice Language:
                  </span>
                  <select
                    aria-label="Voice language"
                    value={memoVoiceLanguage}
                    onChange={(e) => handleUpdateMemoConfig({ voice_language: e.target.value })}
                    style={{
                      fontSize: '12px',
                      padding: '5px 10px',
                      borderRadius: '6px',
                      border: '1px solid var(--border)',
                      backgroundColor: 'var(--canvas)',
                      color: 'var(--text)',
                    }}
                  >
                    {MEMO_LANGUAGES.map((lang) => (
                      <option key={lang.code} value={lang.code}>
                        {lang.label}
                      </option>
                    ))}
                  </select>
                </div>

                <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                  <span style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', textTransform: 'uppercase', minWidth: '100px' }}>
                    OCR Language:
                  </span>
                  <select
                    aria-label="OCR language"
                    value={memoOcrLanguage}
                    onChange={(e) => handleUpdateMemoConfig({ ocr_language: e.target.value })}
                    style={{
                      fontSize: '12px',
                      padding: '5px 10px',
                      borderRadius: '6px',
                      border: '1px solid var(--border)',
                      backgroundColor: 'var(--canvas)',
                      color: 'var(--text)',
                    }}
                  >
                    {MEMO_LANGUAGES.map((lang) => (
                      <option key={lang.code} value={lang.code}>
                        {lang.label}
                      </option>
                    ))}
                  </select>
                </div>
              </div>
            </div>
          )}
        </div>

      </div>
      )}

      {/* Community Extensions Tab */}
      {activeTab === 1 && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
          {/* Top header card */}
          <div
            className="google-card"
            style={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
              padding: '16px 20px',
              gap: '16px',
              flexWrap: 'wrap',
            }}
          >
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
              <Blocks size={16} color="var(--text-muted)" />
              <h3 style={{ margin: 0, fontSize: '15px', fontWeight: 700, color: 'var(--text)' }}>
                Community Extensions
              </h3>
            </div>

            <div style={{ display: 'flex', alignItems: 'center', gap: '8px', flexShrink: 0 }}>
              <button
                type="button"
                onClick={() => {
                  setIsImportModalOpen(true)
                  setImportError(null)
                  setImportJsonText('')
                }}
                className="btn-pill-outlined"
                style={{
                  display: 'inline-flex',
                  alignItems: 'center',
                  gap: '6px',
                  fontSize: '12px',
                  padding: '6px 14px',
                  whiteSpace: 'nowrap',
                }}
              >
                <Upload size={14} />
                <span>Import JSON</span>
              </button>
              <button
                type="button"
                onClick={openCreateForm}
                className="btn-pill-primary"
                style={{
                  display: 'inline-flex',
                  alignItems: 'center',
                  gap: '6px',
                  fontSize: '12px',
                  padding: '6px 14px',
                  whiteSpace: 'nowrap',
                }}
              >
                <Plus size={14} />
                <span>Create Extension</span>
              </button>
            </div>
          </div>

          {/* Import Modal / Card */}
          {isImportModalOpen && (
            <div
              className="google-card"
              style={{
                display: 'flex',
                flexDirection: 'column',
                gap: '14px',
                padding: '18px 20px',
              }}
            >
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                  <Upload size={16} color="var(--primary)" />
                  <h3 style={{ margin: 0, fontSize: '15px', fontWeight: 700, color: 'var(--text)' }}>
                    Import Community Extensions
                  </h3>
                </div>
                <button
                  type="button"
                  onClick={() => {
                    setIsImportModalOpen(false)
                    setImportError(null)
                  }}
                  style={{
                    border: 'none',
                    background: 'transparent',
                    cursor: 'pointer',
                    padding: '4px',
                    color: 'var(--text-muted)',
                  }}
                  aria-label="Close Import Dialog"
                >
                  <X size={16} />
                </button>
              </div>

              <p style={{ margin: 0, fontSize: '13px', color: 'var(--text-muted)', lineHeight: 1.45 }}>
                Paste an extension JSON object or array of extensions, or upload a JSON file.
              </p>

              {importError && (
                <div
                  style={{
                    padding: '8px 12px',
                    borderRadius: '6px',
                    backgroundColor: 'rgba(220, 38, 38, 0.08)',
                    border: '1px solid rgba(220, 38, 38, 0.25)',
                    color: '#dc2626',
                    fontSize: '12px',
                    display: 'flex',
                    alignItems: 'center',
                    gap: '8px',
                  }}
                >
                  <AlertTriangle size={14} />
                  <span>{importError}</span>
                </div>
              )}

              <div style={{ display: 'flex', alignItems: 'center', gap: '12px', flexWrap: 'wrap' }}>
                <label
                  className="btn-pill-tonal"
                  style={{
                    display: 'inline-flex',
                    alignItems: 'center',
                    gap: '6px',
                    fontSize: '12px',
                    cursor: 'pointer',
                    whiteSpace: 'nowrap',
                    padding: '6px 12px',
                  }}
                >
                  <Upload size={14} />
                  <span>Choose JSON File</span>
                  <input
                    type="file"
                    accept=".json,application/json"
                    style={{ display: 'none' }}
                    onChange={(e) => {
                      const file = e.target.files?.[0]
                      if (file) {
                        const reader = new FileReader()
                        reader.onload = (ev) => {
                          const text = String(ev.target?.result || '')
                          setImportJsonText(text)
                        }
                        reader.readAsText(file)
                      }
                    }}
                  />
                </label>
                <span style={{ fontSize: '12px', color: 'var(--text-muted)' }}>or paste JSON below:</span>
              </div>

              <textarea
                aria-label="Import JSON Content"
                placeholder='{
  "id": "my-extension",
  "name": "My Extension",
  "source_type": "url",
  "url": "http://localhost:3000"
}'
                value={importJsonText}
                onChange={(e) => setImportJsonText(e.target.value)}
                rows={6}
                style={{
                  fontSize: '12px',
                  padding: '8px 12px',
                  borderRadius: '6px',
                  border: '1px solid var(--border)',
                  backgroundColor: 'var(--canvas)',
                  color: 'var(--text)',
                  fontFamily: 'monospace',
                  resize: 'vertical',
                }}
              />

              <div style={{ display: 'flex', alignItems: 'center', gap: '8px', justifyContent: 'flex-end' }}>
                <button
                  type="button"
                  onClick={() => {
                    setIsImportModalOpen(false)
                    setImportError(null)
                  }}
                  className="btn-pill-tonal"
                  style={{ fontSize: '12px', whiteSpace: 'nowrap' }}
                >
                  Cancel
                </button>
                <button
                  type="button"
                  onClick={handleImportSubmit}
                  className="btn-pill-primary"
                  style={{ fontSize: '12px', whiteSpace: 'nowrap' }}
                >
                  Import
                </button>
              </div>
            </div>
          )}

          {/* Create / Edit Extension Card */}
          {isEditorOpen && (
            <div
              className="google-card"
              style={{
                display: 'flex',
                flexDirection: 'column',
                gap: '16px',
                padding: '20px',
              }}
            >
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                  {editingId ? <Edit3 size={16} color="var(--primary)" /> : <Plus size={16} color="var(--primary)" />}
                  <h3 style={{ margin: 0, fontSize: '15px', fontWeight: 700, color: 'var(--text)' }}>
                    {editingId ? 'Edit Community Extension' : 'Create Community Extension'}
                  </h3>
                </div>
                <button
                  type="button"
                  onClick={() => setIsEditorOpen(false)}
                  style={{
                    border: 'none',
                    background: 'transparent',
                    cursor: 'pointer',
                    padding: '4px',
                    color: 'var(--text-muted)',
                  }}
                  aria-label="Close Editor"
                >
                  <X size={16} />
                </button>
              </div>

              {/* Form Grid */}
              <div style={{ display: 'flex', flexDirection: 'column', gap: '14px' }}>
                {/* Row 1: Name and ID */}
                <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(240px, 1fr))', gap: '14px' }}>
                  <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
                    <label style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text)' }}>
                      Name *
                    </label>
                    <input
                      type="text"
                      required
                      aria-label="Extension Name"
                      placeholder="e.g. SQLite Viewer"
                      value={formName}
                      onChange={(e) => {
                        const val = e.target.value
                        setFormName(val)
                        if (!editingId && !formIdEdited) {
                          setFormId(slugify(val))
                        }
                      }}
                      style={{
                        fontSize: '12px',
                        padding: '8px 12px',
                        borderRadius: '6px',
                        border: '1px solid var(--border)',
                        backgroundColor: 'var(--canvas)',
                        color: 'var(--text)',
                      }}
                    />
                  </div>

                  <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
                    <label style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text)' }}>
                      Extension ID *
                    </label>
                    <input
                      type="text"
                      aria-label="Extension ID"
                      placeholder="e.g. sqlite-viewer"
                      value={formId}
                      onChange={(e) => {
                        setFormIdEdited(true)
                        setFormId(e.target.value)
                      }}
                      disabled={Boolean(editingId)}
                      style={{
                        fontSize: '12px',
                        padding: '8px 12px',
                        borderRadius: '6px',
                        border: '1px solid var(--border)',
                        backgroundColor: editingId ? 'var(--tonal)' : 'var(--canvas)',
                        color: 'var(--text)',
                        fontFamily: 'monospace',
                      }}
                    />
                  </div>
                </div>

                {/* Row 2: Version and Author */}
                <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(240px, 1fr))', gap: '14px' }}>
                  <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
                    <label style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text)' }}>
                      Version
                    </label>
                    <input
                      type="text"
                      aria-label="Extension Version"
                      placeholder="1.0.0"
                      value={formVersion}
                      onChange={(e) => setFormVersion(e.target.value)}
                      style={{
                        fontSize: '12px',
                        padding: '8px 12px',
                        borderRadius: '6px',
                        border: '1px solid var(--border)',
                        backgroundColor: 'var(--canvas)',
                        color: 'var(--text)',
                      }}
                    />
                  </div>

                  <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
                    <label style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text)' }}>
                      Author
                    </label>
                    <input
                      type="text"
                      aria-label="Extension Author"
                      placeholder="Community"
                      value={formAuthor}
                      onChange={(e) => setFormAuthor(e.target.value)}
                      style={{
                        fontSize: '12px',
                        padding: '8px 12px',
                        borderRadius: '6px',
                        border: '1px solid var(--border)',
                        backgroundColor: 'var(--canvas)',
                        color: 'var(--text)',
                      }}
                    />
                  </div>
                </div>

                {/* Row 3: Description */}
                <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
                  <label style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text)' }}>
                    Description
                  </label>
                  <textarea
                    aria-label="Extension Description"
                    placeholder="Describe what this extension does..."
                    value={formDescription}
                    onChange={(e) => setFormDescription(e.target.value)}
                    rows={2}
                    style={{
                      fontSize: '12px',
                      padding: '8px 12px',
                      borderRadius: '6px',
                      border: '1px solid var(--border)',
                      backgroundColor: 'var(--canvas)',
                      color: 'var(--text)',
                      resize: 'vertical',
                    }}
                  />
                </div>

                {/* Row 4: Icon Selector */}
                <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
                  <label style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text)' }}>
                    Icon
                  </label>
                  <div style={{ display: 'flex', gap: '6px', flexWrap: 'wrap' }}>
                    {COMMUNITY_ICONS.map((item) => {
                      const isSelected = formIcon === item.id
                      return (
                        <button
                          key={item.id}
                          type="button"
                          onClick={() => setFormIcon(item.id)}
                          aria-label={`Select icon ${item.label}`}
                          style={{
                            display: 'inline-flex',
                            alignItems: 'center',
                            gap: '6px',
                            padding: '6px 12px',
                            borderRadius: '6px',
                            border: isSelected ? '1px solid var(--primary)' : '1px solid var(--border)',
                            backgroundColor: isSelected ? 'var(--tonal)' : 'var(--surface)',
                            color: isSelected ? 'var(--primary)' : 'var(--text)',
                            fontSize: '12px',
                            fontWeight: isSelected ? 600 : 500,
                            cursor: 'pointer',
                            whiteSpace: 'nowrap',
                          }}
                        >
                          {renderCommunityIcon(item.id, 14, isSelected ? 'var(--primary)' : 'var(--text-muted)')}
                          <span>{item.label}</span>
                        </button>
                      )
                    })}
                  </div>
                </div>

                {/* Row 5: Source Type */}
                <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
                  <label style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text)' }}>
                    Source Type
                  </label>
                  <div style={{ display: 'flex', gap: '4px', backgroundColor: 'var(--tonal)', padding: '2px', borderRadius: '6px', width: 'fit-content' }}>
                    <button
                      type="button"
                      onClick={() => setFormSourceType('url')}
                      style={segBtnStyle(formSourceType === 'url')}
                      aria-pressed={formSourceType === 'url'}
                    >
                      Web URL
                    </button>
                    <button
                      type="button"
                      onClick={() => setFormSourceType('html')}
                      style={segBtnStyle(formSourceType === 'html')}
                      aria-pressed={formSourceType === 'html'}
                    >
                      Custom HTML / JS
                    </button>
                  </div>
                </div>

                {/* Source Inputs */}
                {formSourceType === 'url' ? (
                  <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
                    <label style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text)' }}>
                      Target URL
                    </label>
                    <input
                      type="text"
                      aria-label="Extension Target URL"
                      placeholder="http://localhost:3000"
                      value={formUrl}
                      onChange={(e) => setFormUrl(e.target.value)}
                      style={{
                        fontSize: '12px',
                        padding: '8px 12px',
                        borderRadius: '6px',
                        border: '1px solid var(--border)',
                        backgroundColor: 'var(--canvas)',
                        color: 'var(--text)',
                        fontFamily: 'monospace',
                      }}
                    />
                  </div>
                ) : (
                  <div style={{ display: 'flex', flexDirection: 'column', gap: '12px' }}>
                    <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
                      <label style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text)' }}>
                        HTML Markup
                      </label>
                      <textarea
                        aria-label="Extension HTML"
                        placeholder="<div id='app'>Custom extension content</div>"
                        value={formHtml}
                        onChange={(e) => setFormHtml(e.target.value)}
                        rows={4}
                        style={{
                          fontSize: '12px',
                          padding: '8px 12px',
                          borderRadius: '6px',
                          border: '1px solid var(--border)',
                          backgroundColor: 'var(--canvas)',
                          color: 'var(--text)',
                          fontFamily: 'monospace',
                          resize: 'vertical',
                        }}
                      />
                    </div>

                    <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
                      <label style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text)' }}>
                        CSS Styles
                      </label>
                      <textarea
                        aria-label="Extension CSS"
                        placeholder="body { font-family: sans-serif; }"
                        value={formCss}
                        onChange={(e) => setFormCss(e.target.value)}
                        rows={3}
                        style={{
                          fontSize: '12px',
                          padding: '8px 12px',
                          borderRadius: '6px',
                          border: '1px solid var(--border)',
                          backgroundColor: 'var(--canvas)',
                          color: 'var(--text)',
                          fontFamily: 'monospace',
                          resize: 'vertical',
                        }}
                      />
                    </div>

                    <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
                      <label style={{ fontSize: '12px', fontWeight: 600, color: 'var(--text)' }}>
                        JavaScript
                      </label>
                      <textarea
                        aria-label="Extension JavaScript"
                        placeholder="console.log('Extension ready');"
                        value={formJs}
                        onChange={(e) => setFormJs(e.target.value)}
                        rows={4}
                        style={{
                          fontSize: '12px',
                          padding: '8px 12px',
                          borderRadius: '6px',
                          border: '1px solid var(--border)',
                          backgroundColor: 'var(--canvas)',
                          color: 'var(--text)',
                          fontFamily: 'monospace',
                          resize: 'vertical',
                        }}
                      />
                    </div>
                  </div>
                )}

                {/* Placement Toggles */}
                <div style={{ display: 'flex', alignItems: 'center', gap: '20px', flexWrap: 'wrap', paddingTop: '8px', borderTop: '1px solid var(--border-subtle)' }}>
                  <span style={extSwitchLabelStyle}>
                    Auxiliary Panel
                    <ToggleSwitch
                      size="sm"
                      checked={formAuxPanel}
                      onChange={setFormAuxPanel}
                      ariaLabel="Extension available in auxiliary panel"
                    />
                  </span>
                  <span style={extSwitchLabelStyle}>
                    Main Page
                    <ToggleSwitch
                      size="sm"
                      checked={formMainPage}
                      onChange={setFormMainPage}
                      ariaLabel="Extension available in main page"
                    />
                  </span>
                </div>

                {/* Action Buttons */}
                <div style={{ display: 'flex', alignItems: 'center', gap: '8px', justifyContent: 'flex-end', marginTop: '8px' }}>
                  <button
                    type="button"
                    onClick={() => setIsEditorOpen(false)}
                    className="btn-pill-tonal"
                    style={{ fontSize: '12px', whiteSpace: 'nowrap' }}
                  >
                    Cancel
                  </button>
                  <button
                    type="button"
                    onClick={handleSaveExtension}
                    className="btn-pill-primary"
                    style={{ fontSize: '12px', whiteSpace: 'nowrap' }}
                  >
                    Save Extension
                  </button>
                </div>
              </div>
            </div>
          )}

          {/* Community Extensions List / Empty State */}
          {communityExtensions.length === 0 && !isEditorOpen ? (
            <div
              className="google-card"
              style={{
                padding: '36px 20px',
                textAlign: 'center',
                color: 'var(--text-muted)',
                fontSize: '13px',
                lineHeight: 1.5,
              }}
            >
              No community extensions installed. Click Create Extension or Import JSON to add your first custom extension.
            </div>
          ) : (
            communityExtensions.map((ext) => (
              <div
                key={ext.id}
                className="google-card"
                style={{
                  display: 'flex',
                  alignItems: 'flex-start',
                  justifyContent: 'space-between',
                  padding: '18px 20px',
                  gap: '16px',
                  flexWrap: 'wrap',
                }}
              >
                {/* Left Side */}
                <div style={{ display: 'flex', flexDirection: 'column', gap: '8px', flex: 1, minWidth: '260px' }}>
                  <div style={{ display: 'flex', alignItems: 'flex-start', gap: '10px' }}>
                    <div
                      style={{
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'center',
                        width: '32px',
                        height: '32px',
                        borderRadius: '8px',
                        backgroundColor: 'var(--tonal)',
                        flexShrink: 0,
                        marginTop: '2px',
                      }}
                    >
                      {renderCommunityIcon(ext.icon, 18, 'var(--primary)')}
                    </div>
                    <div style={{ display: 'flex', flexDirection: 'column', gap: '2px' }}>
                      <div style={{ display: 'flex', alignItems: 'center', gap: '8px', flexWrap: 'wrap' }}>
                        <h3 style={{ margin: 0, fontSize: '15px', fontWeight: 700, color: 'var(--text)' }}>
                          {ext.name}
                        </h3>
                        <span
                          style={{
                            fontSize: '11px',
                            padding: '1px 6px',
                            borderRadius: '4px',
                            backgroundColor: 'var(--tonal)',
                            color: 'var(--text-muted)',
                            fontWeight: 600,
                            whiteSpace: 'nowrap',
                          }}
                        >
                          v{ext.version || '1.0.0'}
                        </span>
                        <span style={{ fontSize: '12px', color: 'var(--text-muted)', whiteSpace: 'nowrap' }}>
                          by {ext.author || 'Community'}
                        </span>
                      </div>
                      {ext.description && (
                        <p style={{ margin: '2px 0 0', fontSize: '13px', color: 'var(--text-muted)', lineHeight: 1.45 }}>
                          {ext.description}
                        </p>
                      )}
                    </div>
                  </div>

                  {/* Source Badge */}
                  <div style={{ display: 'flex', alignItems: 'center', gap: '8px', flexWrap: 'wrap', marginTop: '2px' }}>
                    <span
                      style={{
                        display: 'inline-flex',
                        alignItems: 'center',
                        gap: '5px',
                        fontSize: '11px',
                        fontFamily: 'monospace',
                        padding: '3px 8px',
                        borderRadius: '4px',
                        backgroundColor: 'var(--tonal)',
                        color: 'var(--text)',
                        maxWidth: '480px',
                        overflow: 'hidden',
                        textOverflow: 'ellipsis',
                        whiteSpace: 'nowrap',
                      }}
                    >
                      {ext.source_type === 'url' ? <Globe size={12} /> : <Code size={12} />}
                      <span>{ext.source_type === 'url' ? ext.url : 'Custom HTML / JS'}</span>
                    </span>
                  </div>

                  {/* Actions */}
                  <div style={{ display: 'flex', alignItems: 'center', gap: '6px', flexWrap: 'wrap', marginTop: '4px' }}>
                    <button
                      type="button"
                      onClick={() => openEditForm(ext)}
                      className="btn-pill-tonal"
                      style={{ display: 'inline-flex', alignItems: 'center', gap: '5px', padding: '4px 10px', fontSize: '12px', whiteSpace: 'nowrap' }}
                    >
                      <Edit3 size={13} />
                      <span>Edit</span>
                    </button>
                    <button
                      type="button"
                      onClick={() => handleExportJson(ext)}
                      className="btn-pill-tonal"
                      style={{ display: 'inline-flex', alignItems: 'center', gap: '5px', padding: '4px 10px', fontSize: '12px', whiteSpace: 'nowrap' }}
                    >
                      <Download size={13} />
                      <span>Export JSON</span>
                    </button>
                    <button
                      type="button"
                      onClick={() => handleDeleteCommunityExtension(ext.id)}
                      className="btn-pill-tonal"
                      style={{ display: 'inline-flex', alignItems: 'center', gap: '5px', padding: '4px 10px', fontSize: '12px', whiteSpace: 'nowrap', color: '#dc2626' }}
                    >
                      <Trash2 size={13} />
                      <span>Delete</span>
                    </button>
                  </div>
                </div>

                {/* Right Side: Visibility Toggles */}
                <div style={{ display: 'flex', flexDirection: 'row', alignItems: 'center', gap: '16px', flexShrink: 0 }}>
                  <span style={extSwitchLabelStyle} title="Available in the IDE's right auxiliary panel">
                    Auxiliary Panel
                    <ToggleSwitch
                      size="sm"
                      checked={ext.aux_panel !== false}
                      onChange={(v) => handleCommunityVisChange(ext, 'aux_panel', v)}
                      ariaLabel={`${ext.name} available in auxiliary panel`}
                    />
                  </span>
                  <span style={extSwitchLabelStyle} title="Openable on the main stage via a left-sidebar tab button">
                    Main Page
                    <ToggleSwitch
                      size="sm"
                      checked={ext.main_page === true}
                      onChange={(v) => handleCommunityVisChange(ext, 'main_page', v)}
                      ariaLabel={`${ext.name} available in main page`}
                    />
                  </span>
                </div>
              </div>
            ))
          )}
        </div>
      )}

    </div>
  )
}

export { ExtensionsPage as FeaturePluginsPage }
