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
  last_polled?: string
}

export interface AccountState {
  email: string
  label?: string
  plan_tier?: string
  priority?: 'High' | 'Mid' | 'Low' | string
  notes?: string
  password?: string
  is_active: boolean
  status: 'ACTIVE' | 'STANDBY' | 'ERROR' | 'BANNED' | string
  quota_5h_available: number
  quota_weekly: number
  reset_horizon_text: string
  has_mfa: boolean
  totp_secret?: string
  refresh_token?: string
  error_message?: string
  status_reason?: string
}

export interface FleetQuotaSummary {
  fleet_5h_available: number
  fleet_weekly_available: number
  total_accounts: number
  active_account: string
  accounts: AccountState[]
}

export interface RuleConfig {
  auto_switch_enabled: boolean
  auto_switch_threshold: number
  polling_interval_seconds: number
  warmup_enabled: boolean
  warmup_lead_time_seconds: number
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
export type QuotaType = 'cost_based' | 'quota_based' | 'none'

export interface CustomModel {
  id: string
  name: string
  display_name: string
  provider_type: ProviderType
  base_url: string
  api_key?: string
  project_mappings: string[]
  quota_type: QuotaType
  prepaid_balance: number
  total_budget: number
  quota_fraction: number | null
  is_default: boolean
  context_window?: number
  supports_thinking?: boolean
  thinking_levels?: string[]
  thinking_level?: string
  enabled: boolean
  created_at?: string
  updated_at?: string
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
  color_mode: 'default' | 'project' | 'custom'
  custom_color: string
}

export interface EnhancementsConfig {
  version: string
  enabled: boolean
  prompt_jump_bar: PromptJumpBarConfig
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
  path: string
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
  active_conversation_indicator: 'background' | 'border'
  active_conversation_bold: boolean
  project_colors: Record<string, string>
  drag_rearrange_enabled: boolean
  project_order: string[]
  archived_projects: string[]
  conversation_tabs_mode: 'fixed' | 'dynamic'
  conversation_tabs_fixed_limit: number
  conversation_tabs_age_threshold: '1d' | '3d' | '7d'
  conversation_tabs_min: number
  conversation_tabs_max: number
  auto_archive_conversations: boolean
  auto_archive_horizon: '7d' | '14d' | '30d' | '60d' | '90d'
  auto_inject: boolean
}


