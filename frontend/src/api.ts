import type {
  CacheBreakdown,
  CacheConfig,
  VaultStatus,
  VaultSyncResult,
  CustomModel,
  CustomModelsConfig,
  DeviceProfile,
  FleetQuotaSummary,
  ProviderPreset,
  QuotaSummary,
  RuleConfig,
  SystemInstallations,
  AppReleaseInfo,
  UpgradeResult,
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
  AvailableModelsResponse,
  StorageInfo,
  PrivacySettings,
  DiagnosticResult,
  ModelPricingRecord,
  TokenSummaryResponse,
  TargetApp,
  AcpAgentInstance,
  ComputerUseStatus,
  CalibrationResult,
  OSComputerUseSettings,
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

  refreshFleetQuota: () => request<{ status: string }>('/api/quota/refresh', { method: 'POST' }),

  getQuotaSummary: (email?: string, refresh?: boolean) => {
    const params = new URLSearchParams()
    if (email) params.set('email', email)
    if (refresh) params.set('refresh', 'true')
    const qs = params.toString()
    return request<QuotaSummary>(qs ? `/api/quota?${qs}` : '/api/quota')
  },

  refreshAccountQuota: (email: string) =>
    request<QuotaSummary>(`/api/quota?email=${encodeURIComponent(email)}&refresh=true`),

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

  exportAccounts: () => request<any[]>('/api/accounts/export'),

  batchImportAccounts: (accounts: any) =>
    request<{ success: boolean; imported: number; message: string }>('/api/accounts/batch-import', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(accounts),
    }),

  switchAccount: (email: string, relaunch_ide: boolean = true, target_app?: TargetApp | string) =>
    request<{
      success: boolean
      active_account: string
      relaunch_ide?: boolean
      target_app?: string
      multi_app_sync_mode?: string
      active_app_accounts?: Record<string, string>
    }>('/api/switch', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email, relaunch_ide, target_app }),
    }),

  relaunchHostIDE: () =>
    request<{ success: boolean; message?: string }>('/api/desktop/relaunch', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({}),
    }),

  launchHostIDE: () =>
    request<{ success: boolean; message?: string }>('/api/desktop/launch', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({}),
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
    access_token?: string
    credits?: number
    enable_credit_overages?: boolean
    allow_claude_gpt?: boolean
    set_active?: boolean
  }) =>
    request<{
      success: boolean
      email: string
      plan_tier?: string
      credits?: number
      quota?: QuotaSummary
    }>('/api/accounts/update', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    }),

  startGoogleOAuth: (signal?: AbortSignal) =>
    request<{ success: boolean; email?: string; refresh_token?: string; access_token?: string; error?: string }>(
      '/api/oauth/google/start',
      { method: 'POST', signal }
    ),

  cancelGoogleOAuth: () =>
    request<{ success: boolean; cancelled?: boolean }>('/api/oauth/google/cancel', {
      method: 'POST',
    }),

  exchangeGoogleOAuth: (payload: { callback_url?: string; code?: string; redirect_uri?: string }) =>
    request<{ success: boolean; email?: string; refresh_token?: string; access_token?: string; error?: string }>(
      '/api/oauth/google/exchange',
      {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      }
    ),

  getGoogleOAuthURL: () =>
    request<{ success: boolean; active?: boolean; auth_url?: string }>('/api/oauth/google/url'),

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

  getStorageSettings: () => request<StorageInfo>('/api/settings/storage'),

  setStorageSettings: (data: { storage_mode: string; migrate_data: boolean }) =>
    request<{ success: boolean; storage: StorageInfo; error?: string }>('/api/settings/storage', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    }),

  saveAppPath: (appType: string, path: string) =>
    request<{ success: boolean; storage: StorageInfo; error?: string }>('/api/settings/app_path', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ app_type: appType, path }),
    }),

  saveAccountOverride: (appType: string, email: string, path: string) =>
    request<{ success: boolean; storage: StorageInfo; error?: string }>('/api/settings/account_override', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ app_type: appType, email, path }),
    }),

  clearAppCache: (appType: string) =>
    request<import('./types').ClearCacheResult>('/api/settings/cache_clear', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ app_type: appType }),
    }),

  factoryReset: () =>
    request<import('./types').FactoryResetResult>('/api/system/factory_reset', {
      method: 'POST',
    }),

  getPrivacySettings: () => request<PrivacySettings>('/api/settings/privacy'),

  setPrivacySettings: (data: { anonymous_error_reports: boolean; anonymous_telemetry: boolean }) =>
    request<{ success: boolean; anonymous_error_reports: boolean; anonymous_telemetry: boolean; error?: string }>(
      '/api/settings/privacy',
      {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(data),
      }
    ),

  runIssueDiagnosis: (data: { description: string; include_system_info?: boolean; include_logs?: boolean }) =>
    request<DiagnosticResult>('/api/settings/diagnose-issue', {
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

  scanCache: (days = 0, maxSizeGB = 0) =>
    request<CacheBreakdown>(`/api/cache/scan?days=${days}&max_size_gb=${maxSizeGB}`),

  pruneCache: (days = 0, maxSizeGB = 0) =>
    request<{ freed_bytes: number; deleted_files: number }>('/api/cache/prune', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ older_than_days: days, max_size_gb: maxSizeGB, cascade_shield: true }),
    }),

  getCacheConfig: () => request<CacheConfig>('/api/cache/config'),

  saveCacheConfig: (cfg: Partial<CacheConfig>) =>
    request<CacheConfig & { success: boolean }>('/api/cache/config', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(cfg),
    }),

  getVaultStatus: () => request<VaultStatus>('/api/vault/status'),

  syncVault: () =>
    request<VaultSyncResult>('/api/vault/sync', {
      method: 'POST',
    }),

  toggleVault: (enabled: boolean) =>
    request<{ success: boolean; enabled: boolean }>('/api/vault/toggle', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ enabled }),
    }),

  getRules: () => request<RuleConfig>('/api/rules'),

  getAvailableModels: (force?: boolean) =>
    request<AvailableModelsResponse>(force ? '/api/models/available?force=true' : '/api/models/available'),

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

  // App Releases & Upgrade Management
  getAppRelease: () => request<AppReleaseInfo>('/api/system/app_release'),

  checkAppRelease: () =>
    request<AppReleaseInfo>('/api/system/check_app_release', {
      method: 'POST',
    }),

  saveAppReleaseSettings: (settings: { auto_check: boolean; auto_upgrade: boolean }) =>
    request<{ success: boolean; auto_check: boolean; auto_upgrade: boolean }>('/api/system/app_release/settings', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(settings),
    }),

  upgradeApp: () =>
    request<UpgradeResult>('/api/system/app_release/upgrade', {
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

  getDesktopPersistenceStatus: () =>
    request<{ installed: boolean; persistent_visual_effects: boolean; backup_exists: boolean }>('/api/gui/desktop/status'),

  setDesktopPersistence: (enabled: boolean) =>
    request<{ success: boolean; persistent_visual_effects: boolean; installed?: boolean; message?: string }>('/api/gui/desktop/persistence', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ enabled }),
    }),

  getRuntimeMode: () =>
    request<{ runtime_mode: 'app' | 'daemon' }>('/api/settings/runtime-mode'),

  setRuntimeMode: (runtime_mode: 'app' | 'daemon') =>
    request<{ success: boolean; runtime_mode: 'app' | 'daemon' }>('/api/settings/runtime-mode', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ runtime_mode }),
    }),

  startDaemon: () =>
    request<{ success: boolean; daemon_running?: boolean; message?: string }>('/api/daemon/start', {
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

  deleteFiles: (paths: string[]) =>
    request<{ success: boolean; paths: string[] }>('/api/files/delete', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ paths }),
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

  copyFiles: (items: Array<{ src: string; dst: string }>) =>
    request<{ success: boolean; results: Array<{ src: string; dst: string }>; count: number }>('/api/files/copy', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ items }),
    }),

  moveFile: (src: string, dst: string) =>
    request<{ success: boolean; src: string; dst: string }>('/api/files/move', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ src, dst }),
    }),

  moveFiles: (items: Array<{ src: string; dst: string }>) =>
    request<{ success: boolean; results: Array<{ src: string; dst: string }>; count: number }>('/api/files/move', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ items }),
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

  openIDE: (path: string, ide?: string) =>
    request<{ success: boolean; dir: string; ide?: string; launched?: string }>('/api/files/open_ide', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ path, ide }),
    }),

  getPreferredIDE: () =>
    request<{ success: boolean; preferred_ide: string }>('/api/files/ide/config'),

  setPreferredIDE: (ide: string) =>
    request<{ success: boolean; preferred_ide: string }>('/api/files/ide/config', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ preferred_ide: ide }),
    }),

  // Quick Memos API
  getMemos: (params?: { workspacePath?: string; storage?: 'global' | 'project'; scope?: 'all' | 'current' }) => {
    const q = new URLSearchParams();
    if (params?.workspacePath) q.set('workspace_path', params.workspacePath);
    if (params?.storage) q.set('storage', params.storage);
    if (params?.scope) q.set('scope', params.scope);
    const qs = q.toString();
    const headers: Record<string, string> = {};
    if (params?.workspacePath) {
      headers['X-Workspace-Path'] = params.workspacePath;
    }
    return request<{ success: boolean; memos: any[]; fallback?: boolean }>(
      qs ? `/api/memos?${qs}` : '/api/memos',
      { headers }
    );
  },

  saveMemo: (memo: any, options?: { workspacePath?: string; storage?: 'global' | 'project' }) => {
    const q = new URLSearchParams();
    if (options?.workspacePath) q.set('workspace_path', options.workspacePath);
    if (options?.storage) q.set('storage', options.storage);
    const qs = q.toString();
    const headers: Record<string, string> = { 'Content-Type': 'application/json' };
    if (options?.workspacePath) {
      headers['X-Workspace-Path'] = options.workspacePath;
    }
    return request<{ success: boolean; memo: any; fallback?: boolean; storage_location_effective?: string }>(
      qs ? `/api/memos/save?${qs}` : '/api/memos/save',
      {
        method: 'POST',
        headers,
        body: JSON.stringify(memo),
      }
    );
  },

  deleteMemo: (id: string, options?: { workspacePath?: string; storage?: 'global' | 'project' }) => {
    const q = new URLSearchParams();
    q.set('id', id);
    if (options?.workspacePath) q.set('workspace_path', options.workspacePath);
    if (options?.storage) q.set('storage', options.storage);
    const headers: Record<string, string> = {};
    if (options?.workspacePath) {
      headers['X-Workspace-Path'] = options.workspacePath;
    }
    return request<{ success: boolean; deleted: string }>(`/api/memos/delete?${q.toString()}`, {
      method: 'POST',
      headers,
    });
  },

  getMemoConfig: () =>
    request<{
      success: boolean;
      config: { storage_location: string; view_scope: string; search_scope: string };
      storage_location?: string;
      view_scope?: string;
      search_scope?: string;
    }>('/api/memos/config'),

  updateMemoConfig: (cfg: { storage_location?: string; view_scope?: string; search_scope?: string }) =>
    request<{
      success: boolean;
      config: { storage_location: string; view_scope: string; search_scope: string };
      storage_location?: string;
      view_scope?: string;
      search_scope?: string;
    }>('/api/memos/config', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(cfg),
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
    request<{ status: string; mesh_nodes: number; protocol_version: string; agents: AcpAgentInstance[] }>('/api/utilities/acp'),

  // Real Token Analytics & Pricing API
  getTokenSummary: () =>
    request<TokenSummaryResponse>('/api/tokens/summary'),

  getTokenPricing: (force?: boolean) =>
    request<{ success: boolean; pricing_records: ModelPricingRecord[]; models?: ModelPricingRecord[]; timestamp: string }>(
      force ? '/api/tokens/pricing?force=true' : '/api/tokens/pricing'
    ),

  updateTokenPricing: (data: {
    internal_id?: number
    canonical_id?: string
    model_id?: string
    input_price_per_m: number | null
    cached_input_price_per_m: number | null
    output_price_per_m: number | null
  }) =>
    request<{ success: boolean; pricing_record?: ModelPricingRecord; model?: ModelPricingRecord; models?: ModelPricingRecord[] }>('/api/tokens/pricing', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        ...data,
        canonical_id: data.canonical_id || data.model_id,
        model_id: data.model_id || data.canonical_id,
      }),
    }),

  deleteTokenPricingModel: (params: { internal_id?: number; canonical_id?: string; model_id?: string }) => {
    const qs = new URLSearchParams()
    if (params.internal_id) qs.set('internal_id', String(params.internal_id))
    const mId = params.canonical_id || params.model_id
    if (mId) {
      qs.set('canonical_id', mId)
      qs.set('model_id', mId)
    }
    return request<{ success: boolean; deleted?: string; models?: ModelPricingRecord[] }>(`/api/tokens/pricing?${qs.toString()}`, {
      method: 'DELETE',
    })
  },

  // GitHub Workspace & Task Tracking API
  getGitHubRepo: (workspacePath?: string) =>
    request<{ success: boolean; repo: any }>(`/api/github/repo?workspace_path=${encodeURIComponent(workspacePath || '.')}`),

  getGitHubIssues: (workspacePath?: string, state?: string, search?: string) =>
    request<{ success: boolean; issues: any[]; repo: any }>(
      `/api/github/issues?workspace_path=${encodeURIComponent(workspacePath || '.')}&state=${encodeURIComponent(state || 'all')}&search=${encodeURIComponent(search || '')}`
    ),

  getGitHubIssueDetail: (number: number, workspacePath?: string) =>
    request<{ success: boolean; issue: any }>(
      `/api/github/issues/detail?number=${number}&workspace_path=${encodeURIComponent(workspacePath || '.')}`
    ),

  updateGitHubIssue: (data: { workspace_path?: string; number: number; title?: string; body?: string; state?: string }) =>
    request<{ success: boolean; issue: any }>('/api/github/issues/update', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    }),

  addGitHubComment: (data: { workspace_path?: string; number: number; comment: string }) =>
    request<{ success: boolean }>('/api/github/issues/comment', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    }),

  createGitHubIssue: (data: { workspace_path?: string; title: string; body: string; labels?: string[]; assignees?: string[] }) =>
    request<{ success: boolean; issue: any }>('/api/github/issues/create', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    }),

  getGitHubPRs: (workspacePath?: string, state?: string) =>
    request<{ success: boolean; prs: any[]; repo: any }>(
      `/api/github/prs?workspace_path=${encodeURIComponent(workspacePath || '.')}&state=${encodeURIComponent(state || 'all')}`
    ),

  getGitHubPRDetail: (number: number, workspacePath?: string) =>
    request<{ success: boolean; pr: any }>(
      `/api/github/prs/detail?number=${number}&workspace_path=${encodeURIComponent(workspacePath || '.')}`
    ),

  getGitHubAgentTasks: (workspacePath?: string) =>
    request<{ success: boolean; tasks: any[] }>(
      `/api/github/agent-tasks?workspace_path=${encodeURIComponent(workspacePath || '.')}`
    ),

  bindGitHubAgentTask: (data: { conversation_id: string; issue_number: number; agent_label?: string }) =>
    request<{ success: boolean }>('/api/github/agent-tasks/bind', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    }),

  setGitHubAgentLabel: (data: { conversation_id: string; agent_label: string }) =>
    request<{ success: boolean }>('/api/github/agent-tasks/label', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    }),

  getGitHubContext: (number: number, type?: string, workspacePath?: string) =>
    request<{ success: boolean; context: string }>(
      `/api/github/context?number=${number}&type=${type || 'issue'}&workspace_path=${encodeURIComponent(workspacePath || '.')}`
    ),

  getGitHubKanbanBoard: (workspacePath?: string, projectNumber?: number) =>
    request<{ success: boolean; board?: any; repo?: any; error?: string }>(
      `/api/github/kanban?workspace_path=${encodeURIComponent(workspacePath || '.')}${
        projectNumber ? `&project_number=${projectNumber}` : ''
      }`
    ),

  moveGitHubKanbanCard: (data: {
    workspace_path?: string
    card_id: string
    card_type: string
    number: number
    source_column: string
    target_column: string
    project_number?: number
    project_item_id?: string
  }) =>
    request<{ success: boolean; message?: string; error?: string }>('/api/github/kanban/move', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    }),

  // Computer Use Enhancer API
  getComputerUseStatus: () => request<{ success: boolean; data: ComputerUseStatus }>('/api/system/computer-use'),
  calibrateComputerUse: (targetOS: string, inputX: number, inputY: number) =>
    request<{ success: boolean; data: CalibrationResult }>('/api/system/computer-use/calibrate', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ target_os: targetOS, input_x: inputX, input_y: inputY }),
    }),
  saveComputerUseSettings: (settings: Partial<OSComputerUseSettings>) =>
    request<{ success: boolean; settings: OSComputerUseSettings }>('/api/system/computer-use/config', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(settings),
    }),
}

