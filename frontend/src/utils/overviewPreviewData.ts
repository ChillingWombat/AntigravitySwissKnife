export const OVERVIEW_PREVIEW_HEADER = 'Overview Panel Preview'

export const OVERVIEW_DIVISION_BADGE_LABELS: Record<'divider_line' | 'border_zone', string> = {
  divider_line: 'Divider Lines',
  border_zone: 'Border Zones',
}

export const DEMO_PROJECT_NAME = 'Demo Project'

export interface SyntheticFileItem {
  name: string
  extBadge: string
  extColor: string
  directory?: string
}

export interface SyntheticUploadItem {
  name: string
  timestamp: string
  label: string
}

export interface SyntheticGoalItem {
  text: string
}

export interface SyntheticSkillItem {
  name: string
  path: string
}

export interface SyntheticSubagentInfo {
  title: string
  duration: string
}

export interface SyntheticArtifactInfo {
  title: string
}

export const SYNTHETIC_SUBAGENTS_COUNT = 1
export const SYNTHETIC_SUBAGENT: SyntheticSubagentInfo = {
  title: 'Codebase Architecture Reviewer (2 subagents)',
  duration: 'Worked for 12m',
}

export const SYNTHETIC_FILES_COUNT = 18
export const SYNTHETIC_FILES_TAG = 'Uncommitted'

export const SYNTHETIC_VISIBLE_FILES: SyntheticFileItem[] = [
  { name: 'README.md', extBadge: 'M↓', extColor: '#0284c7' },
  { name: 'DashboardView.tsx', extBadge: 'TSX', extColor: '#6366f1', directory: 'src/components' },
  { name: 'apiClient.ts', extBadge: 'TS', extColor: '#3b82f6', directory: 'src/api' },
  { name: 'auth_service.go', extBadge: 'Go', extColor: '#059669', directory: 'pkg/auth' },
  { name: 'UserSettingsModal.tsx', extBadge: 'TSX', extColor: '#6366f1', directory: 'src/components' },
]

export const SYNTHETIC_EXPANDED_FILES: SyntheticFileItem[] = [
  { name: 'session_handler.go', extBadge: 'Go', extColor: '#059669', directory: 'pkg/handlers' },
  { name: 'models.go', extBadge: 'Go', extColor: '#059669', directory: 'pkg/models' },
  { name: 'schema.ts', extBadge: 'TS', extColor: '#3b82f6', directory: 'src/types' },
]

export const SYNTHETIC_ARTIFACTS_COUNT = 1
export const SYNTHETIC_ARTIFACT: SyntheticArtifactInfo = {
  title: 'Architecture RFC',
}

export const SYNTHETIC_UPLOADS_COUNT = 8

export const SYNTHETIC_VISIBLE_UPLOADS: SyntheticUploadItem[] = [
  { name: 'architecture-diagram.png', timestamp: 'Today 10:45 AM', label: 'architecture-diagram.png (Today 10:45 AM)' },
  { name: 'wireframe-mockup.png', timestamp: 'Today 10:30 AM', label: 'wireframe-mockup.png (Today 10:30 AM)' },
  { name: 'benchmark-chart.png', timestamp: 'Today 10:15 AM', label: 'benchmark-chart.png (Today 10:15 AM)' },
]

export const SYNTHETIC_EXPANDED_UPLOADS: SyntheticUploadItem[] = [
  { name: 'schema-diagram.png', timestamp: 'Today 09:50 AM', label: 'schema-diagram.png (Today 09:50 AM)' },
  { name: 'component-spec.png', timestamp: 'Today 09:30 AM', label: 'component-spec.png (Today 09:30 AM)' },
]

export const SYNTHETIC_TASKS_COUNT = 0
export const SYNTHETIC_TERMINALS_COUNT = 0

export const SYNTHETIC_GOALS_COUNT = 2
export const SYNTHETIC_GOALS: SyntheticGoalItem[] = [
  { text: 'Implement responsive layout grid for dashboard cards' },
  { text: 'Add unit tests for auth token expiration handling' },
]

export const SYNTHETIC_SKILLS_COUNT = 2
export const SYNTHETIC_SKILLS: SyntheticSkillItem[] = [
  { name: 'code-review', path: '.../skills/code-review' },
  { name: 'tdd', path: '.../skills/tdd' },
]

export const SYNTHETIC_OVERVIEW_FIXTURES = {
  header: OVERVIEW_PREVIEW_HEADER,
  badgeLabels: OVERVIEW_DIVISION_BADGE_LABELS,
  demoProjectName: DEMO_PROJECT_NAME,
  subagent: SYNTHETIC_SUBAGENT,
  subagentsCount: SYNTHETIC_SUBAGENTS_COUNT,
  filesCount: SYNTHETIC_FILES_COUNT,
  filesTag: SYNTHETIC_FILES_TAG,
  visibleFiles: SYNTHETIC_VISIBLE_FILES,
  expandedFiles: SYNTHETIC_EXPANDED_FILES,
  artifact: SYNTHETIC_ARTIFACT,
  artifactsCount: SYNTHETIC_ARTIFACTS_COUNT,
  uploadsCount: SYNTHETIC_UPLOADS_COUNT,
  visibleUploads: SYNTHETIC_VISIBLE_UPLOADS,
  expandedUploads: SYNTHETIC_EXPANDED_UPLOADS,
  tasksCount: SYNTHETIC_TASKS_COUNT,
  terminalsCount: SYNTHETIC_TERMINALS_COUNT,
  goals: SYNTHETIC_GOALS,
  goalsCount: SYNTHETIC_GOALS_COUNT,
  skills: SYNTHETIC_SKILLS,
  skillsCount: SYNTHETIC_SKILLS_COUNT,
}
