import React, { useState, useEffect, useMemo, useRef } from 'react'
import {
  StickyNote,
  RotateCw,
  Edit3,
  Square,
  Crosshair,
  Send,
  Trash2,
  Plus,
  Mic,
  MicOff,
  Check,
  Search,
  X,
  ArrowLeft,
  ArrowRight,
  ArrowUp,
  Folder,
  Globe,
  FileCode,
  Code,
  FileText,
  Terminal,
  File,
  Highlighter,
  Underline,
  ChevronRight,
  Tablet,
  CheckCircle2,
  Save,
  Copy,
  Scissors,
  Clipboard,
} from 'lucide-react'
import { ToggleSwitch } from '../components/ToggleSwitch'
import { api } from '../api'
import { filterMemos, type MemoItem, type SearchScope } from '../utils/memoSearch'
import {
  formatDuration,
  resolveVoiceMemoTitle,
  getSupportedAudioMimeType,
  blobToBase64,
} from '../utils/memoVoice'
import { GitHubWorkspacePage } from './GitHubWorkspacePage'

export interface ExtensionsPageProps {
  activeTab?: number
  onTabChange?: (tab: number) => void
  scope?: string
  onScopeChange?: (scope: string) => void
  fallbackProject?: string
}

export const ExtensionsPage: React.FC<ExtensionsPageProps> = ({
  activeTab: propActiveTab,
  onTabChange: _onTabChange,
  scope,
  onScopeChange: _onScopeChange,
  fallbackProject,
}) => {
  const activeTab = propActiveTab !== undefined ? propActiveTab : 0

  // --- 1. Preview Browser State ---
  const [previewUrl, setPreviewUrl] = useState('http://localhost:5173')
  const [activeAnnotateTool, setActiveAnnotateTool] = useState<'none' | 'pen' | 'rect' | 'inspect'>('none')
  const [annotationColor] = useState('#ea4335') // Google Red
  const [commentText, setCommentText] = useState('')
  const [payloadType, setPayloadType] = useState<'hybrid' | 'screenshot' | 'code'>('hybrid')
  const [sentFeedback, setSentFeedback] = useState<string | null>(null)
  const [showIpadModal, setShowIpadModal] = useState(false)
  const [capturedSnippet, setCapturedSnippet] = useState<string>('')

  // --- 2. File Explorer State ---
  const [currentProjectFolder, setCurrentProjectFolder] = useState('.')
  const [addressBarPath, setAddressBarPath] = useState('.')
  const [historyStack, setHistoryStack] = useState<string[]>(['.'])
  const [historyIndex, setHistoryIndex] = useState(0)
  const [searchQuery, setSearchQuery] = useState('')
  const [filesList, setFilesList] = useState<Array<{ name: string; isDir: boolean; type: string; size: string; path: string; modTime: string }>>([])
  const [isFilesLoading, setIsFilesLoading] = useState(false)
  const [activeFileViewer, setActiveFileViewer] = useState<{ name: string; type: 'code' | 'markdown' | 'pdf' | 'office'; path: string; content?: string } | null>(null)
  const [codeSnippetToAnnotate, setCodeSnippetToAnnotate] = useState('')
  const [codeLineRange, setCodeLineRange] = useState('L1-L20')
  const [pdfHighlightMode, setPdfHighlightMode] = useState<'highlight' | 'underline'>('highlight')
  const [copiedFileFeedback, setCopiedFileFeedback] = useState<string | null>(null)
  const [fileEditorContent, setFileEditorContent] = useState<string>('')
  const [isSavingFile, setIsSavingFile] = useState<boolean>(false)
  const [fileSaveFeedback, setFileSaveFeedback] = useState<string | null>(null)
  const [isCodeEditingMode, setIsCodeEditingMode] = useState<boolean>(false)
  const [markdownViewMode, setMarkdownViewMode] = useState<'preview' | 'edit'>('edit')
  const [selectedFilePaths, setSelectedFilePaths] = useState<string[]>([])
  const [fileClipboard, setFileClipboard] = useState<{ action: 'copy' | 'cut'; items: Array<{ path: string; name: string; isDir: boolean }> }>({ action: 'copy', items: [] })
  const [contextMenu, setContextMenu] = useState<{
    x: number
    y: number
    type: 'row' | 'blank'
    targetItem?: { name: string; isDir: boolean; type: string; size: string; path: string }
  } | null>(null)
  const [preferredIDE, setPreferredIDE] = useState<string>(() => {
    return localStorage.getItem('antigravity_preferred_ide') || 'code'
  })

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

  // --- 3. Quick Memos State ---
  const [memos, setMemos] = useState<MemoItem[]>(() => {
    try {
      const saved = localStorage.getItem('antigravity_memos')
      return saved ? JSON.parse(saved) : []
    } catch {
      return []
    }
  })
  const [newMemoText, setNewMemoText] = useState('')
  const [isRecording, setIsRecording] = useState(false)
  const [recordingSeconds, setRecordingSeconds] = useState(0)
  const [memoSearchQuery, setMemoSearchQuery] = useState('')
  const [memoSearchScope, setMemoSearchScope] = useState<SearchScope>(() => {
    return (localStorage.getItem('antigravity_memo_search_scope') as SearchScope) || 'text'
  })
  const [memoStorageLocation, setMemoStorageLocation] = useState<'global' | 'project'>(() => {
    return (localStorage.getItem('antigravity_memo_storage_location') as 'global' | 'project') || 'global'
  })
  const [memoViewScope, setMemoViewScope] = useState<'all' | 'current'>(() => {
    return (localStorage.getItem('antigravity_memo_view_scope') as 'all' | 'current') || 'all'
  })

  const mediaRecorderRef = useRef<MediaRecorder | null>(null)
  const mediaStreamRef = useRef<MediaStream | null>(null)
  const audioChunksRef = useRef<Blob[]>([])
  const speechRecognitionRef = useRef<any>(null)
  const transcriptRef = useRef<string>('')
  const recordIntervalRef = useRef<any>(null)
  const recordingStartTimeRef = useRef<number>(0)

  useEffect(() => {
    return () => {
      if (recordIntervalRef.current) {
        clearInterval(recordIntervalRef.current)
      }
      if (speechRecognitionRef.current) {
        try {
          speechRecognitionRef.current.stop()
        } catch {}
      }
      if (mediaStreamRef.current) {
        mediaStreamRef.current.getTracks().forEach((t) => t.stop())
      }
    }
  }, [])

  const displayedMemos = useMemo(() => {
    return filterMemos(memos, memoSearchQuery, memoSearchScope)
  }, [memos, memoSearchQuery, memoSearchScope])

  // Sync memos to localStorage
  useEffect(() => {
    try {
      localStorage.setItem('antigravity_memos', JSON.stringify(memos))
    } catch {}
  }, [memos])

  const loadMemos = async (
    scopeOverride?: 'all' | 'current',
    wsOverride?: string,
    storageOverride?: 'global' | 'project'
  ) => {
    const scopeToUse = scopeOverride || memoViewScope
    const storageToUse = storageOverride || memoStorageLocation
    const currentWs = wsOverride !== undefined ? wsOverride : addressBarPath
    const effectiveWs = currentWs && currentWs !== '.' ? currentWs : undefined

    try {
      const res = await api.getMemos({
        workspacePath: effectiveWs,
        storage: storageToUse,
        scope: scopeToUse,
      })
      if (res && Array.isArray(res.memos)) {
        setMemos(res.memos)
      }
    } catch {
      try {
        const saved = localStorage.getItem('antigravity_memos')
        if (saved) setMemos(JSON.parse(saved))
      } catch {}
    }
  }

  // Fetch remote memo config on mount and sync state & localStorage
  useEffect(() => {
    api.getMemoConfig().then((res) => {
      if (res && res.success && res.config) {
        if (res.config.storage_location === 'global' || res.config.storage_location === 'project') {
          setMemoStorageLocation(res.config.storage_location)
          localStorage.setItem('antigravity_memo_storage_location', res.config.storage_location)
        }
        if (res.config.view_scope === 'all' || res.config.view_scope === 'current') {
          setMemoViewScope(res.config.view_scope)
          localStorage.setItem('antigravity_memo_view_scope', res.config.view_scope)
        }
        if (res.config.search_scope === 'text' || res.config.search_scope === 'all') {
          setMemoSearchScope(res.config.search_scope as SearchScope)
          localStorage.setItem('antigravity_memo_search_scope', res.config.search_scope)
        }
      }
    }).catch(() => {})
  }, [])

  // Sync memos whenever scope, storage location, or workspace directory changes
  useEffect(() => {
    loadMemos(memoViewScope, addressBarPath, memoStorageLocation)
  }, [memoViewScope, memoStorageLocation, addressBarPath])

  const handleToggleViewScope = async (nextScope: 'all' | 'current') => {
    setMemoViewScope(nextScope)
    localStorage.setItem('antigravity_memo_view_scope', nextScope)
    try {
      await api.updateMemoConfig({ view_scope: nextScope })
    } catch {}
    loadMemos(nextScope, addressBarPath, memoStorageLocation)
  }

  // Sync top-bar scope selector with File Explorer and Quick Memos
  useEffect(() => {
    if (!scope) return
    if (scope === 'GLOBAL') {
      setMemoViewScope('all')
      if (currentProjectFolder !== '.' && currentProjectFolder !== '~/.gemini/antigravity') {
        setCurrentProjectFolder('.')
        setAddressBarPath('.')
        setHistoryStack(['.'])
        setHistoryIndex(0)
      }
    } else {
      setCurrentProjectFolder(scope)
      setAddressBarPath(scope)
      setHistoryStack([scope])
      setHistoryIndex(0)
      setMemoViewScope('current')
    }
  }, [scope])

  // Load real files from Go backend
  useEffect(() => {
    loadFiles(addressBarPath)
  }, [addressBarPath])

  const loadFiles = async (dir: string) => {
    try {
      setIsFilesLoading(true)
      const res = await api.listFiles(dir)
      if (res && res.files) {
        setFilesList(res.files)
        if (res.path && (!addressBarPath || addressBarPath === '.')) {
          setAddressBarPath(res.path)
          setHistoryStack([res.path])
        }
      }
    } catch (err) {
      console.error('Error listing files:', err)
    } finally {
      setIsFilesLoading(false)
    }
  }

  // Dismiss context menu on outside events
  useEffect(() => {
    const handleCloseCtx = () => setContextMenu(null)
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setContextMenu(null)
    }
    if (contextMenu) {
      window.addEventListener('click', handleCloseCtx)
      window.addEventListener('contextmenu', handleCloseCtx)
      window.addEventListener('keydown', handleKeyDown)
      window.addEventListener('resize', handleCloseCtx)
    }
    return () => {
      window.removeEventListener('click', handleCloseCtx)
      window.removeEventListener('contextmenu', handleCloseCtx)
      window.removeEventListener('keydown', handleKeyDown)
      window.removeEventListener('resize', handleCloseCtx)
    }
  }, [contextMenu])

  const handleRowClick = (e: React.MouseEvent, item: { name: string; isDir: boolean; type: string; size: string; path: string }) => {
    const isMulti = e.ctrlKey || e.metaKey
    if (isMulti) {
      e.preventDefault()
      e.stopPropagation()
      if (selectedFilePaths.includes(item.path)) {
        setSelectedFilePaths(selectedFilePaths.filter((p) => p !== item.path))
      } else {
        setSelectedFilePaths([...selectedFilePaths, item.path])
      }
      return
    }

    setSelectedFilePaths([item.path])
    if (item.isDir) {
      handleNavigatePath(item.path)
    } else {
      handleSelectFile(item)
    }
  }

  const handleRowContextMenu = (e: React.MouseEvent, item: { name: string; isDir: boolean; type: string; size: string; path: string }) => {
    e.preventDefault()
    e.stopPropagation()
    window.getSelection()?.removeAllRanges()

    if (!selectedFilePaths.includes(item.path)) {
      setSelectedFilePaths([item.path])
    }

    const pad = 8
    const menuWidth = 190
    const menuHeight = 280
    const x = Math.min(e.clientX, window.innerWidth - menuWidth - pad)
    const y = Math.min(e.clientY, window.innerHeight - menuHeight - pad)

    setContextMenu({
      x,
      y,
      type: 'row',
      targetItem: item,
    })
  }

  const handleBlankContextMenu = (e: React.MouseEvent) => {
    if ((e.target as HTMLElement).closest('.swiss-file-item-row')) return
    e.preventDefault()
    e.stopPropagation()
    window.getSelection()?.removeAllRanges()

    const pad = 8
    const menuWidth = 190
    const menuHeight = 220
    const x = Math.min(e.clientX, window.innerWidth - menuWidth - pad)
    const y = Math.min(e.clientY, window.innerHeight - menuHeight - pad)

    setContextMenu({
      x,
      y,
      type: 'blank',
    })
  }

  const handleCopySelected = () => {
    const targets = filesList.filter((f) => selectedFilePaths.includes(f.path))
    const items = targets.length > 0 ? targets : (contextMenu?.targetItem ? [contextMenu.targetItem] : [])
    setFileClipboard({ action: 'copy', items })
    setCopiedFileFeedback(`Copied ${items.length} item(s) to clipboard`)
    setTimeout(() => setCopiedFileFeedback(null), 3000)
    setContextMenu(null)
  }

  const handleCutSelected = () => {
    const targets = filesList.filter((f) => selectedFilePaths.includes(f.path))
    const items = targets.length > 0 ? targets : (contextMenu?.targetItem ? [contextMenu.targetItem] : [])
    setFileClipboard({ action: 'cut', items })
    setCopiedFileFeedback(`Cut ${items.length} item(s) to clipboard`)
    setTimeout(() => setCopiedFileFeedback(null), 3000)
    setContextMenu(null)
  }

  const handlePaste = async (targetFolder?: string) => {
    if (!fileClipboard.items || fileClipboard.items.length === 0) return
    const isCut = fileClipboard.action === 'cut'
    const dest = targetFolder || addressBarPath
    const payload = fileClipboard.items.map((i) => ({
      src: i.path,
      dst: `${dest}/${i.name}`,
    }))

    try {
      const res = isCut ? await api.moveFiles(payload) : await api.copyFiles(payload)
      if (res && res.success) {
        setCopiedFileFeedback(`${isCut ? 'Moved' : 'Copied'} ${fileClipboard.items.length} item(s)`)
        if (isCut) {
          setFileClipboard({ action: 'copy', items: [] })
        }
        loadFiles(addressBarPath)
      } else {
        setCopiedFileFeedback('Paste error')
      }
    } catch (err: any) {
      setCopiedFileFeedback(`Paste failed: ${err.message}`)
    }
    setTimeout(() => setCopiedFileFeedback(null), 3000)
    setContextMenu(null)
  }

  const handleDeleteSelected = async () => {
    const targets = filesList.filter((f) => selectedFilePaths.includes(f.path))
    const items = targets.length > 0 ? targets : (contextMenu?.targetItem ? [contextMenu.targetItem] : [])
    if (items.length === 0) return
    const count = items.length
    const msg = count === 1 ? `Delete '${items[0].name}'?` : `Delete ${count} selected items?`
    if (!window.confirm(msg)) return

    try {
      const res = await api.deleteFiles(items.map((i) => i.path))
      if (res && res.success) {
        setCopiedFileFeedback(`Deleted ${count} item(s)`)
        setSelectedFilePaths([])
        loadFiles(addressBarPath)
      } else {
        setCopiedFileFeedback('Delete failed')
      }
    } catch (err: any) {
      setCopiedFileFeedback(`Delete failed: ${err.message}`)
    }
    setTimeout(() => setCopiedFileFeedback(null), 3000)
    setContextMenu(null)
  }

  const handleRenameItem = async () => {
    if (!contextMenu?.targetItem) return
    const item = contextMenu.targetItem
    const newName = window.prompt('Rename to:', item.name)
    if (!newName || newName === item.name) return
    const parentDir = item.path.substring(0, item.path.lastIndexOf('/')) || '.'
    const newPath = `${parentDir}/${newName}`
    try {
      const res = await api.renameFile(item.path, newPath)
      if (res && res.success) {
        setCopiedFileFeedback(`Renamed to '${newName}'`)
        loadFiles(addressBarPath)
      }
    } catch (err: any) {
      setCopiedFileFeedback(`Rename failed: ${err.message}`)
    }
    setTimeout(() => setCopiedFileFeedback(null), 3000)
    setContextMenu(null)
  }

  const handleSelectFile = async (item: { name: string; isDir: boolean; type: string; path: string }) => {
    if (item.isDir) {
      handleNavigatePath(item.path)
      return
    }
    try {
      const res = await api.readFile(item.path)
      const text = res.content || ''
      setFileEditorContent(text)
      setActiveFileViewer({
        name: item.name,
        type: item.type as any,
        path: item.path,
        content: text,
      })
    } catch (err) {
      console.error('Error reading file:', err)
      const errText = `Error loading file: ${err}`
      setFileEditorContent(errText)
      setActiveFileViewer({
        name: item.name,
        type: item.type as any,
        path: item.path,
        content: errText,
      })
    }
  }

  const handleSaveFileContent = async () => {
    if (!activeFileViewer) return
    setIsSavingFile(true)
    setFileSaveFeedback(null)
    try {
      await api.writeFile(activeFileViewer.path, fileEditorContent)
      setActiveFileViewer((prev) => (prev ? { ...prev, content: fileEditorContent } : null))
      setFileSaveFeedback('Saved to disk!')
      setTimeout(() => setFileSaveFeedback(null), 3000)
    } catch (err: any) {
      setFileSaveFeedback(`Save error: ${err.message}`)
    } finally {
      setIsSavingFile(false)
    }
  }

  // --- 4. Mobile Simulator State ---
  const [selectedDevice, setSelectedDevice] = useState<'iphone16' | 'pixel9' | 'ipad'>('iphone16')
  const [isLandscape, setIsLandscape] = useState(false)
  const [showBezel, setShowBezel] = useState(true)

  // --- 5. Computer Use State ---
  const [dpiNormalization, setDpiNormalization] = useState(true)
  const [waylandPipeWire, setWaylandPipeWire] = useState(true)
  const [accessibilityGrounding, setAccessibilityGrounding] = useState(true)

  // Handlers for Preview Browser
  const handleSendToChat = () => {
    let payload = ''
    if (payloadType === 'hybrid') {
      payload = `[UI Annotation @ ${previewUrl}]\nComment: "${commentText || 'Check layout and design alignment'}"\nSelected Element:\n\`\`\`html\n${capturedSnippet}\n\`\`\`\n[Attached: Annotated High-Res Screenshot Canvas]`
    } else if (payloadType === 'screenshot') {
      payload = `[UI Screenshot @ ${previewUrl}]: "${commentText || 'Please inspect attached design annotations'}"`
    } else {
      payload = `[DOM Snippet @ ${previewUrl}]:\n\`\`\`html\n${capturedSnippet}\n\`\`\`\nComment: "${commentText}"`
    }

    navigator.clipboard.writeText(payload)
    setSentFeedback('Annotation payload copied & attached! Ready to paste into Antigravity chat.')
    setTimeout(() => setSentFeedback(null), 3500)
    setCommentText('')
  }

  // Handlers for File Explorer
  const handleNavigatePath = (newPath: string) => {
    setAddressBarPath(newPath)
    const nextHistory = historyStack.slice(0, historyIndex + 1)
    nextHistory.push(newPath)
    setHistoryStack(nextHistory)
    setHistoryIndex(nextHistory.length - 1)
  }

  const handleBack = () => {
    if (historyIndex > 0) {
      const newIdx = historyIndex - 1
      setHistoryIndex(newIdx)
      setAddressBarPath(historyStack[newIdx])
    }
  }

  const handleForward = () => {
    if (historyIndex < historyStack.length - 1) {
      const newIdx = historyIndex + 1
      setHistoryIndex(newIdx)
      setAddressBarPath(historyStack[newIdx])
    }
  }

  const handleSendCodeAnnotationToChat = () => {
    if (!activeFileViewer) return
    const text = `[Code Annotation: ${activeFileViewer.name}#${codeLineRange}]\n\`\`\`\n${codeSnippetToAnnotate || '// Selected lines from editor'}\n\`\`\`\nComment: "${commentText || 'Please review these lines and propose refactoring'}"`
    navigator.clipboard.writeText(text)
    setCopiedFileFeedback(`Snippet ${codeLineRange} copied! Attached to Antigravity chat input.`)
    setTimeout(() => setCopiedFileFeedback(null), 3000)
  }

  const handleAddTextMemo = async () => {
    if (!newMemoText.trim()) return
    const effectiveWs = addressBarPath && addressBarPath !== '.' ? addressBarPath : undefined
    const projectName = effectiveWs ? effectiveWs.split('/').filter(Boolean).pop() : undefined

    const memoObj: MemoItem = {
      id: `memo-${Date.now()}`,
      type: 'text' as const,
      content: newMemoText.trim(),
      title: newMemoText.trim().substring(0, 24),
      workspace_path: effectiveWs,
      project: projectName,
      createdAt: 'Just now',
      color: '#e8f0fe',
    }
    setMemos([memoObj, ...memos])
    setNewMemoText('')
    try {
      await api.saveMemo(memoObj, {
        workspacePath: effectiveWs,
        storage: memoStorageLocation,
      })
    } catch {}
  }

  const handleDeleteMemo = async (id: string) => {
    setMemos(memos.filter(m => m.id !== id))
    const effectiveWs = addressBarPath && addressBarPath !== '.' ? addressBarPath : undefined
    try {
      await api.deleteMemo(id, {
        workspacePath: effectiveWs,
        storage: memoStorageLocation,
      })
    } catch {}
  }

  const stopAndSaveRecording = async () => {
    if (recordIntervalRef.current) {
      clearInterval(recordIntervalRef.current)
      recordIntervalRef.current = null
    }
    setIsRecording(false)

    // Stop Speech Recognition
    if (speechRecognitionRef.current) {
      try {
        speechRecognitionRef.current.stop()
      } catch {}
      speechRecognitionRef.current = null
    }

    const recorder = mediaRecorderRef.current
    if (!recorder) return

    // Stop recorder and await stop event
    const stopPromise = new Promise<void>((resolve) => {
      recorder.onstop = () => resolve()
    })

    if (recorder.state === 'recording') {
      recorder.stop()
    }
    await stopPromise

    // Release microphone tracks
    if (mediaStreamRef.current) {
      mediaStreamRef.current.getTracks().forEach((track) => track.stop())
      mediaStreamRef.current = null
    }

    const recordedBlob = new Blob(audioChunksRef.current, {
      type: recorder.mimeType || 'audio/webm',
    })
    const base64Audio = await blobToBase64(recordedBlob)

    const elapsedSeconds = Math.max(1, Math.round((Date.now() - recordingStartTimeRef.current) / 1000))
    const formattedDuration = formatDuration(elapsedSeconds)
    const finalTranscript = transcriptRef.current.trim()

    let promptTitle: string | null = null
    try {
      if (finalTranscript) {
        promptTitle = window.prompt('Voice recorded & transcribed! Edit title:', finalTranscript)
      } else {
        promptTitle = window.prompt('Voice recorded! Enter a transcript / note title:', 'Voice Memo Note')
      }
    } catch {
      promptTitle = null
    }

    const effectiveTitle = promptTitle !== null && promptTitle.trim()
      ? promptTitle.trim()
      : resolveVoiceMemoTitle(finalTranscript, formattedDuration)

    const finalTitle = effectiveTitle.toLowerCase().startsWith('[voice]')
      ? effectiveTitle
      : `[Voice] ${effectiveTitle}`

    const effectiveWs = addressBarPath && addressBarPath !== '.' ? addressBarPath : undefined
    const projectName = effectiveWs ? effectiveWs.split('/').filter(Boolean).pop() : undefined

    const audioMemo: MemoItem = {
      id: `memo-audio-${Date.now()}`,
      type: 'voice',
      title: finalTitle,
      content: finalTranscript || effectiveTitle,
      transcript: finalTranscript,
      audio_data: base64Audio,
      audioData: base64Audio,
      duration: formattedDuration,
      workspace_path: effectiveWs,
      project: projectName,
      createdAt: 'Just now',
      created_at: new Date().toLocaleString(),
      color: '#fef7e0',
      tags: ['voice'],
    }

    setMemos((prev) => [audioMemo, ...prev])
    setRecordingSeconds(0)

    try {
      await api.saveMemo(audioMemo, {
        workspacePath: effectiveWs,
        storage: memoStorageLocation,
      })
    } catch {}
  }

  const handleToggleRecord = async () => {
    if (isRecording) {
      await stopAndSaveRecording()
    } else {
      try {
        const stream = await navigator.mediaDevices.getUserMedia({ audio: true })
        mediaStreamRef.current = stream
        const mimeType = getSupportedAudioMimeType()
        const recorder = mimeType ? new MediaRecorder(stream, { mimeType }) : new MediaRecorder(stream)
        mediaRecorderRef.current = recorder
        audioChunksRef.current = []
        transcriptRef.current = ''
        recordingStartTimeRef.current = Date.now()
        setRecordingSeconds(0)

        recorder.ondataavailable = (e) => {
          if (e.data && e.data.size > 0) {
            audioChunksRef.current.push(e.data)
          }
        }

        // Initialize Web Speech API concurrently
        const SpeechRec = (window as any).SpeechRecognition || (window as any).webkitSpeechRecognition
        if (SpeechRec) {
          try {
            const recognition = new SpeechRec()
            recognition.continuous = true
            recognition.interimResults = true
            recognition.lang = navigator.language || 'en-US'
            recognition.onresult = (event: any) => {
              let str = ''
              for (let i = 0; i < event.results.length; ++i) {
                if (event.results[i] && event.results[i][0]) {
                  str += event.results[i][0].transcript + ' '
                }
              }
              transcriptRef.current = str.trim()
            }
            recognition.onerror = (e: any) => {
              console.warn('SpeechRecognition error:', e?.error || e)
            }
            recognition.onend = () => {
              speechRecognitionRef.current = null
            }
            recognition.start()
            speechRecognitionRef.current = recognition
          } catch (recErr) {
            console.warn('Speech recognition init failed:', recErr)
            speechRecognitionRef.current = null
          }
        } else {
          speechRecognitionRef.current = null
        }

        recorder.start(250)
        setIsRecording(true)

        recordIntervalRef.current = setInterval(() => {
          setRecordingSeconds((prev) => {
            if (prev >= 60) {
              stopAndSaveRecording()
              return prev
            }
            return prev + 1
          })
        }, 1000)
      } catch (err: any) {
        console.error('Microphone access denied or error:', err)
        setSentFeedback('Microphone access error: ' + (err?.message || 'Access denied'))
        setTimeout(() => setSentFeedback(''), 4000)
      }
    }
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '20px' }}>


      {sentFeedback && (
        <div
          style={{
            padding: '12px 16px',
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
          <CheckCircle2 size={16} />
          <span>{sentFeedback}</span>
        </div>
      )}

      {/* ========================================================================= */}
      {/* TAB 0: Preview Browser in Auxiliary Panel                                  */}
      {/* ========================================================================= */}
      {activeTab === 0 && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
          {/* Controls Bar */}
          <div className="google-card" style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '12px', flexWrap: 'wrap' }}>
            {/* Address Bar */}
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px', flex: 1, minWidth: '320px' }}>
              <button className="btn-pill-tonal" style={{ padding: '6px' }} title="Reload Preview">
                <RotateCw size={14} />
              </button>
              <input
                type="text"
                value={previewUrl}
                onChange={(e) => setPreviewUrl(e.target.value)}
                style={{
                  flex: 1,
                  fontSize: '12px',
                  padding: '7px 12px',
                  fontFamily: 'monospace',
                  borderRadius: '20px',
                  border: '1px solid var(--border)',
                  backgroundColor: 'var(--canvas)',
                }}
                placeholder="http://localhost:5173 or target web URL"
              />
              <div style={{ display: 'flex', gap: '4px' }}>
                {['5173', '3000', '8080'].map((port) => (
                  <button
                    key={port}
                    onClick={() => setPreviewUrl(`http://localhost:${port}`)}
                    className="btn-pill-tonal"
                    style={{ padding: '3px 8px', fontSize: '10px', fontWeight: 600 }}
                  >
                    :{port}
                  </button>
                ))}
              </div>
            </div>

            {/* Annotation Tools */}
            <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
              <span style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', textTransform: 'uppercase', marginRight: '4px' }}>
                Tools:
              </span>
              <button
                onClick={() => setActiveAnnotateTool(activeAnnotateTool === 'pen' ? 'none' : 'pen')}
                style={{
                  padding: '6px 12px',
                  borderRadius: '16px',
                  fontSize: '11px',
                  fontWeight: 600,
                  border: `1px solid ${activeAnnotateTool === 'pen' ? annotationColor : 'var(--border)'}`,
                  backgroundColor: activeAnnotateTool === 'pen' ? '#fce8e6' : 'var(--canvas)',
                  color: activeAnnotateTool === 'pen' ? annotationColor : 'var(--text)',
                  cursor: 'pointer',
                  display: 'inline-flex',
                  alignItems: 'center',
                  gap: '5px',
                }}
              >
                <Edit3 size={13} color={activeAnnotateTool === 'pen' ? annotationColor : 'var(--text)'} />
                <span>Red Pen</span>
              </button>

              <button
                onClick={() => setActiveAnnotateTool(activeAnnotateTool === 'rect' ? 'none' : 'rect')}
                style={{
                  padding: '6px 12px',
                  borderRadius: '16px',
                  fontSize: '11px',
                  fontWeight: 600,
                  border: `1px solid ${activeAnnotateTool === 'rect' ? annotationColor : 'var(--border)'}`,
                  backgroundColor: activeAnnotateTool === 'rect' ? '#fce8e6' : 'var(--canvas)',
                  color: activeAnnotateTool === 'rect' ? annotationColor : 'var(--text)',
                  cursor: 'pointer',
                  display: 'inline-flex',
                  alignItems: 'center',
                  gap: '5px',
                }}
              >
                <Square size={13} color={activeAnnotateTool === 'rect' ? annotationColor : 'var(--text)'} />
                <span>Bounding Box</span>
              </button>

              <button
                onClick={() => setActiveAnnotateTool(activeAnnotateTool === 'inspect' ? 'none' : 'inspect')}
                style={{
                  padding: '6px 12px',
                  borderRadius: '16px',
                  fontSize: '11px',
                  fontWeight: 600,
                  border: `1px solid ${activeAnnotateTool === 'inspect' ? 'var(--primary)' : 'var(--border)'}`,
                  backgroundColor: activeAnnotateTool === 'inspect' ? 'rgba(26, 115, 232, 0.1)' : 'var(--canvas)',
                  color: activeAnnotateTool === 'inspect' ? 'var(--primary)' : 'var(--text)',
                  cursor: 'pointer',
                  display: 'inline-flex',
                  alignItems: 'center',
                  gap: '5px',
                }}
              >
                <Crosshair size={13} color={activeAnnotateTool === 'inspect' ? 'var(--primary)' : 'var(--text)'} />
                <span>Inspect Element</span>
              </button>

              <button
                onClick={() => setShowIpadModal(true)}
                className="btn-pill-tonal"
                style={{ padding: '6px 12px', fontSize: '11px', display: 'inline-flex', alignItems: 'center', gap: '5px' }}
                title="Connect iPad for Apple Pencil annotation"
              >
                <Tablet size={13} />
                <span>iPad Mirror</span>
              </button>
            </div>
          </div>

          {/* Main Preview Container with Simulated Live App Canvas */}
          <div
            style={{
              display: 'grid',
              gridTemplateColumns: '1fr 340px',
              gap: '16px',
              minHeight: '480px',
            }}
          >
            {/* Viewport Canvas */}
            <div
              className="google-card"
              style={{
                padding: '0px',
                overflow: 'hidden',
                position: 'relative',
                display: 'flex',
                flexDirection: 'column',
                backgroundColor: '#ffffff',
                border: '1.5px solid var(--border)',
              }}
            >
              {/* Browser Window Bar */}
              <div
                style={{
                  height: '36px',
                  backgroundColor: '#f1f3f4',
                  borderBottom: '1px solid var(--border)',
                  display: 'flex',
                  alignItems: 'center',
                  padding: '0 12px',
                  gap: '8px',
                }}
              >
                <div style={{ display: 'flex', gap: '6px' }}>
                  <div style={{ width: '10px', height: '10px', borderRadius: '50%', backgroundColor: '#ea4335' }} />
                  <div style={{ width: '10px', height: '10px', borderRadius: '50%', backgroundColor: '#fbbc05' }} />
                  <div style={{ width: '10px', height: '10px', borderRadius: '50%', backgroundColor: '#34a853' }} />
                </div>
                <div style={{ fontSize: '11px', color: 'var(--text-muted)', fontFamily: 'monospace', marginLeft: '8px' }}>
                  {previewUrl}
                </div>
                <div style={{ marginLeft: 'auto', display: 'flex', alignItems: 'center', gap: '6px' }}>
                  {activeAnnotateTool !== 'none' && (
                    <span style={{ fontSize: '10px', fontWeight: 700, color: annotationColor, textTransform: 'uppercase', display: 'flex', alignItems: 'center', gap: '5px' }}>
                      <span style={{ width: '6px', height: '6px', borderRadius: '50%', backgroundColor: annotationColor, display: 'inline-block' }} />
                      <span>Annotation Mode: {activeAnnotateTool}</span>
                    </span>
                  )}
                </div>
              </div>

              {/* Rendered Live App Simulation with Active Visual Annotations */}
              <div
                style={{
                  flex: 1,
                  padding: '32px',
                  backgroundColor: '#fafafa',
                  position: 'relative',
                  overflow: 'hidden',
                  display: 'flex',
                  flexDirection: 'column',
                  gap: '20px',
                  userSelect: 'none',
                }}
              >
                {/* Simulated Web Application Header */}
                <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', borderBottom: '1px solid #e0e0e0', paddingBottom: '16px' }}>
                  <div style={{ fontSize: '18px', fontWeight: 800, color: '#1a73e8' }}>Antigravity Cloud Demo</div>
                  <div style={{ display: 'flex', gap: '16px', fontSize: '13px', color: '#5f6368', fontWeight: 500 }}>
                    <span>Features</span>
                    <span>Documentation</span>
                    <span>Pricing</span>
                  </div>
                </div>

                {/* Hero Section with Red Bounding Box Highlighter */}
                <div
                  style={{
                    position: 'relative',
                    padding: '24px',
                    borderRadius: '12px',
                    backgroundColor: '#ffffff',
                    border: '1px solid #e0e0e0',
                  }}
                >
                  {/* Visual Annotation Overlay Box */}
                  <div
                    style={{
                      position: 'absolute',
                      top: '12px',
                      right: '12px',
                      padding: '16px 20px',
                      border: '2px dashed #ea4335',
                      backgroundColor: 'rgba(234, 67, 53, 0.08)',
                      borderRadius: '8px',
                      pointerEvents: 'none',
                    }}
                  >
                    <span
                      style={{
                        position: 'absolute',
                        top: '-10px',
                        left: '8px',
                        backgroundColor: '#ea4335',
                        color: '#ffffff',
                        fontSize: '9px',
                        fontWeight: 700,
                        padding: '1px 6px',
                        borderRadius: '4px',
                      }}
                    >
                      #annotation-1
                    </span>
                  </div>

                  <h1 style={{ fontSize: '24px', fontWeight: 700, color: '#202124', margin: '0 0 10px' }}>
                    Agentic AI Development Made Seamless
                  </h1>
                  <p style={{ fontSize: '13px', color: '#5f6368', lineHeight: 1.6, maxWidth: '440px', margin: '0 0 16px' }}>
                    Antigravity 2.0 shifts software engineering from manual autocomplete to autonomous coworker agent teams.
                  </p>

                  <button
                    onClick={() => {
                      setCapturedSnippet('<button id="cta-action" class="btn-primary">Get Started</button>')
                      setCommentText('Button text should be 14px bold and use gradient primary background.')
                    }}
                    style={{
                      padding: '10px 24px',
                      borderRadius: '8px',
                      backgroundColor: '#1a73e8',
                      color: '#ffffff',
                      fontWeight: 600,
                      fontSize: '13px',
                      border: '2px solid #ea4335', // Highlighted
                      boxShadow: '0 0 0 4px rgba(234, 67, 53, 0.2)',
                      cursor: 'pointer',
                    }}
                  >
                    Get Started (Inspected)
                  </button>
                </div>

                {/* Freehand Pen Drawing Simulation */}
                <svg
                  style={{
                    position: 'absolute',
                    inset: 0,
                    width: '100%',
                    height: '100%',
                    pointerEvents: 'none',
                  }}
                >
                  <path
                    d="M 280 230 Q 320 280 420 240 T 470 210"
                    fill="none"
                    stroke="#ea4335"
                    strokeWidth="3.5"
                    strokeLinecap="round"
                  />
                  <circle cx="470" cy="210" r="4" fill="#ea4335" />
                </svg>
              </div>
            </div>

            {/* Annotation & Chat Dispatch Drawer */}
            <div className="google-card" style={{ display: 'flex', flexDirection: 'column', justifyContent: 'space-between', padding: '20px' }}>
              <div>
                <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', textTransform: 'uppercase', letterSpacing: '0.8px', marginBottom: '12px' }}>
                  Annotation &amp; Chat Dispatch
                </div>

                {/* Payload Type Selector */}
                <div style={{ marginBottom: '14px' }}>
                  <label style={{ display: 'block', fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '6px' }}>
                    PAYLOAD TO SEND TO GEMINI:
                  </label>
                  <div style={{ display: 'flex', gap: '6px' }}>
                    {[
                      { id: 'hybrid', label: 'Hybrid (Both)' },
                      { id: 'screenshot', label: 'Screenshot' },
                      { id: 'code', label: 'DOM Code' },
                    ].map((mode) => (
                      <button
                        key={mode.id}
                        onClick={() => setPayloadType(mode.id as any)}
                        style={{
                          flex: 1,
                          padding: '5px 8px',
                          borderRadius: '6px',
                          fontSize: '11px',
                          fontWeight: payloadType === mode.id ? 700 : 500,
                          backgroundColor: payloadType === mode.id ? 'var(--primary-container)' : 'var(--tonal)',
                          color: payloadType === mode.id ? 'var(--primary)' : 'var(--text-muted)',
                          border: `1px solid ${payloadType === mode.id ? 'var(--primary)' : 'transparent'}`,
                          cursor: 'pointer',
                        }}
                      >
                        {mode.label}
                      </button>
                    ))}
                  </div>
                  <div style={{ fontSize: '10px', color: 'var(--text-muted)', marginTop: '4px' }}>
                    {payloadType === 'hybrid'
                      ? 'Best for Gemini: attaches high-res bounding box crop + exact HTML snippet & CSS selector.'
                      : payloadType === 'screenshot'
                      ? 'Visual snapshot only with red markup.'
                      : 'Structured DOM tag and CSS hierarchy without images.'}
                  </div>
                </div>

                {/* Inspected Element Code Preview */}
                <div style={{ marginBottom: '14px' }}>
                  <label style={{ display: 'block', fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '4px' }}>
                    TARGET ELEMENT CODE:
                  </label>
                  <textarea
                    rows={3}
                    value={capturedSnippet}
                    onChange={(e) => setCapturedSnippet(e.target.value)}
                    style={{
                      width: '100%',
                      fontSize: '11px',
                      fontFamily: 'monospace',
                      padding: '8px',
                      borderRadius: '6px',
                      border: '1px solid var(--border)',
                      backgroundColor: 'var(--canvas)',
                      boxSizing: 'border-box',
                    }}
                  />
                </div>

                {/* Comment Box */}
                <div>
                  <label style={{ display: 'block', fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)', marginBottom: '4px' }}>
                    COMMENT / INSTRUCTIONS FOR AGENT:
                  </label>
                  <textarea
                    rows={4}
                    placeholder="e.g. Center this button on mobile view and change color to brand primary..."
                    value={commentText}
                    onChange={(e) => setCommentText(e.target.value)}
                    style={{
                      width: '100%',
                      fontSize: '12px',
                      padding: '8px 10px',
                      borderRadius: '6px',
                      border: '1px solid var(--border)',
                      boxSizing: 'border-box',
                    }}
                  />
                </div>
              </div>

              {/* Action Button */}
              <div style={{ marginTop: '16px' }}>
                <button
                  onClick={handleSendToChat}
                  className="btn-pill-primary"
                  style={{
                    width: '100%',
                    padding: '8px 16px',
                    fontSize: '12px',
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    gap: '6px',
                  }}
                >
                  <Send size={14} />
                  <span>Attach &amp; Send to Chat Input</span>
                </button>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* ========================================================================= */}
      {/* TAB 1: Lightweight Auxiliary File Explorer                                */}
      {/* ========================================================================= */}
      {activeTab === 1 && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
          {/* Top Explorer Controls Bar */}
          <div className="google-card" style={{ display: 'flex', flexDirection: 'column', gap: '12px' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '12px', flexWrap: 'wrap' }}>
              {/* Project Folder Dropdown */}
              <div style={{ minWidth: '260px' }}>
                <label style={{ display: 'block', fontSize: '10px', fontWeight: 700, color: 'var(--text-muted)', textTransform: 'uppercase', marginBottom: '2px' }}>
                  Project Workspace:
                </label>
                <select
                  value={currentProjectFolder}
                  onChange={(e) => {
                    setCurrentProjectFolder(e.target.value)
                    handleNavigatePath(e.target.value)
                  }}
                  style={{
                    width: '100%',
                    fontSize: '12px',
                    fontWeight: 600,
                    padding: '6px 10px',
                    borderRadius: '8px',
                    border: '1px solid var(--border)',
                    backgroundColor: 'var(--canvas)',
                  }}
                >
                  {currentProjectFolder !== '.' && currentProjectFolder !== '~/.gemini/antigravity' && (
                    <option value={currentProjectFolder}>{currentProjectFolder}</option>
                  )}
                  <option value=".">Current Project Workspace</option>
                  <option value="~/.gemini/antigravity">Antigravity Runtime State (~/.gemini)</option>
                </select>
              </div>

              {/* Navigation History & Actions */}
              <div style={{ display: 'flex', alignItems: 'flex-end', gap: '6px', flex: 1, minWidth: '280px' }}>
                <div style={{ display: 'flex', gap: '4px' }}>
                  <button
                    onClick={handleBack}
                    disabled={historyIndex <= 0}
                    className="btn-pill-tonal"
                    style={{ padding: '6px 10px' }}
                    title="Back"
                  >
                    <ArrowLeft size={13} />
                  </button>
                  <button
                    onClick={handleForward}
                    disabled={historyIndex >= historyStack.length - 1}
                    className="btn-pill-tonal"
                    style={{ padding: '6px 10px' }}
                    title="Forward"
                  >
                    <ArrowRight size={13} />
                  </button>
                  <button
                    onClick={() => {
                      const parent = addressBarPath.split('/').slice(0, -1).join('/') || '/'
                      handleNavigatePath(parent)
                    }}
                    className="btn-pill-tonal"
                    style={{ padding: '6px 10px' }}
                    title="Up one folder"
                  >
                    <ArrowUp size={13} />
                  </button>
                </div>

                {/* Absolute Address Bar */}
                <input
                  type="text"
                  value={addressBarPath}
                  onChange={(e) => setAddressBarPath(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter') handleNavigatePath(addressBarPath)
                  }}
                  style={{
                    flex: 1,
                    fontSize: '12px',
                    padding: '6px 12px',
                    fontFamily: 'monospace',
                    borderRadius: '8px',
                    border: '1px solid var(--border)',
                    backgroundColor: 'var(--canvas)',
                  }}
                  placeholder="Enter absolute directory path..."
                />

                <button
                  onClick={async () => {
                    try {
                      await api.revealFile(addressBarPath)
                    } catch (e: any) {
                      console.error('Failed to open system file manager:', e)
                    }
                  }}
                  className="btn-pill-tonal"
                  style={{ padding: '6px 12px', fontSize: '11px', display: 'flex', alignItems: 'center', gap: '5px' }}
                  title="Open folder in Linux File Manager (Nautilus/Dolphin)"
                >
                  <Folder size={13} />
                  <span>Open System</span>
                </button>

                <button
                  onClick={async () => {
                    try {
                      await api.openIDE(addressBarPath, preferredIDE)
                    } catch (e: any) {
                      console.error('Failed to open IDE workspace:', e)
                    }
                  }}
                  className="btn-pill-tonal"
                  style={{ padding: '6px 12px', fontSize: '11px', display: 'flex', alignItems: 'center', gap: '5px' }}
                  title={`Open folder in ${getIDEName(preferredIDE)}`}
                >
                  <Code size={13} />
                  <span>Open {getIDEName(preferredIDE)}</span>
                </button>
              </div>
            </div>

            {/* Breadcrumbs & Quick Search Bar */}
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '12px' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '4px', fontSize: '11px', color: 'var(--text-muted)' }}>
                {addressBarPath.split('/').filter(Boolean).map((part, i, arr) => (
                  <React.Fragment key={i}>
                    <span
                      onClick={() => handleNavigatePath('/' + arr.slice(0, i + 1).join('/'))}
                      style={{ cursor: 'pointer', color: i === arr.length - 1 ? 'var(--primary)' : 'inherit', fontWeight: i === arr.length - 1 ? 700 : 500 }}
                    >
                      {part}
                    </span>
                    {i < arr.length - 1 && <ChevronRight size={11} />}
                  </React.Fragment>
                ))}
              </div>

              <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                <select
                  value={preferredIDE}
                  onChange={(e) => {
                    const val = e.target.value
                    setPreferredIDE(val)
                    localStorage.setItem('antigravity_preferred_ide', val)
                    api.setPreferredIDE(val).catch(() => {})
                  }}
                  style={{
                    fontSize: '11px',
                    padding: '4px 8px',
                    borderRadius: '8px',
                    border: '1px solid var(--border)',
                    backgroundColor: 'var(--canvas)',
                    color: 'var(--text)',
                  }}
                  title="Configure Preferred IDE"
                >
                  <option value="code">VS Code (code)</option>
                  <option value="cursor">Cursor (cursor)</option>
                  <option value="windsurf">Windsurf (windsurf)</option>
                  <option value="codium">VSCodium (codium)</option>
                  <option value="zed">Zed (zed)</option>
                </select>

                <div style={{ position: 'relative', width: '200px' }}>
                  <Search size={13} style={{ position: 'absolute', left: '8px', top: '50%', transform: 'translateY(-50%)', color: 'var(--text-muted)' }} />
                  <input
                    type="text"
                    placeholder="Filter files (e.g. *.ts)..."
                    value={searchQuery}
                    onChange={(e) => setSearchQuery(e.target.value)}
                    style={{ width: '100%', fontSize: '11px', padding: '5px 8px 5px 28px', borderRadius: '14px', border: '1px solid var(--border)', boxSizing: 'border-box' }}
                  />
                </div>
              </div>
            </div>
          </div>

          {copiedFileFeedback && (
            <div style={{ padding: '8px 14px', borderRadius: '6px', backgroundColor: 'var(--green-bg)', color: 'var(--green)', fontSize: '11px', fontWeight: 600 }}>
              {copiedFileFeedback}
            </div>
          )}

          {/* Explorer Split View: Tree on Left, In-App Editor on Right */}
          <div style={{ display: 'grid', gridTemplateColumns: '280px 1fr', gap: '16px' }}>
            {/* File List / Tree View */}
            <div
              className="google-card"
              onContextMenu={handleBlankContextMenu}
              onClick={(e) => {
                if (e.target === e.currentTarget) {
                  setSelectedFilePaths([])
                }
              }}
              style={{ padding: '12px', display: 'flex', flexDirection: 'column', gap: '6px', minHeight: '440px', position: 'relative' }}
            >
              <div style={{ fontSize: '10px', fontWeight: 700, color: 'var(--text-muted)', textTransform: 'uppercase', letterSpacing: '0.8px', padding: '4px 8px', display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                <span>Files &amp; Folders</span>
                {selectedFilePaths.length > 1 && (
                  <span style={{ fontSize: '10px', color: 'var(--primary)', fontWeight: 600 }}>
                    {selectedFilePaths.length} selected
                  </span>
                )}
              </div>

              {isFilesLoading && (
                <div style={{ padding: '20px', textAlign: 'center', fontSize: '11px', color: 'var(--text-muted)' }}>
                  Loading files from filesystem...
                </div>
              )}

              {!isFilesLoading && filesList.length === 0 && (
                <div style={{ padding: '20px', textAlign: 'center', fontSize: '11px', color: 'var(--text-muted)' }}>
                  No files found in directory
                </div>
              )}

              {filesList
                .filter((item) => !searchQuery || item.name.toLowerCase().includes(searchQuery.toLowerCase()))
                .map((item) => {
                  const isSelected = selectedFilePaths.includes(item.path) || activeFileViewer?.path === item.path
                  const isCut = fileClipboard.action === 'cut' && fileClipboard.items.some((i) => i.path === item.path)
                  return (
                    <div
                      key={item.path}
                      className="swiss-file-item-row"
                      onClick={(e) => handleRowClick(e, item)}
                      onContextMenu={(e) => handleRowContextMenu(e, item)}
                      draggable={!item.isDir}
                      onDragStart={(e) => {
                        e.dataTransfer.setData('text/plain', `[File: ${item.path}]`)
                      }}
                      style={{
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'space-between',
                        padding: '8px 10px',
                        borderRadius: '8px',
                        fontSize: '12px',
                        cursor: 'pointer',
                        userSelect: 'none',
                        backgroundColor: isSelected ? 'var(--primary-container)' : 'transparent',
                        color: isSelected ? 'var(--primary)' : 'var(--text)',
                        fontWeight: isSelected ? 600 : 500,
                        opacity: isCut ? 0.5 : 1,
                        transition: 'background-color 0.1s ease',
                      }}
                      onMouseEnter={(e) => {
                        if (!isSelected) e.currentTarget.style.backgroundColor = 'var(--tonal)'
                      }}
                      onMouseLeave={(e) => {
                        if (!isSelected) e.currentTarget.style.backgroundColor = 'transparent'
                      }}
                    >
                      <div style={{ display: 'flex', alignItems: 'center', gap: '8px', minWidth: 0 }}>
                        {item.isDir ? (
                          <Folder size={15} color="var(--primary)" style={{ flexShrink: 0 }} />
                        ) : item.type === 'markdown' ? (
                          <FileText size={15} color="#059669" style={{ flexShrink: 0 }} />
                        ) : item.type === 'pdf' ? (
                          <File size={15} color="#dc2626" style={{ flexShrink: 0 }} />
                        ) : item.type === 'office' ? (
                          <FileText size={15} color="#d97706" style={{ flexShrink: 0 }} />
                        ) : (
                          <FileCode size={15} color="#2563eb" style={{ flexShrink: 0 }} />
                        )}
                        <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{item.name}</span>
                      </div>

                      {!item.isDir && item.size && (
                        <span style={{ fontSize: '10px', color: 'var(--text-muted)', flexShrink: 0 }}>{item.size}</span>
                      )}
                    </div>
                  )
                })}

              {/* Custom Right-Click Context Menu */}
              {contextMenu && (
                <div
                  style={{
                    position: 'fixed',
                    left: `${contextMenu.x}px`,
                    top: `${contextMenu.y}px`,
                    zIndex: 99999,
                    backgroundColor: 'var(--canvas, #ffffff)',
                    border: '1px solid var(--border, #cbd5e1)',
                    borderRadius: '8px',
                    boxShadow: '0 4px 18px rgba(0,0,0,0.12)',
                    padding: '4px 0',
                    minWidth: '180px',
                    fontSize: '12px',
                    userSelect: 'none',
                  }}
                  onClick={(e) => e.stopPropagation()}
                  onContextMenu={(e) => {
                    e.preventDefault()
                    e.stopPropagation()
                  }}
                >
                  {contextMenu.type === 'row' && contextMenu.targetItem ? (
                    <>
                      <div
                        onClick={handleCopySelected}
                        style={{ display: 'flex', alignItems: 'center', gap: '8px', padding: '6px 14px', cursor: 'pointer' }}
                        onMouseEnter={(e) => (e.currentTarget.style.backgroundColor = 'var(--tonal)')}
                        onMouseLeave={(e) => (e.currentTarget.style.backgroundColor = 'transparent')}
                      >
                        <Copy size={13} />
                        <span>Copy</span>
                      </div>
                      <div
                        onClick={handleCutSelected}
                        style={{ display: 'flex', alignItems: 'center', gap: '8px', padding: '6px 14px', cursor: 'pointer' }}
                        onMouseEnter={(e) => (e.currentTarget.style.backgroundColor = 'var(--tonal)')}
                        onMouseLeave={(e) => (e.currentTarget.style.backgroundColor = 'transparent')}
                      >
                        <Scissors size={13} />
                        <span>Cut</span>
                      </div>
                      {contextMenu.targetItem.isDir && (
                        <div
                          onClick={() => handlePaste(contextMenu.targetItem?.path)}
                          style={{
                            display: 'flex',
                            alignItems: 'center',
                            gap: '8px',
                            padding: '6px 14px',
                            cursor: fileClipboard.items.length > 0 ? 'pointer' : 'default',
                            opacity: fileClipboard.items.length > 0 ? 1 : 0.4,
                          }}
                          onMouseEnter={(e) => {
                            if (fileClipboard.items.length > 0) e.currentTarget.style.backgroundColor = 'var(--tonal)'
                          }}
                          onMouseLeave={(e) => (e.currentTarget.style.backgroundColor = 'transparent')}
                        >
                          <Clipboard size={13} />
                          <span>Paste into Folder</span>
                        </div>
                      )}
                      <div style={{ height: '1px', backgroundColor: 'var(--border-subtle)', margin: '4px 0' }} />
                      <div
                        onClick={() => {
                          const targets = filesList.filter((f) => selectedFilePaths.includes(f.path))
                          const text = (targets.length > 0 ? targets : [contextMenu.targetItem!]).map((t) => t.path).join('\n')
                          navigator.clipboard.writeText(text)
                          setCopiedFileFeedback('Path copied to clipboard')
                          setTimeout(() => setCopiedFileFeedback(null), 3000)
                          setContextMenu(null)
                        }}
                        style={{ display: 'flex', alignItems: 'center', gap: '8px', padding: '6px 14px', cursor: 'pointer' }}
                        onMouseEnter={(e) => (e.currentTarget.style.backgroundColor = 'var(--tonal)')}
                        onMouseLeave={(e) => (e.currentTarget.style.backgroundColor = 'transparent')}
                      >
                        <Copy size={13} />
                        <span>Copy Path</span>
                      </div>
                      {selectedFilePaths.length <= 1 && (
                        <div
                          onClick={handleRenameItem}
                          style={{ display: 'flex', alignItems: 'center', gap: '8px', padding: '6px 14px', cursor: 'pointer' }}
                          onMouseEnter={(e) => (e.currentTarget.style.backgroundColor = 'var(--tonal)')}
                          onMouseLeave={(e) => (e.currentTarget.style.backgroundColor = 'transparent')}
                        >
                          <Edit3 size={13} />
                          <span>Rename</span>
                        </div>
                      )}
                      <div
                        onClick={handleDeleteSelected}
                        style={{ display: 'flex', alignItems: 'center', gap: '8px', padding: '6px 14px', cursor: 'pointer', color: '#ef4444' }}
                        onMouseEnter={(e) => (e.currentTarget.style.backgroundColor = 'rgba(239,68,68,0.08)')}
                        onMouseLeave={(e) => (e.currentTarget.style.backgroundColor = 'transparent')}
                      >
                        <Trash2 size={13} />
                        <span>Delete</span>
                      </div>
                      <div style={{ height: '1px', backgroundColor: 'var(--border-subtle)', margin: '4px 0' }} />
                      <div
                        onClick={async () => {
                          await api.revealFile(contextMenu.targetItem!.path)
                          setContextMenu(null)
                        }}
                        style={{ display: 'flex', alignItems: 'center', gap: '8px', padding: '6px 14px', cursor: 'pointer' }}
                        onMouseEnter={(e) => (e.currentTarget.style.backgroundColor = 'var(--tonal)')}
                        onMouseLeave={(e) => (e.currentTarget.style.backgroundColor = 'transparent')}
                      >
                        <Folder size={13} />
                        <span>Reveal in File Manager</span>
                      </div>
                      <div
                        onClick={async () => {
                          await api.openTerminal(contextMenu.targetItem!.path)
                          setContextMenu(null)
                        }}
                        style={{ display: 'flex', alignItems: 'center', gap: '8px', padding: '6px 14px', cursor: 'pointer' }}
                        onMouseEnter={(e) => (e.currentTarget.style.backgroundColor = 'var(--tonal)')}
                        onMouseLeave={(e) => (e.currentTarget.style.backgroundColor = 'transparent')}
                      >
                        <Terminal size={13} />
                        <span>Open in Terminal</span>
                      </div>
                      <div
                        onClick={async () => {
                          await api.openIDE(contextMenu.targetItem!.path, preferredIDE)
                          setContextMenu(null)
                        }}
                        style={{ display: 'flex', alignItems: 'center', gap: '8px', padding: '6px 14px', cursor: 'pointer' }}
                        onMouseEnter={(e) => (e.currentTarget.style.backgroundColor = 'var(--tonal)')}
                        onMouseLeave={(e) => (e.currentTarget.style.backgroundColor = 'transparent')}
                      >
                        <Code size={13} />
                        <span>Open in {getIDEName(preferredIDE)}</span>
                      </div>
                    </>
                  ) : (
                    <>
                      <div
                        onClick={() => handlePaste(addressBarPath)}
                        style={{
                          display: 'flex',
                          alignItems: 'center',
                          gap: '8px',
                          padding: '6px 14px',
                          cursor: fileClipboard.items.length > 0 ? 'pointer' : 'default',
                          opacity: fileClipboard.items.length > 0 ? 1 : 0.4,
                        }}
                        onMouseEnter={(e) => {
                          if (fileClipboard.items.length > 0) e.currentTarget.style.backgroundColor = 'var(--tonal)'
                        }}
                        onMouseLeave={(e) => (e.currentTarget.style.backgroundColor = 'transparent')}
                      >
                        <Clipboard size={13} />
                        <span>Paste</span>
                      </div>
                      <div style={{ height: '1px', backgroundColor: 'var(--border-subtle)', margin: '4px 0' }} />
                      <div
                        onClick={async () => {
                          setContextMenu(null)
                          const name = window.prompt('Enter new file name:')
                          if (!name) return
                          await api.createFile(`${addressBarPath}/${name}`, false)
                          loadFiles(addressBarPath)
                        }}
                        style={{ display: 'flex', alignItems: 'center', gap: '8px', padding: '6px 14px', cursor: 'pointer' }}
                        onMouseEnter={(e) => (e.currentTarget.style.backgroundColor = 'var(--tonal)')}
                        onMouseLeave={(e) => (e.currentTarget.style.backgroundColor = 'transparent')}
                      >
                        <Plus size={13} />
                        <span>New File</span>
                      </div>
                      <div
                        onClick={async () => {
                          setContextMenu(null)
                          const name = window.prompt('Enter new folder name:')
                          if (!name) return
                          await api.createFile(`${addressBarPath}/${name}`, true)
                          loadFiles(addressBarPath)
                        }}
                        style={{ display: 'flex', alignItems: 'center', gap: '8px', padding: '6px 14px', cursor: 'pointer' }}
                        onMouseEnter={(e) => (e.currentTarget.style.backgroundColor = 'var(--tonal)')}
                        onMouseLeave={(e) => (e.currentTarget.style.backgroundColor = 'transparent')}
                      >
                        <Folder size={13} />
                        <span>New Folder</span>
                      </div>
                      <div style={{ height: '1px', backgroundColor: 'var(--border-subtle)', margin: '4px 0' }} />
                      <div
                        onClick={() => {
                          loadFiles(addressBarPath)
                          setContextMenu(null)
                        }}
                        style={{ display: 'flex', alignItems: 'center', gap: '8px', padding: '6px 14px', cursor: 'pointer' }}
                        onMouseEnter={(e) => (e.currentTarget.style.backgroundColor = 'var(--tonal)')}
                        onMouseLeave={(e) => (e.currentTarget.style.backgroundColor = 'transparent')}
                      >
                        <RotateCw size={13} />
                        <span>Refresh</span>
                      </div>
                      <div
                        onClick={async () => {
                          await api.revealFile(addressBarPath)
                          setContextMenu(null)
                        }}
                        style={{ display: 'flex', alignItems: 'center', gap: '8px', padding: '6px 14px', cursor: 'pointer' }}
                        onMouseEnter={(e) => (e.currentTarget.style.backgroundColor = 'var(--tonal)')}
                        onMouseLeave={(e) => (e.currentTarget.style.backgroundColor = 'transparent')}
                      >
                        <Folder size={13} />
                        <span>Reveal in File Manager</span>
                      </div>
                      <div
                        onClick={async () => {
                          await api.openTerminal(addressBarPath)
                          setContextMenu(null)
                        }}
                        style={{ display: 'flex', alignItems: 'center', gap: '8px', padding: '6px 14px', cursor: 'pointer' }}
                        onMouseEnter={(e) => (e.currentTarget.style.backgroundColor = 'var(--tonal)')}
                        onMouseLeave={(e) => (e.currentTarget.style.backgroundColor = 'transparent')}
                      >
                        <Terminal size={13} />
                        <span>Open in Terminal</span>
                      </div>
                      <div
                        onClick={async () => {
                          await api.openIDE(addressBarPath, preferredIDE)
                          setContextMenu(null)
                        }}
                        style={{ display: 'flex', alignItems: 'center', gap: '8px', padding: '6px 14px', cursor: 'pointer' }}
                        onMouseEnter={(e) => (e.currentTarget.style.backgroundColor = 'var(--tonal)')}
                        onMouseLeave={(e) => (e.currentTarget.style.backgroundColor = 'transparent')}
                      >
                        <Code size={13} />
                        <span>Open in {getIDEName(preferredIDE)}</span>
                      </div>
                    </>
                  )}
                </div>
              )}

              <div style={{ marginTop: 'auto', borderTop: '1px solid var(--border-subtle)', paddingTop: '10px', display: 'flex', gap: '6px' }}>
                <button
                  onClick={async () => {
                    await api.openTerminal(addressBarPath)
                  }}
                  className="btn-pill-tonal"
                  style={{ flex: 1, padding: '6px 8px', fontSize: '11px', display: 'flex', alignItems: 'center', justifyContent: 'center', gap: '5px' }}
                >
                  <Terminal size={12} />
                  <span>Terminal</span>
                </button>
                <button
                  onClick={async () => {
                    await api.openIDE(addressBarPath, preferredIDE)
                  }}
                  className="btn-pill-tonal"
                  style={{ padding: '6px 8px', fontSize: '11px', display: 'flex', alignItems: 'center', justifyContent: 'center', gap: '5px' }}
                  title={`Open in ${getIDEName(preferredIDE)}`}
                >
                  <Code size={12} />
                  <span>IDE</span>
                </button>
                <button
                  onClick={async () => {
                    await api.revealFile(addressBarPath)
                  }}
                  className="btn-pill-tonal"
                  style={{ padding: '6px 8px', fontSize: '11px', display: 'flex', alignItems: 'center', justifyContent: 'center', gap: '5px' }}
                  title="Open in System File Manager"
                >
                  <Folder size={12} />
                  <span>Reveal</span>
                </button>
                <button
                  onClick={async () => {
                    const name = prompt('Enter new file name:')
                    if (name) {
                      await api.createFile(`${addressBarPath}/${name}`, false)
                      loadFiles(addressBarPath)
                    }
                  }}
                  className="btn-pill-tonal"
                  style={{ padding: '6px 10px', fontSize: '11px' }}
                  title="New File"
                >
                  <Plus size={13} />
                </button>
              </div>
            </div>

            {/* In-App Viewer / Lightweight Editor */}
            {!activeFileViewer ? (
              <div
                className="google-card"
                style={{
                  minHeight: '440px',
                  display: 'flex',
                  flexDirection: 'column',
                  alignItems: 'center',
                  justifyContent: 'center',
                  padding: '24px',
                  textAlign: 'center',
                  color: 'var(--text-muted)',
                }}
              >
                <FileCode size={36} color="var(--primary)" style={{ opacity: 0.4, marginBottom: '10px' }} />
                <div style={{ fontSize: '14px', fontWeight: 600, color: 'var(--text)' }}>No File Selected</div>
                <div style={{ fontSize: '12px', marginTop: '4px', maxWidth: '340px' }}>
                  Click any file in the workspace directory tree on the left to inspect, edit in the lightweight editor, or annotate to Antigravity chat.
                </div>
              </div>
            ) : (
              <div className="google-card" style={{ padding: '0px', overflow: 'hidden', display: 'flex', flexDirection: 'column' }}>
                {/* Viewer Ribbon */}
                <div
                  style={{
                    height: '40px',
                    backgroundColor: '#f8fafc',
                    borderBottom: '1px solid var(--border)',
                    padding: '0 16px',
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'space-between',
                  }}
                >
                  <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                    <span style={{ fontSize: '12px', fontWeight: 700, color: 'var(--text)' }}>
                      {activeFileViewer.name}
                    </span>
                    <span style={{ fontSize: '10px', color: 'var(--text-muted)', fontFamily: 'monospace' }}>
                      {activeFileViewer.path}
                    </span>
                  </div>

                  {activeFileViewer.type === 'code' && (
                    <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                      <div style={{ display: 'flex', backgroundColor: 'var(--tonal)', borderRadius: '8px', padding: '2px' }}>
                        <button
                          onClick={() => setIsCodeEditingMode(false)}
                          style={{
                            border: 'none',
                            padding: '3px 10px',
                            borderRadius: '6px',
                            fontSize: '11px',
                            fontWeight: !isCodeEditingMode ? 700 : 500,
                            backgroundColor: !isCodeEditingMode ? '#ffffff' : 'transparent',
                            color: !isCodeEditingMode ? 'var(--primary)' : 'var(--text-muted)',
                            cursor: 'pointer',
                          }}
                        >
                          Annotate
                        </button>
                        <button
                          onClick={() => setIsCodeEditingMode(true)}
                          style={{
                            border: 'none',
                            padding: '3px 10px',
                            borderRadius: '6px',
                            fontSize: '11px',
                            fontWeight: isCodeEditingMode ? 700 : 500,
                            backgroundColor: isCodeEditingMode ? '#ffffff' : 'transparent',
                            color: isCodeEditingMode ? 'var(--primary)' : 'var(--text-muted)',
                            cursor: 'pointer',
                          }}
                        >
                          Edit
                        </button>
                      </div>

                      <button
                        onClick={handleSaveFileContent}
                        disabled={isSavingFile}
                        className="btn-pill-tonal"
                        style={{ padding: '4px 10px', fontSize: '11px', display: 'flex', alignItems: 'center', gap: '4px' }}
                        title="Save changes to disk"
                      >
                        <Save size={11} />
                        <span>{isSavingFile ? 'Saving...' : 'Save'}</span>
                      </button>

                      <button
                        onClick={handleSendCodeAnnotationToChat}
                        className="btn-pill-primary"
                        style={{ padding: '4px 12px', fontSize: '11px', display: 'flex', alignItems: 'center', gap: '5px' }}
                      >
                        <Send size={11} />
                        <span>Annotate to Chat</span>
                      </button>

                      {fileSaveFeedback && (
                        <span style={{ fontSize: '11px', color: 'var(--green)', fontWeight: 600 }}>
                          {fileSaveFeedback}
                        </span>
                      )}
                    </div>
                  )}

                  {activeFileViewer.type === 'markdown' && (
                    <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                      <div style={{ display: 'flex', backgroundColor: 'var(--tonal)', borderRadius: '8px', padding: '2px' }}>
                        <button
                          onClick={() => setMarkdownViewMode('edit')}
                          style={{
                            border: 'none',
                            padding: '3px 10px',
                            borderRadius: '6px',
                            fontSize: '11px',
                            fontWeight: markdownViewMode === 'edit' ? 700 : 500,
                            backgroundColor: markdownViewMode === 'edit' ? '#ffffff' : 'transparent',
                            color: markdownViewMode === 'edit' ? 'var(--primary)' : 'var(--text-muted)',
                            cursor: 'pointer',
                          }}
                        >
                          Edit Source
                        </button>
                        <button
                          onClick={() => setMarkdownViewMode('preview')}
                          style={{
                            border: 'none',
                            padding: '3px 10px',
                            borderRadius: '6px',
                            fontSize: '11px',
                            fontWeight: markdownViewMode === 'preview' ? 700 : 500,
                            backgroundColor: markdownViewMode === 'preview' ? '#ffffff' : 'transparent',
                            color: markdownViewMode === 'preview' ? 'var(--primary)' : 'var(--text-muted)',
                            cursor: 'pointer',
                          }}
                        >
                          Preview
                        </button>
                      </div>

                      <button
                        onClick={handleSaveFileContent}
                        disabled={isSavingFile}
                        className="btn-pill-primary"
                        style={{ padding: '4px 12px', fontSize: '11px', display: 'flex', alignItems: 'center', gap: '4px' }}
                        title="Save changes to disk"
                      >
                        <Save size={11} />
                        <span>{isSavingFile ? 'Saving...' : 'Save File'}</span>
                      </button>

                      {fileSaveFeedback && (
                        <span style={{ fontSize: '11px', color: 'var(--green)', fontWeight: 600 }}>
                          {fileSaveFeedback}
                        </span>
                      )}
                    </div>
                  )}

                  {activeFileViewer.type === 'pdf' && (
                    <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                      <button
                        onClick={() => setPdfHighlightMode('highlight')}
                        className={pdfHighlightMode === 'highlight' ? 'btn-pill-primary' : 'btn-pill-tonal'}
                        style={{ padding: '3px 8px', fontSize: '10px', display: 'flex', alignItems: 'center', gap: '4px' }}
                      >
                        <Highlighter size={11} />
                        <span>Highlight</span>
                      </button>
                      <button
                        onClick={() => setPdfHighlightMode('underline')}
                        className={pdfHighlightMode === 'underline' ? 'btn-pill-primary' : 'btn-pill-tonal'}
                        style={{ padding: '3px 8px', fontSize: '10px', display: 'flex', alignItems: 'center', gap: '4px' }}
                      >
                        <Underline size={11} />
                        <span>Underline</span>
                      </button>
                      <button
                        onClick={() => {
                          const note = `[PDF Annotation: ${activeFileViewer.name}]\nHighlighted excerpt from real document.\nComment: "Verify in Antigravity chat."`
                          navigator.clipboard.writeText(note)
                          setCopiedFileFeedback('PDF excerpt & annotation attached to Antigravity chat!')
                          setTimeout(() => setCopiedFileFeedback(null), 3000)
                        }}
                        className="btn-pill-tonal"
                        style={{ padding: '3px 8px', fontSize: '10px' }}
                      >
                        Send Mark to Chat
                      </button>
                    </div>
                  )}
                </div>

                {/* Viewer Content Body */}
                <div style={{ flex: 1, padding: '16px', overflowY: 'auto' }}>
                  {activeFileViewer.type === 'markdown' && (
                    <div style={{ height: '100%', display: 'flex', flexDirection: 'column' }}>
                      {markdownViewMode === 'edit' ? (
                        <textarea
                          value={fileEditorContent}
                          onChange={(e) => setFileEditorContent(e.target.value)}
                          placeholder="Type markdown content..."
                          style={{
                            width: '100%',
                            flex: 1,
                            minHeight: '380px',
                            fontFamily: 'monospace',
                            fontSize: '12px',
                            lineHeight: 1.6,
                            padding: '12px',
                            borderRadius: '8px',
                            border: '1px solid var(--border)',
                            backgroundColor: 'var(--canvas)',
                            color: 'var(--text)',
                            boxSizing: 'border-box',
                            resize: 'vertical',
                          }}
                        />
                      ) : (
                        <div style={{ fontSize: '13px', lineHeight: 1.6, color: 'var(--text)', padding: '8px' }}>
                          <pre
                            style={{
                              whiteSpace: 'pre-wrap',
                              fontFamily: 'inherit',
                              margin: 0,
                              backgroundColor: 'transparent',
                              color: 'var(--text)',
                            }}
                          >
                            {fileEditorContent || 'Empty markdown document.'}
                          </pre>
                        </div>
                      )}
                    </div>
                  )}

                  {activeFileViewer.type === 'code' && (
                    <div style={{ height: '100%', display: 'flex', flexDirection: 'column' }}>
                      {isCodeEditingMode ? (
                        <textarea
                          value={fileEditorContent}
                          onChange={(e) => setFileEditorContent(e.target.value)}
                          placeholder="Edit code..."
                          style={{
                            width: '100%',
                            flex: 1,
                            minHeight: '380px',
                            fontFamily: 'monospace',
                            fontSize: '12px',
                            lineHeight: 1.6,
                            padding: '12px',
                            borderRadius: '8px',
                            border: '1px solid var(--border)',
                            backgroundColor: 'var(--canvas)',
                            color: 'var(--text)',
                            boxSizing: 'border-box',
                            resize: 'vertical',
                          }}
                        />
                      ) : (
                        <div style={{ fontFamily: 'monospace', fontSize: '12px', lineHeight: 1.6 }}>
                          {(fileEditorContent || 'Empty code file.')
                            .split('\n')
                            .map((line, i) => (
                              <div
                                key={i}
                                onClick={() => {
                                  setCodeSnippetToAnnotate(line)
                                  setCodeLineRange(`L${i + 1}`)
                                }}
                                style={{
                                  padding: '1px 6px',
                                  cursor: 'pointer',
                                  display: 'flex',
                                  gap: '12px',
                                  borderRadius: '2px',
                                  backgroundColor: codeSnippetToAnnotate === line ? '#fef3c7' : 'transparent',
                                }}
                              >
                                <span style={{ color: 'var(--text-muted)', userSelect: 'none', width: '32px', textAlign: 'right' }}>
                                  {i + 1}
                                </span>
                                <span>{line || ' '}</span>
                              </div>
                            ))}
                        </div>
                      )}
                    </div>
                  )}

                  {activeFileViewer.type === 'pdf' && (
                    <div style={{ textAlign: 'center', padding: '30px' }}>
                      <File size={48} color="#dc2626" style={{ margin: '0 auto 12px' }} />
                      <div style={{ fontSize: '15px', fontWeight: 700 }}>{activeFileViewer.name} Preview</div>
                      <div style={{ fontSize: '12px', color: 'var(--text-muted)', marginTop: '4px' }}>
                        PDF loaded ({activeFileViewer.path}). Select any text block to highlight, underline, or annotate directly into the Antigravity conversation prompt.
                      </div>
                    </div>
                  )}

                  {activeFileViewer.type === 'office' && (
                    <div style={{ textAlign: 'center', padding: '30px' }}>
                      <FileText size={48} color="#d97706" style={{ margin: '0 auto 12px' }} />
                      <div style={{ fontSize: '15px', fontWeight: 700 }}>Univer Spreadsheet Viewer Active</div>
                      <div style={{ fontSize: '12px', color: 'var(--text-muted)', marginTop: '4px' }}>
                        Spreadsheet loaded ({activeFileViewer.name}). Drag cell ranges or table views into Antigravity chat.
                      </div>
                    </div>
                  )}
                </div>
              </div>
            )}
          </div>
        </div>
      )}

      {/* ========================================================================= */}
      {/* TAB 2: Quick Memos                                                        */}
      {/* ========================================================================= */}
      {activeTab === 2 && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
          {/* Memo Composer */}
          <div className="google-card" style={{ display: 'flex', flexDirection: 'column', gap: '12px' }}>
            <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', textTransform: 'uppercase', letterSpacing: '0.8px' }}>
              Quick Text &amp; Sound Memo Pad
            </div>
            <p style={{ margin: 0, fontSize: '13px', color: 'var(--text-muted)' }}>
              Capture fleeting ideas, voice recordings, or prompt snippets. Drag any memo card directly into Antigravity's chat input.
            </p>

            <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
              <textarea
                rows={2}
                placeholder="Type a quick memo or thought... (Drag onto Antigravity conversation later)"
                value={newMemoText}
                onChange={(e) => setNewMemoText(e.target.value)}
                style={{
                  width: '100%',
                  fontSize: '12px',
                  padding: '8px 12px',
                  borderRadius: '8px',
                  border: '1px solid var(--border)',
                  boxSizing: 'border-box',
                }}
              />
              <div style={{ display: 'flex', gap: '8px', width: '100%' }}>
                <button
                  onClick={handleAddTextMemo}
                  className="btn-pill-primary"
                  style={{
                    flex: '1 1 0',
                    minWidth: 0,
                    display: 'inline-flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    textAlign: 'center',
                    padding: '8px 14px',
                    fontSize: '12px',
                    whiteSpace: 'nowrap',
                    borderRadius: '8px',
                    gap: '6px',
                    cursor: 'pointer',
                  }}
                >
                  <Plus size={14} />
                  <span>+ Text Memo</span>
                </button>
                <button
                  onClick={handleToggleRecord}
                  style={{
                    flex: '1 1 0',
                    minWidth: 0,
                    display: 'inline-flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    textAlign: 'center',
                    padding: '8px 14px',
                    fontSize: '12px',
                    fontWeight: 600,
                    whiteSpace: 'nowrap',
                    borderRadius: '8px',
                    backgroundColor: isRecording ? '#dc2626' : 'var(--canvas)',
                    color: isRecording ? '#ffffff' : 'var(--text)',
                    border: isRecording ? '1px solid #dc2626' : '1px solid var(--border)',
                    cursor: 'pointer',
                    gap: '6px',
                    transition: 'background 0.15s, border-color 0.15s',
                  }}
                >
                  {isRecording ? <MicOff size={14} /> : <Mic size={14} />}
                  <span>{isRecording ? `Recording... (${recordingSeconds}s)` : 'Voice Memo'}</span>
                </button>
              </div>
            </div>
          </div>

          {/* In-Panel Memo Search Bar */}
          <div
            className="google-card"
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: '8px',
              padding: '8px 12px',
            }}
          >
            <div style={{ position: 'relative', flex: 1, display: 'flex', alignItems: 'center' }}>
              <Search
                size={14}
                style={{
                  position: 'absolute',
                  left: '10px',
                  color: 'var(--text-muted)',
                  pointerEvents: 'none',
                }}
              />
              <input
                type="text"
                placeholder="Search memos..."
                value={memoSearchQuery}
                onChange={(e) => setMemoSearchQuery(e.target.value)}
                style={{
                  width: '100%',
                  padding: '6px 30px 6px 30px',
                  fontSize: '12px',
                  borderRadius: '6px',
                  border: '1px solid var(--border)',
                  backgroundColor: 'transparent',
                  color: 'var(--text)',
                  boxSizing: 'border-box',
                  outline: 'none',
                }}
              />
              {memoSearchQuery && (
                <button
                  type="button"
                  onClick={() => setMemoSearchQuery('')}
                  style={{
                    position: 'absolute',
                    right: '8px',
                    background: 'none',
                    border: 'none',
                    padding: '2px',
                    cursor: 'pointer',
                    color: 'var(--text-muted)',
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                  }}
                  title="Clear search"
                >
                  <X size={13} />
                </button>
              )}
            </div>

            {/* In-Panel View Scope Toggle */}
            <button
              type="button"
              onClick={() => handleToggleViewScope(memoViewScope === 'all' ? 'current' : 'all')}
              style={{
                display: 'inline-flex',
                alignItems: 'center',
                gap: '6px',
                padding: '6px 12px',
                fontSize: '11px',
                fontWeight: 600,
                borderRadius: '6px',
                border: '1px solid var(--border)',
                backgroundColor: memoViewScope === 'current' ? 'var(--primary, #1a73e8)' : 'var(--canvas)',
                color: memoViewScope === 'current' ? '#ffffff' : 'var(--text-muted)',
                cursor: 'pointer',
                whiteSpace: 'nowrap',
                transition: 'all 0.15s ease',
              }}
              title="Toggle memo view scope: All Projects vs Current Project Only"
            >
              <Globe size={13} />
              <span>View: {memoViewScope === 'all' ? 'All' : 'Project'}</span>
            </button>

            {/* Search Scope Toggle */}
            <button
              type="button"
              onClick={() => setMemoSearchScope(memoSearchScope === 'text' ? 'all' : 'text')}
              style={{
                display: 'inline-flex',
                alignItems: 'center',
                gap: '6px',
                padding: '6px 12px',
                fontSize: '11px',
                fontWeight: 600,
                borderRadius: '6px',
                border: '1px solid var(--border)',
                backgroundColor: memoSearchScope === 'all' ? 'var(--primary, #1a73e8)' : 'var(--canvas)',
                color: memoSearchScope === 'all' ? '#ffffff' : 'var(--text-muted)',
                cursor: 'pointer',
                whiteSpace: 'nowrap',
                transition: 'all 0.15s ease',
              }}
              title="Toggle search scope: Text only vs Text + Voice"
            >
              <span>Scope: {memoSearchScope === 'text' ? 'Text' : 'Text+Voice'}</span>
            </button>
          </div>

          {/* Memos Masonry Grid or Empty State */}
          {displayedMemos.length === 0 ? (
            <div
              className="google-card"
              style={{
                textAlign: 'center',
                padding: '32px 16px',
                color: 'var(--text-muted)',
                fontSize: '12px',
              }}
            >
              {memoSearchQuery.trim() ? (
                <>
                  <div style={{ fontWeight: 600, color: 'var(--text)', marginBottom: '4px' }}>
                    No memos found matching "{memoSearchQuery.trim()}"
                  </div>
                  <div style={{ fontSize: '11px' }}>
                    Try searching different keywords or toggle scope to Text+Voice.
                  </div>
                </>
              ) : (
                <div>No memos yet. Click + Text Memo or Voice Memo above to get started.</div>
              )}
            </div>
          ) : (
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(280px, 1fr))', gap: '14px' }}>
              {displayedMemos.map((memo) => (
                <div
                  key={memo.id}
                  draggable
                  onDragStart={(e) => {
                    e.dataTransfer.setData('text/plain', `[Memo]: ${memo.transcript || memo.content}`)
                  }}
                  className="google-card"
                  style={{
                    backgroundColor: memo.color,
                    cursor: 'grab',
                    padding: '16px',
                    display: 'flex',
                    flexDirection: 'column',
                    justifyContent: 'space-between',
                    border: '1px solid var(--border)',
                    minHeight: '110px',
                  }}
                  title="Drag into Antigravity chat input"
                >
                  <div>
                    <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '8px' }}>
                      <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                        {memo.type === 'voice' || memo.type === 'audio' ? (
                          <span style={{ fontSize: '10px', fontWeight: 700, color: '#b06000', display: 'flex', alignItems: 'center', gap: '3px' }}>
                            <Mic size={12} /> Audio ({memo.duration || '0:00'})
                          </span>
                        ) : (
                          <span style={{ fontSize: '10px', fontWeight: 700, color: 'var(--primary)', display: 'flex', alignItems: 'center', gap: '3px' }}>
                            <StickyNote size={12} /> Text Memo
                          </span>
                        )}
                      </div>
                      <span style={{ fontSize: '10px', color: 'var(--text-muted)' }}>{memo.createdAt || memo.created_at}</span>
                    </div>

                    {memo.title && memo.title !== memo.content && (
                      <div style={{ fontWeight: 600, fontSize: '12px', color: 'var(--text)', marginBottom: '4px' }}>
                        {memo.title}
                      </div>
                    )}

                    <p style={{ margin: 0, fontSize: '13px', color: 'var(--text)', lineHeight: 1.5 }}>
                      {memo.content}
                    </p>

                    {(memo.audio_data || memo.audioData) && (
                      <audio
                        controls
                        src={memo.audio_data || memo.audioData}
                        style={{ width: '100%', height: '28px', marginTop: '8px' }}
                      />
                    )}
                  </div>

                  <div style={{ marginTop: '12px', display: 'flex', alignItems: 'center', justifyContent: 'space-between', borderTop: '1px solid rgba(0,0,0,0.06)', paddingTop: '8px' }}>
                    <span style={{ fontSize: '10px', color: 'var(--text-muted)' }}>Drag to Chat</span>
                    <button
                      onClick={() => handleDeleteMemo(memo.id)}
                      style={{ background: 'none', border: 'none', cursor: 'pointer', padding: '2px', color: 'var(--text-muted)' }}
                      title="Delete Memo"
                    >
                      <Trash2 size={13} />
                    </button>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      )}

      {/* ========================================================================= */}
      {/* TAB 3: GitHub Workspace                                                   */}
      {/* ========================================================================= */}
      {activeTab === 3 && (
        <div
          style={{
            display: 'flex',
            flexDirection: 'column',
            backgroundColor: 'var(--card, #ffffff)',
            borderRadius: '10px',
            border: '1px solid var(--border)',
            overflow: 'hidden',
            minHeight: '680px',
            height: 'calc(100vh - 130px)',
          }}
        >
          <GitHubWorkspacePage scope={scope} fallbackProject={fallbackProject} />
        </div>
      )}

      {/* ========================================================================= */}
      {/* TAB 4: Mobile Simulator                                                   */}
      {/* ========================================================================= */}
      {activeTab === 4 && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
          <div className="google-card" style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', flexWrap: 'wrap', gap: '12px' }}>
            <div>
              <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', textTransform: 'uppercase', letterSpacing: '0.8px' }}>
                Mobile Simulator &amp; Viewport Emulation
              </div>
              <div style={{ fontSize: '13px', color: 'var(--text-muted)', marginTop: '2px' }}>
                Emulate mobile device viewports, touch gestures, and orientations (inspired by ZSeven-W/dsh-ios).
              </div>
            </div>

            <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
              <select
                value={selectedDevice}
                onChange={(e) => setSelectedDevice(e.target.value as any)}
                style={{ fontSize: '12px', padding: '6px 12px', borderRadius: '8px', border: '1px solid var(--border)' }}
              >
                <option value="iphone16">iPhone 16 Pro (393 &times; 852)</option>
                <option value="pixel9">Google Pixel 9 (412 &times; 924)</option>
                <option value="ipad">iPad Air (820 &times; 1180)</option>
              </select>

              <button
                onClick={() => setIsLandscape(!isLandscape)}
                className="btn-pill-tonal"
                style={{ padding: '6px 12px', fontSize: '12px' }}
              >
                {isLandscape ? 'Rotate to Portrait' : 'Rotate to Landscape'}
              </button>

              <label style={{ display: 'flex', alignItems: 'center', gap: '6px', fontSize: '12px', cursor: 'pointer' }}>
                <ToggleSwitch size="sm" checked={showBezel} onChange={setShowBezel} />
                <span>Show Hardware Bezel</span>
              </label>
            </div>
          </div>

          {/* Simulator Device Frame */}
          <div style={{ display: 'flex', justifyContent: 'center', padding: '24px 0' }}>
            <div
              style={{
                width: isLandscape ? (selectedDevice === 'ipad' ? '700px' : '580px') : (selectedDevice === 'ipad' ? '480px' : '360px'),
                height: isLandscape ? (selectedDevice === 'ipad' ? '480px' : '320px') : (selectedDevice === 'ipad' ? '680px' : '620px'),
                backgroundColor: '#ffffff',
                borderRadius: showBezel ? '42px' : '12px',
                border: showBezel ? '12px solid #1e293b' : '1px solid var(--border)',
                boxShadow: '0 20px 48px rgba(0,0,0,0.18)',
                overflow: 'hidden',
                display: 'flex',
                flexDirection: 'column',
                position: 'relative',
                transition: 'all 0.3s ease',
              }}
            >
              {/* Dynamic Island / Camera Notch */}
              {showBezel && !isLandscape && (
                <div style={{ display: 'flex', justifyContent: 'center', paddingTop: '10px' }}>
                  <div style={{ width: '90px', height: '24px', backgroundColor: '#000000', borderRadius: '20px' }} />
                </div>
              )}

              {/* Viewport Frame */}
              <div style={{ flex: 1, padding: '16px', overflowY: 'auto', backgroundColor: '#fafafa', display: 'flex', flexDirection: 'column', gap: '14px' }}>
                <div style={{ padding: '16px', borderRadius: '14px', backgroundColor: '#ffffff', boxShadow: '0 1px 3px rgba(0,0,0,0.06)' }}>
                  <div style={{ fontSize: '16px', fontWeight: 700, color: 'var(--text)', marginBottom: '4px' }}>
                    Mobile Responsive Preview
                  </div>
                  <div style={{ fontSize: '12px', color: 'var(--text-muted)' }}>
                    Touch emulation active. Tap elements to capture mobile coordinates.
                  </div>
                </div>

                <div style={{ padding: '14px', borderRadius: '12px', backgroundColor: '#e8f0fe', color: '#1a73e8', fontSize: '12px', fontWeight: 600 }}>
                  Device: {selectedDevice === 'iphone16' ? 'iPhone 16 Pro' : selectedDevice === 'pixel9' ? 'Pixel 9' : 'iPad Air'} ({isLandscape ? 'Landscape' : 'Portrait'})
                </div>

                <div style={{ marginTop: 'auto', textAlign: 'center', fontSize: '11px', color: 'var(--text-muted)' }}>
                  Hardware virtualization powered by Chromium touch emulation engine.
                </div>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* ========================================================================= */}
      {/* TAB 5: Computer Use Enhancer & Evaluation                                 */}
      {/* ========================================================================= */}
      {activeTab === 5 && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
          <div className="google-card" style={{ display: 'flex', flexDirection: 'column', gap: '12px' }}>
            <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', textTransform: 'uppercase', letterSpacing: '0.8px' }}>
              Antigravity Computer Use Evaluation &amp; Optimization
            </div>
            <p style={{ margin: 0, fontSize: '13px', color: 'var(--text-muted)', lineHeight: 1.5 }}>
              Many users report that Antigravity's default computer use suffers from fractional DPI scaling drift (clicking offsets on 4K/125% scaling), excessive token burn from raw screenshots, and Wayland Linux black screens. Our plugin injects coordinate normalizers and hybrid accessibility grounding.
            </p>
          </div>

          {/* Enhancer Toggles */}
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(320px, 1fr))', gap: '14px' }}>
            <div className="google-card" style={{ display: 'flex', flexDirection: 'column', justifyContent: 'space-between' }}>
              <div>
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '8px' }}>
                  <span style={{ fontSize: '14px', fontWeight: 700, color: 'var(--text)' }}>
                    DPI Coordinate Normalizer
                  </span>
                  <ToggleSwitch checked={dpiNormalization} onChange={setDpiNormalization} size="sm" />
                </div>
                <p style={{ margin: 0, fontSize: '12px', color: 'var(--text-muted)', lineHeight: 1.5 }}>
                  Automatically compensates for display scaling factors (125%, 150%, 200%) on HiDPI displays, eliminating false clicks and cursor drift.
                </p>
              </div>
            </div>

            <div className="google-card" style={{ display: 'flex', flexDirection: 'column', justifyContent: 'space-between' }}>
              <div>
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '8px' }}>
                  <span style={{ fontSize: '14px', fontWeight: 700, color: 'var(--text)' }}>
                    Wayland PipeWire Portal Fix
                  </span>
                  <ToggleSwitch checked={waylandPipeWire} onChange={setWaylandPipeWire} size="sm" />
                </div>
                <p style={{ margin: 0, fontSize: '12px', color: 'var(--text-muted)', lineHeight: 1.5 }}>
                  Fixes Linux Wayland black screenshots by capturing via the desktop portal PipeWire stream rather than deprecated X11 root windows.
                </p>
              </div>
            </div>

            <div className="google-card" style={{ display: 'flex', flexDirection: 'column', justifyContent: 'space-between' }}>
              <div>
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '8px' }}>
                  <span style={{ fontSize: '14px', fontWeight: 700, color: 'var(--text)' }}>
                    Hybrid Accessibility Tree Grounding
                  </span>
                  <ToggleSwitch checked={accessibilityGrounding} onChange={setAccessibilityGrounding} size="sm" />
                </div>
                <p style={{ margin: 0, fontSize: '12px', color: 'var(--text-muted)', lineHeight: 1.5 }}>
                  Inspects OS accessibility nodes (AT-SPI on Linux / UI Automation on Windows) before taking screenshots, cutting token consumption by 60%.
                </p>
              </div>
            </div>
          </div>

          {/* Open-Source Comparison Matrix */}
          <div className="google-card" style={{ padding: '0px', overflow: 'hidden' }}>
            <div style={{ padding: '14px 20px', borderBottom: '1px solid var(--border)' }}>
              <div style={{ fontSize: '11px', fontWeight: 700, color: 'var(--text-muted)', textTransform: 'uppercase', letterSpacing: '0.8px' }}>
                Open-Source Alternative Benchmarking
              </div>
            </div>
            <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '12px' }}>
              <thead>
                <tr style={{ backgroundColor: 'var(--canvas)', borderBottom: '1px solid var(--border)' }}>
                  <th style={{ padding: '10px 16px', textAlign: 'left' }}>Engine</th>
                  <th style={{ padding: '10px 16px', textAlign: 'left' }}>Latency</th>
                  <th style={{ padding: '10px 16px', textAlign: 'left' }}>Token Cost / Step</th>
                  <th style={{ padding: '10px 16px', textAlign: 'left' }}>HiDPI / Multi-Monitor</th>
                  <th style={{ padding: '10px 16px', textAlign: 'left' }}>Recommendation</th>
                </tr>
              </thead>
              <tbody>
                <tr style={{ borderBottom: '1px solid var(--border-subtle)' }}>
                  <td style={{ padding: '12px 16px', fontWeight: 600 }}>Default Antigravity 2.0</td>
                  <td style={{ padding: '12px 16px' }}>1200ms</td>
                  <td style={{ padding: '12px 16px', color: '#b3261e' }}>~1600 tokens (raw PNG)</td>
                  <td style={{ padding: '12px 16px', color: '#b3261e' }}>Drifts on 125%/150% scaling</td>
                  <td style={{ padding: '12px 16px' }}>Needs normalizer patch</td>
                </tr>
                <tr style={{ borderBottom: '1px solid var(--border-subtle)' }}>
                  <td style={{ padding: '12px 16px', fontWeight: 600 }}>Open-Computer-Use (MCP)</td>
                  <td style={{ padding: '12px 16px' }}>650ms</td>
                  <td style={{ padding: '12px 16px', color: '#137333' }}>~600 tokens (diff ROI)</td>
                  <td style={{ padding: '12px 16px', color: '#137333' }}>Calibrated via ydotool</td>
                  <td style={{ padding: '12px 16px', fontWeight: 600, color: 'var(--primary)' }}>Integrated via MCP server</td>
                </tr>
                <tr>
                  <td style={{ padding: '12px 16px', fontWeight: 600 }}>OSWorld / Show-UI Hybrid</td>
                  <td style={{ padding: '12px 16px' }}>800ms</td>
                  <td style={{ padding: '12px 16px', color: '#137333' }}>~450 tokens (AT-SPI nodes)</td>
                  <td style={{ padding: '12px 16px', color: '#137333' }}>Exact bounding boxes</td>
                  <td style={{ padding: '12px 16px' }}>Planned for Stage 2 plugin</td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* iPad Live Mirroring Modal */}
      {showIpadModal && (
        <div
          style={{
            position: 'fixed',
            inset: 0,
            backgroundColor: 'rgba(0, 0, 0, 0.5)',
            backdropFilter: 'blur(3px)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            zIndex: 1100,
          }}
          onClick={() => setShowIpadModal(false)}
        >
          <div
            className="google-card"
            style={{ width: '480px', maxWidth: '92vw', padding: '24px', backgroundColor: '#ffffff' }}
            onClick={(e) => e.stopPropagation()}
          >
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '14px' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                <Tablet size={20} color="var(--primary)" />
                <h3 style={{ margin: 0, fontSize: '16px', fontWeight: 700 }}>Connect iPad for Apple Pencil Markup</h3>
              </div>
              <button onClick={() => setShowIpadModal(false)} className="btn-pill-tonal" style={{ padding: '4px' }}>
                <Check size={16} />
              </button>
            </div>
            <p style={{ margin: '0 0 16px', fontSize: '13px', color: 'var(--text-muted)', lineHeight: 1.5 }}>
              Open Safari on your iPad connected to the same Wi-Fi or USB tether, and visit:
            </p>
            <div style={{ backgroundColor: 'var(--canvas)', border: '1px solid var(--border)', borderRadius: '8px', padding: '12px', fontFamily: 'monospace', fontSize: '13px', color: 'var(--primary)', textAlign: 'center', fontWeight: 700, marginBottom: '16px' }}>
              http://192.168.1.105:8765/ipad-preview
            </div>
            <div style={{ fontSize: '12px', color: 'var(--text-muted)', lineHeight: 1.5, marginBottom: '16px' }}>
              Pressure and tilt data from Apple Pencil are streamed over low-latency WebSockets into the Antigravity auxiliary preview browser.
            </div>
            <div style={{ display: 'flex', justifyContent: 'flex-end' }}>
              <button onClick={() => setShowIpadModal(false)} className="btn-pill-primary" style={{ padding: '6px 18px', fontSize: '12px' }}>
                Done
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

export { ExtensionsPage as FeaturePluginsPage }
