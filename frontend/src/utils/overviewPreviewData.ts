export const OVERVIEW_PREVIEW_HEADER = 'Overview Panel Preview'

export const OVERVIEW_DIVISION_BADGE_LABELS: Record<'divider_line' | 'border_zone', string> = {
  divider_line: 'Divider Lines',
  border_zone: 'Border Zones',
}

export interface OverviewFileItem {
  name: string
  extBadge?: string
  extColor?: string
  directory?: string
}

export interface OverviewUploadItem {
  name: string
  timestamp: string
  label: string
}

export interface OverviewGoalItem {
  text: string
}

export interface OverviewSkillItem {
  name: string
  path: string
}

export interface OverviewSubagentInfo {
  title: string
  duration: string
}

export interface OverviewArtifactInfo {
  title: string
}
