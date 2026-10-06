export interface SystemStatus {
  daemon_running: boolean
  daemon_pid: number
  version: string
  active_account: string
  total_accounts: number
  antigravity_running: boolean
  antigravity_pid?: number
}

export interface ModelQuota {
  model_name: string
  fraction: number
  reset_time: string
  reset_text: string
  health_status: string
}

export interface QuotaSummary {
  account_email: string
  models: ModelQuota[]
  min_fraction: number
  overall_health: string
}

export const CANONICAL_PLAN_TIERS = [
  'Free',
  'Plus',
  'Pro',
  'Pro - Trial',
  'Edu',
  'Ultra 5X',
  'Ultra 10X',
  'Ultra 20X',
  'Enterprise',
] as const

export type PlanTier = typeof CANONICAL_PLAN_TIERS[number]

export function normalizePlanTier(raw?: string): string {
  if (!raw) return 'Free'
  const trimmed = raw.trim()
  const lower = trimmed.toLowerCase()
  if (lower === 'free' || lower === 'free-tier' || lower === 'tier_free') return 'Free'
  if (lower.includes('trial')) return 'Pro - Trial'
  if (lower.includes('20x') || lower.includes('ultra_20x') || lower.includes('ultra 20x')) return 'Ultra 20X'
  if (lower.includes('10x') || lower.includes('ultra_10x') || lower.includes('ultra 10x')) return 'Ultra 10X'
  if (lower.includes('5x') || lower.includes('ultra_5x') || lower.includes('ultra 5x')) return 'Ultra 5X'
  if (lower.includes('ultra')) return 'Ultra 20X'
  if (lower.includes('edu') || lower.includes('education') || lower.includes('student') || lower.includes('academic')) return 'Edu'
  if (lower.includes('enterprise') || lower.includes('teams_tier_enterprise')) return 'Enterprise'
  if (lower.includes('plus')) return 'Plus'
  if (lower.includes('pro') || lower.includes('standard') || lower.includes('code assist') || lower.includes('ai premium') || lower.includes('g1_ai') || lower.includes('team')) return 'Pro'
  return trimmed
}

export type SwitchMode = 'balanced' | 'max_tokens' | 'max_continuous'

export interface AccountState {
  email: string
  label?: string
  plan_tier?: string
  priority?: 'High' | 'Mid' | 'Low' | string
  notes?: string
  password?: string
  is_active: boolean
  status: 'ACTIVE' | 'STANDBY' | 'ERROR' | 'BANNED' | string
  quota_5h_current?: number
  quota_5h_available: number
  reset_seconds?: number
  quota_weekly: number
  reset_seconds_weekly?: number
  quota_5h_claude_gpt?: number
  quota_weekly_claude_gpt?: number
  reset_horizon_text: string
  reset_horizon_weekly_text?: string
  has_mfa: boolean
  totp_secret?: string
  refresh_token?: string
  credits?: number
  enable_credit_overages?: boolean
  allow_claude_gpt?: boolean
  error_message?: string
  status_reason?: string
}

export interface FleetQuotaSummary {
  fleet_5h_available: number
  fleet_weekly_available: number
  fleet_5h_gemini_available?: number
  fleet_weekly_gemini_available?: number
  fleet_5h_claude_gpt_available?: number
  fleet_weekly_claude_gpt_available?: number
  total_accounts: number
  active_account: string
  accounts: AccountState[]
}

export interface RuleConfig {
  auto_switch_enabled: boolean
  auto_switch_threshold: number
  switch_mode?: SwitchMode
  polling_interval_seconds: number
  active_polling_interval_seconds?: number
  standby_polling_interval_seconds?: number
  standby_random_jitter_seconds?: number
  warmup_enabled: boolean
  warmup_lead_time_seconds: number
  preferred_native_model?: string
  allow_ai_credits_usage?: boolean
  allow_non_gemini_native_models?: boolean
  model_source_hierarchy?: string[]
  default_gemini_model?: string
  default_custom_model?: string
  default_non_gemini_model?: string
  default_gemini_reasoning_level?: string
  auto_import_active_account?: boolean
}

export interface AvailableModelItem {
  id: string
  display_name: string
  supports_thinking?: boolean
  thinking_levels?: string[]
  recommended?: boolean
  provider?: string
}

export interface AvailableModelsResponse {
  success: boolean
  gemini_models: AvailableModelItem[]
  non_gemini_models: AvailableModelItem[]
  default_gemini?: string
  default_non_gemini?: string
  timestamp?: string
}

export interface SurfaceAccount {
  email: string
  surface: 'desktop' | 'vscode' | 'cli'
  surface_name: string
  access_token?: string
  refresh_token?: string
  id_token?: string
}

export interface SurfacesResponse {
  surfaces: Record<string, SurfaceAccount | null>
  active_surface_account: SurfaceAccount | null
  priority_sequence: string[]
}

export interface DiscoveredAccount {
  email: string
  source: string
  has_tokens: boolean
  has_refresh: boolean
  access_token?: string
  refresh_token?: string
  is_active_in_ide: boolean
  already_in_vault: boolean
}

export interface DeviceProfile {
  account_email: string
  machine_id: string
  updater_id: string
  installation_id: string
  installation_uuid: string
  last_generated?: string
}

export interface CacheCategory {
  name: string
  path: string
  size_bytes: number
  file_count: number
}

export interface CacheBreakdown {
  total_bytes: number
  total_files: number
  categories: CacheCategory[]
  reclaimable_bytes: number
  safe_to_delete: boolean
}

export type ProviderType = 'openai' | 'anthropic' | 'gemini' | 'custom' | 'local'
export type QuotaType = 'na' | 'balance' | 'quota' | 'cost_based' | 'quota_based' | 'none'

export interface QuotaResult {
  quota_type: QuotaType
  balance_value?: string
  quota_value?: string
  fraction: number | null
  has_percentage: boolean
  message?: string
}

export interface CustomModel {
  id: string
  name: string
  display_name: string
  provider_type: ProviderType
  base_url: string
  api_key?: string
  project_mappings: string[]
  quota_type: QuotaType
  balance_value?: string
  quota_value?: string
  prepaid_balance: number
  total_budget: number
  quota_fraction: number | null
  is_default: boolean
  context_window?: number
  supports_thinking?: boolean
  thinking_levels?: string[]
  thinking_level?: string
  enabled: boolean
  notes?: string
  security_risk_level?: 'low' | 'medium' | 'high' | 'critical'
  security_audit_score?: number
  last_security_audit?: string
  created_at?: string
  updated_at?: string
}

export interface SecurityAuditProbe {
  id: string
  name: string
  category: string
  description: string
  status: 'passed' | 'warning' | 'failed'
  details: string
  evidence?: string
}

export interface SecurityAuditReport {
  risk_level: 'low' | 'medium' | 'high' | 'critical'
  risk_score: number // 0 (safest) to 100 (critical danger)
  security_grade?: string // 'A+' | 'A' | 'B' | 'C' | 'D' | 'F'
  model_id: string
  endpoint: string
  provider_type: string
  audited_at: string
  summary: string
  probes: SecurityAuditProbe[]
  recommendations: string[]
}

export interface ModelInfo {
  id: string
  display_name?: string
  context_window?: number
  supports_thinking?: boolean
  thinking_levels?: string[]
  description?: string
}

export interface FetchModelsResponse {
  success: boolean
  models: ModelInfo[]
  message?: string
}

export interface CustomModelsConfig {
  version: string
  active_model_id?: string
  models: CustomModel[]
  project_binds: Record<string, string>
}

export interface ProviderPreset {
  id: string
  name: string
  provider_type: ProviderType
  default_base_url: string
  calling_format: string
  description: string
  auth_header: string
  popular_models: string[]
}

export interface TestResult {
  success: boolean
  latency_ms: number
  status_code: number
  message: string
  endpoint: string
  quota_result?: QuotaResult
}

export interface InstallationInfo {
  installed: boolean
  path: string
  version: string
  up_to_date: boolean
  latest_version: string
  process_state?: string
  target_type: string
}

export interface SystemInstallations {
  desktop_app: InstallationInfo
  vscode_extension: InstallationInfo
  platform: string
  arch: string
}

// App Enhancements types
export interface PromptJumpBarConfig {
  enabled: boolean
  show_tooltip: boolean
  focus_pulse: boolean
  sync_scroll: boolean
  position: 'gutter' | 'floating'
  dash_width: number
  dash_thickness?: number
  inactive_thickness?: number
  color_mode: 'default' | 'project' | 'custom'
  custom_color: string
}

export interface OverviewPanelConfig {
  enabled: boolean
  division_style: 'divider_line' | 'border_zone'
  line_thickness: number
  line_width_percent: number
  line_color: string
  line_style: 'solid' | 'dashed' | 'dotted'
  line_margin: number
  zone_border_radius: number
  zone_border_color: string
  zone_background_contrast: 'whiter' | 'subtle' | 'card'
  zone_padding: number
  zone_gap: number
  replace_see_all_triangle: boolean
  aux_tabs_format?: 'icon' | 'icon_and_name'
}

export interface EnhancementsConfig {
  version: string
  enabled: boolean
  prompt_jump_bar: PromptJumpBarConfig
  overview_panel?: OverviewPanelConfig
  tool_density_mode: 'normal' | 'muted' | 'hidden'
  breaker_line_enabled: boolean
  scroll_to_bottom: boolean
  turn_counter: boolean
  default_new_project?: string
  updated_at?: string
}

export interface ArchivedProjectItem {
  id: string
  name: string
  color: string
  conversation_count: number
  last_active_time: string
  last_active_relative: string
  folder_uri: string
  archived_at: string
}


// Scheduled Task Templates types
export interface TemplateSchedule {
  frequency: string
  time_of_day: string
  days_of_week: number[]
  cron_expression: string
  schedule_text?: string
}

export interface TemplateParameter {
  key: string
  label: string
  type: string
  default_value: string
  description: string
}

export interface ScheduledTemplate {
  id: string
  title: string
  subtitle: string
  category: string
  icon: string
  description: string
  default_schedule: TemplateSchedule
  required_tools: string[]
  required_skills: string[]
  parameters: TemplateParameter[]
  prompt_template: string
}

export interface DeployTaskRequest {
  template_id: string
  display_name: string
  cron_expression: string
  target_project: string
  custom_prompt?: string
  parameters: Record<string, string>
}

export interface SidecarTaskInfo {
  id: string
  display_name: string
  cron_expression: string
  schedule_text?: string
  prompt_preview: string
  prompt?: string
  path: string
}

export interface UpdateTaskRequest {
  id: string
  display_name: string
  cron_expression: string
  prompt: string
}

export interface AutoArchiveResult {
  success: boolean
  archived_count: number
  archived_ids: string[]
  horizon: string
  cutoff_time: string
  message: string
}

export interface GUIConfig {
  enabled: boolean
  color_styling_enabled: boolean
  solid_left_edge: boolean
  tint_opacity: number
  active_conversation_indicator: 'background' | 'border' | 'left_bar'
  active_conversation_border_width?: string
  active_conversation_bold: boolean
  project_colors: Record<string, string>
  drag_rearrange_enabled: boolean
  project_order: string[]
  archived_projects: string[]
  conversation_tabs_mode: 'fixed' | 'dynamic'
  conversation_tabs_fixed_limit: number
  conversation_tabs_age_threshold: '1d' | '3d' | '7d' | '14d' | '30d'
  conversation_tabs_min: number
  conversation_tabs_max: number
  replace_see_all_triangle?: boolean
  auto_archive_conversations: boolean
  auto_archive_horizon: '3d' | '7d' | '14d' | '30d' | '60d' | '90d'
  auto_inject: boolean
}

// Token Monitor types
export interface ModelPricing {
  model_id: string
  name: string
  provider: 'gemini' | 'anthropic' | 'openai' | 'deepseek' | 'local' | 'other'
  input_price_per_m: number // USD per 1M tokens
  cached_input_price_per_m: number
  output_price_per_m: number
  source: 'api' | 'manual'
  updated_at: string
}

export interface TokenUsageSummary {
  total_tokens: number
  input_tokens: number
  cached_input_tokens: number
  output_tokens: number
  total_cost_usd: number
  saved_cost_usd: number
  avg_tps: number
  requests_count: number
}

export interface ModelUsageBreakdown {
  model_id: string
  model_name: string
  provider: string
  total_tokens: number
  input_tokens: number
  cached_tokens: number
  output_tokens: number
  cost_usd: number
  requests: number
  avg_tps: number
}

export interface AccountUsageBreakdown {
  email: string
  display_name: string
  total_tokens: number
  cost_usd: number
  percentage: number
}

export interface ProjectUsageBreakdown {
  project_name: string
  project_uri: string
  total_tokens: number
  cost_usd: number
  requests: number
}

export interface LiveTelemetryEvent {
  id: string
  timestamp: string
  session_id: string
  project_name: string
  model_id: string
  provider: string
  input_tokens: number
  cached_tokens: number
  output_tokens: number
  tps: number
  cost_usd: number
  subagent_count: number
  status: 'completed' | 'streaming' | 'failed'
}

// Utilities (Chat Import & ACP Inspector) types
export type ChatImportSource =
  | 'opencode'
  | 'dsh'
  | 'devin'
  | 'pi'
  | 'cursor'
  | 'chatgpt'
  | 'windsurf'
  | 'copilot'
  | 'openwebui'
  | 'claude-code'
  | 'raw-json'
  | 'custom-file'

export type ProjectMatchOption =
  | 'auto'
  | 'create-new'
  | 'standalone'
  | 'single-dedicated'
  | 'source-dedicated'

export interface ImportCandidate {
  id: string
  source: ChatImportSource
  title: string
  message_count: number
  tool_calls_count: number
  token_estimate: number
  detected_project_path: string
  target_antigravity_project: string
  match_status: 'exact' | 'heuristic' | 'new' | 'standalone'
  selected: boolean
}

export interface ImportHistoryItem {
  id: string
  timestamp: string
  source: string
  conversation_count: number
  target_project: string
  status: 'completed' | 'failed' | 'partial'
  duration_ms: number
}

export interface AcpAgentInstance {
  id: string
  name: string
  type: string
  binary_path: string
  pid: number
  port_socket: string
  acp_version: string
  status: 'active_hosting' | 'listening' | 'connected' | 'idle' | 'unreachable'
  ping_latency_ms: number
  supported_tools: string[]
  last_handshake: string
}

export interface AcpHandshakeLog {
  id: string
  timestamp: string
  from_agent: string
  to_agent: string
  action: string
  payload_summary: string
  status: 'success' | 'warning' | 'error'
}

export interface StoragePaths {
  config_dir: string
  credentials_path: string
  temp_dir: string
  socket_path: string
}

export interface AppZoneDetail {
  app_type: 'desktop' | 'agy' | 'vscode' | string
  display_name: string
  detected_path: string
  custom_path: string
  active_path: string
  installed: boolean
  version: string
  account_overrides: Record<string, string>
}

export interface AppZonesInfo {
  desktop: AppZoneDetail
  agy: AppZoneDetail
  vscode: AppZoneDetail
}

export interface ClearCacheResult {
  success: boolean
  app_type: string
  freed_bytes: number
  deleted_files: number
  message: string
  error?: string
}

export interface FactoryResetResult {
  success: boolean
  message: string
  error?: string
}

export interface StorageInfo {
  storage_mode: 'system_default' | 'app_portable'
  current_paths: StoragePaths
  system_default_paths: StoragePaths
  app_portable_paths: StoragePaths
  app_execution_type: string
  app_execution_detail: string
  can_migrate: boolean
  app_zones?: AppZonesInfo
}

export interface PrivacySettings {
  anonymous_error_reports: boolean
  anonymous_telemetry: boolean
  github_repo: string
}

export interface DiagnosticResult {
  success: boolean
  agent_selected: string
  agent_priority_chain: string[]
  account_or_model_used: string
  resolution_source: string
  sanitized_report: string
  issue_title: string
  issue_url: string
  github_repo: string
  sensitive_data_redacted: boolean
  redacted_token_count: number
  error?: string
}


