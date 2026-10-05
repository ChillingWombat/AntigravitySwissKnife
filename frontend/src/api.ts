import type {
  CacheBreakdown,
  CustomModel,
  CustomModelsConfig,
  DeviceProfile,
  FleetQuotaSummary,
  ProviderPreset,
  QuotaSummary,
  RuleConfig,
  SystemInstallations,
  SystemStatus,
  TestResult,
  EnhancementsConfig,
  ArchivedProjectItem,
  ScheduledTemplate,
  DeployTaskRequest,
  UpdateTaskRequest,
  SidecarTaskInfo,
  GUIConfig,
  ProviderType,
  FetchModelsResponse,
  AutoArchiveResult,
  QuotaResult,
} from './types'

async function request<T>(url: string, options?: RequestInit): Promise<T> {
  const res = await fetch(url, options)
  if (!res.ok) {
    const text = await res.text()
    throw new Error(text || `Request failed with status ${res.status}`)
  }
  return res.json() as Promise<T>
}

export const api = {
  getStatus: () => request<SystemStatus>('/api/status'),

  getFleetQuota: () => request<FleetQuotaSummary>('/api/quota/fleet'),

  getQuotaSummary: (email?: string) =>
    request<QuotaSummary>(email ? `/api/quota?email=${encodeURIComponent(email)}` : '/api/quota'),

  getAccounts: () => request<any[]>('/api/accounts'),

  scanLocalAccounts: () => request<import('./types').DiscoveredAccount[]>('/api/accounts/scan'),

  importAccount: (data: {
    email: string
    refresh_token?: string
    access_token?: string
    label?: string
    totp_secret?: string
  }) =>
    request<any>('/api/accounts/import', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    }),

  switchAccount: (email: string) =>
    request<{ success: boolean; active_account: string }>('/api/switch', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email }),
    }),

  updateAccount: (data: {
    email: string
    label?: string
    plan_tier?: string
    status?: string
    priority?: string
    notes?: string
    password?: string
    totp_secret?: string
    refresh_token?: string
    credits?: number
    enable_credit_overages?: boolean
    set_active?: boolean
  }) =>
    request<{ success: boolean; email: string }>('/api/accounts/update', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    }),

  startGoogleOAuth: () =>
    request<{ success: boolean; email?: string; refresh_token?: string; access_token?: string; error?: string }>(
      '/api/oauth/google/start',
      { method: 'POST' }
    ),

  getAuthStatus: () => request<{ password_required: boolean }>('/api/auth/status'),

  unlockApp: (password: string) =>
    request<{ success: boolean; error?: string }>('/api/auth/unlock', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ password }),
    }),

  getPasswordSettings: () => request<{ enabled: boolean }>('/api/settings/password'),

  setPasswordSettings: (data: { password?: string; current_password?: string; remove?: boolean }) =>
    request<{ success: boolean; enabled?: boolean; error?: string }>('/api/settings/password', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    }),

  deleteAccount: (email: string) =>
    request<{ success: boolean; removed: string }>('/api/accounts/delete', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email }),
    }),

  getTOTP: (email?: string) =>
    request<{ code: string; remaining_seconds: number; expires_in: number; email?: string }>(
      email ? `/api/totp?email=${encodeURIComponent(email)}` : '/api/totp'
    ),

  saveTOTP: (email: string, secret: string) =>
    request<{ success: boolean; email: string }>('/api/totp', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email, secret }),
    }),

  getFingerprint: (email?: string) =>
    request<DeviceProfile>(email ? `/api/fingerprint?email=${encodeURIComponent(email)}` : '/api/fingerprint'),

  listFingerprints: () => request<DeviceProfile[]>('/api/fingerprint?list=true'),

  saveFingerprint: (profile: Partial<DeviceProfile>) =>
    request<{ success: boolean }>('/api/fingerprint', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(profile),
    }),

  scanCache: (days = 7) => request<CacheBreakdown>(`/api/cache/scan?days=${days}`),

  pruneCache: (days = 7) =>
    request<{ freed_bytes: number; deleted_files: number }>('/api/cache/prune', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ older_than_days: days, cascade_shield: true }),
    }),

  getRules: () => request<RuleConfig>('/api/rules'),

  getSurfaces: () => request<import('./types').SurfacesResponse>('/api/surfaces'),

  saveRules: (rules: Partial<RuleConfig>) =>
    request<{ success: boolean }>('/api/rules', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(rules),
    }),

  setAutoSwitch: (enabled: boolean) =>
    request<{ success: boolean; auto_switch_enabled: boolean }>('/api/rules/auto_switch', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ enabled }),
    }),

  // System Installation & Update Diagnostics
  getInstallations: () => request<SystemInstallations>('/api/system/installations'),

  checkUpdates: () =>
    request<SystemInstallations>('/api/system/check_updates', {
      method: 'POST',
    }),

  // Custom Model Provider
  getCustomModels: () => request<CustomModelsConfig>('/api/custom_models'),

  saveCustomModel: (model: CustomModel) =>
    request<{ success: boolean; model: CustomModel }>('/api/custom_models', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(model),
    }),

  deleteCustomModel: (id: string) =>
    request<{ success: boolean; deleted: string }>(`/api/custom_models?id=${encodeURIComponent(id)}`, {
      method: 'DELETE',
    }),

  getProviderPresets: () => request<ProviderPreset[]>('/api/custom_models/presets'),

  testCustomModel: (model: CustomModel) =>
    request<TestResult>('/api/custom_models/test', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(model),
    }),

  fetchCustomModelQuota: (model: CustomModel) =>
    request<QuotaResult>('/api/custom_models/fetch_quota', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(model),
    }),

  refreshCustomModelQuotas: () =>
    request<CustomModelsConfig>('/api/custom_models/refresh_quotas', {
      method: 'POST',
    }),

  bindProjectModel: (project: string, modelId: string) =>
    request<{ success: boolean; project: string; model_id: string }>('/api/custom_models/bind', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ project, model_id: modelId }),
    }),

  fetchModels: (providerType: ProviderType, baseUrl: string, apiKey?: string) =>
    request<FetchModelsResponse>('/api/custom_models/fetch_models', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        provider_type: providerType,
        base_url: baseUrl,
        api_key: apiKey,
      }),
    }),

  setThinkingLevel: (modelId: string, level: string) =>
    request<{ success: boolean; model_id: string; level: string }>('/api/custom_models/thinking_level', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ model_id: modelId, level }),
    }),

  // App Enhancements (Prompt Jump Bar, Tool Density, Breaker Line)
  getEnhancements: () => request<EnhancementsConfig>('/api/enhancements'),

  updateEnhancements: (cfg: EnhancementsConfig) =>
    request<EnhancementsConfig>('/api/enhancements', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(cfg),
    }),

  applyEnhancements: () =>
    request<{ success: boolean; message: string }>('/api/enhancements/apply', {
      method: 'POST',
    }),

  // Project Archiving & GUI Improvements
  getArchivedProjects: () => request<ArchivedProjectItem[]>('/api/gui/projects/archived'),

  archiveProject: (nameOrId: string) =>
    request<{ success: boolean; archived: string }>('/api/gui/projects/archive', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name: nameOrId }),
    }),

  restoreProject: (nameOrId: string) =>
    request<{ success: boolean; restored: string }>('/api/gui/projects/restore', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name: nameOrId }),
    }),

  deleteArchivedProject: (nameOrId: string) =>
    request<{ success: boolean; name: string }>('/api/gui/projects/delete', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name: nameOrId }),
    }),

  openProjectSettings: (nameOrId: string) =>
    request<{ success: boolean; project: string }>('/api/gui/projects/open_settings', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name: nameOrId }),
    }),

  getGUIProjects: () =>
    request<Array<{ name: string; color: string; order: number; is_archived: boolean }>>('/api/gui/projects'),

  getGUIConfig: () => request<GUIConfig>('/api/gui/config'),

  updateGUIConfig: (cfg: Partial<GUIConfig>) =>
    request<GUIConfig>('/api/gui/config', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(cfg),
    }),

  applyGUI: () =>
    request<{ success: boolean; message: string }>('/api/gui/apply', {
      method: 'POST',
    }),

  autoArchiveConversations: (horizon?: string) =>
    request<AutoArchiveResult>('/api/gui/conversations/auto-archive', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ horizon }),
    }),

  // Scheduled Task Templates (Automations)
  getTemplates: () => request<ScheduledTemplate[]>('/api/templates'),

  deployTemplate: (data: DeployTaskRequest) =>
    request<{ success: boolean; task: SidecarTaskInfo }>('/api/templates/deploy', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    }),

  getSidecars: () => request<SidecarTaskInfo[]>('/api/templates/sidecars'),

  updateSidecar: (data: UpdateTaskRequest) =>
    request<{ success: boolean; task: SidecarTaskInfo }>('/api/templates/sidecars/update', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    }),

  deleteSidecar: (id: string) =>
    request<{ success: boolean; deleted: string }>(`/api/templates/sidecars?id=${encodeURIComponent(id)}`, {
      method: 'DELETE',
    }),

  // Real Custom Models Security Audit API
  auditCustomModelSecurity: (model: any) =>
    request<any>('/api/custom_models/security-audit', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(model),
    }),

  // Real Filesystem Explorer API
  listFiles: (path?: string) =>
    request<{ success: boolean; path: string; files: Array<{ name: string; isDir: boolean; type: string; size: string; path: string; modTime: string }> }>(
      `/api/files/list?path=${encodeURIComponent(path || '')}`
    ),

  readFile: (path: string) =>
    request<{ success: boolean; path: string; content: string; size: number }>(
      `/api/files/read?path=${encodeURIComponent(path)}`
    ),

  writeFile: (path: string, content: string) =>
    request<{ success: boolean; path: string; size: number }>('/api/files/write', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ path, content }),
    }),

  renameFile: (oldPath: string, newPath: string) =>
    request<{ success: boolean; old_path: string; new_path: string }>('/api/files/rename', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ old_path: oldPath, new_path: newPath }),
    }),

  deleteFile: (path: string) =>
    request<{ success: boolean; deleted: string }>('/api/files/delete', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ path }),
    }),

  createFile: (path: string, isDir = false) =>
    request<{ success: boolean; path: string; is_dir: boolean }>('/api/files/create', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ path, is_dir: isDir }),
    }),

  copyFile: (src: string, dst: string) =>
    request<{ success: boolean; src: string; dst: string }>('/api/files/copy', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ src, dst }),
    }),

  moveFile: (src: string, dst: string) =>
    request<{ success: boolean; src: string; dst: string }>('/api/files/move', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ src, dst }),
    }),

  revealFile: (path: string) =>
    request<{ success: boolean; revealed: string }>('/api/files/reveal', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ path }),
    }),

  openTerminal: (path: string) =>
    request<{ success: boolean; dir: string }>('/api/files/terminal', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ path }),
    }),

  // Quick Memos API
  getMemos: () =>
    request<{ success: boolean; memos: any[] }>('/api/memos'),

  saveMemo: (memo: any) =>
    request<{ success: boolean; memo: any }>('/api/memos/save', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(memo),
    }),

  deleteMemo: (id: string) =>
    request<{ success: boolean; deleted: string }>(`/api/memos/delete?id=${encodeURIComponent(id)}`, {
      method: 'POST',
    }),

  // Real Utilities & ACP Mesh API
  scanImportCandidates: (source: string) =>
    request<{ success: boolean; source: string; count: number; candidates: any[] }>(
      `/api/utilities/import/scan?source=${encodeURIComponent(source)}`
    ),

  executeImport: (candidateIds: string[], source: string = 'opencode', mode: string = 'auto') =>
    request<{ success: boolean; imported_count: number; message: string }>('/api/utilities/import', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ candidate_ids: candidateIds, source, mode }),
    }),

  getAcpMesh: () =>
    request<{ status: string; mesh_nodes: number; protocol_version: string; agents: any[] }>('/api/utilities/acp'),

  // Real Token Analytics API
  getTokenSummary: () =>
    request<{
      total_tokens: number
      input_tokens: number
      cached_input_tokens: number
      output_tokens: number
      total_cost_usd: number
      saved_cost_usd: number
      avg_tps: number
      requests_count: number
    }>('/api/tokens/summary'),
}
