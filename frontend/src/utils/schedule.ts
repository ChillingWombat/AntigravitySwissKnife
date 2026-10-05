import type { TemplateSchedule } from '../types'

/**
 * Format 24-hour "HH:MM" into ordinary 12-hour civilian time (e.g. "8:00 AM", "5:00 PM", "12:00 AM").
 */
export function formatTime(timeOfDay: string): string {
  if (!timeOfDay) return ''
  const parts = timeOfDay.trim().split(':')
  if (parts.length !== 2) return timeOfDay

  const h = parseInt(parts[0], 10)
  const m = parseInt(parts[1], 10)
  if (isNaN(h) || isNaN(m)) return timeOfDay

  const period = h >= 12 ? 'PM' : 'AM'
  const displayH = h === 0 ? 12 : h > 12 ? h - 12 : h
  const displayM = m < 10 ? `0${m}` : `${m}`
  return `${displayH}:${displayM} ${period}`
}

function getOrdinal(day: number): string {
  if (day >= 11 && day <= 13) {
    return `${day}th`
  }
  switch (day % 10) {
    case 1:
      return `${day}st`
    case 2:
      return `${day}nd`
    case 3:
      return `${day}rd`
    default:
      return `${day}th`
  }
}

/**
 * Converts a cron expression into an ordinary time and day text format.
 * Examples:
 * - "0 8 * * *"    => "Every day at 8:00 AM"
 * - "0 17 * * 1-5"  => "Weekdays at 5:00 PM"
 * - "30 7 * * 1-5"  => "Weekdays at 7:30 AM"
 * - "0 * * * *"     => "Every hour"
 * - "0 9 * * 1"     => "Every Monday at 9:00 AM"
 * - "* /15 * * * *" => "Every 15 minutes"
 */
export function formatCronToHuman(cron: string): string {
  const expr = (cron || '').trim()
  if (!expr) return ''

  const parts = expr.split(/\s+/)
  if (parts.length !== 5) return expr

  const [minStr, hourStr, domStr, , dowStr] = parts

  // 1. Minute intervals: "*/15 * * * *"
  if (hourStr === '*' && domStr === '*' && dowStr === '*') {
    if (minStr === '0' || minStr === '*') {
      return 'Every hour'
    }
    if (minStr.startsWith('*/')) {
      return `Every ${minStr.slice(2)} minutes`
    }
    const m = parseInt(minStr, 10)
    if (!isNaN(m)) {
      return `Hourly at :${m < 10 ? '0' + m : m}`
    }
  }

  // 2. Hour intervals: "0 */2 * * *"
  if (hourStr.startsWith('*/') && domStr === '*' && dowStr === '*') {
    return `Every ${hourStr.slice(2)} hours`
  }

  // Parse hour & minute
  const h = parseInt(hourStr, 10)
  const m = parseInt(minStr, 10)
  if (isNaN(h) || isNaN(m)) {
    return expr
  }

  const timeText = formatTime(`${h < 10 ? '0' + h : h}:${m < 10 ? '0' + m : m}`)

  // Day of Month specified: e.g. "0 8 1 * *"
  if (domStr !== '*') {
    const d = parseInt(domStr, 10)
    if (!isNaN(d)) {
      return `${getOrdinal(d)} of every month at ${timeText}`
    }
    return `Day ${domStr} of every month at ${timeText}`
  }

  // Day of Week specified
  const normDow = dowStr.toUpperCase()
  switch (normDow) {
    case '*':
      return `Every day at ${timeText}`
    case '1-5':
    case '1,2,3,4,5':
    case 'MON-FRI':
      return `Weekdays at ${timeText}`
    case '0,6':
    case '6,0':
    case '6-7':
    case 'SAT,SUN':
      return `Weekends at ${timeText}`
    case '1':
    case 'MON':
      return `Every Monday at ${timeText}`
    case '2':
    case 'TUE':
      return `Every Tuesday at ${timeText}`
    case '3':
    case 'WED':
      return `Every Wednesday at ${timeText}`
    case '4':
    case 'THU':
      return `Every Thursday at ${timeText}`
    case '5':
    case 'FRI':
      return `Every Friday at ${timeText}`
    case '6':
    case 'SAT':
      return `Every Saturday at ${timeText}`
    case '0':
    case '7':
    case 'SUN':
      return `Every Sunday at ${timeText}`
    case '1,3,5':
      return `Mon, Wed, Fri at ${timeText}`
    case '2,4':
      return `Tue, Thu at ${timeText}`
    default:
      return `Days (${dowStr}) at ${timeText}`
  }
}

/**
 * Format a template schedule or cron string into ordinary time and day text.
 */
export function formatSchedule(scheduleOrCron?: Partial<TemplateSchedule> | string | null): string {
  if (!scheduleOrCron) return ''

  if (typeof scheduleOrCron === 'string') {
    return formatCronToHuman(scheduleOrCron)
  }

  if (scheduleOrCron.schedule_text) {
    return scheduleOrCron.schedule_text
  }

  if (scheduleOrCron.cron_expression) {
    return formatCronToHuman(scheduleOrCron.cron_expression)
  }

  if (scheduleOrCron.frequency === 'hourly') {
    return 'Every hour'
  }

  const timeText = scheduleOrCron.time_of_day ? formatTime(scheduleOrCron.time_of_day) : ''
  const days = scheduleOrCron.days_of_week || []

  if (days.length === 5 && days[0] === 1 && days[4] === 5) {
    return timeText ? `Weekdays at ${timeText}` : 'Weekdays'
  }

  if (days.length === 7 || days.length === 0) {
    return timeText ? `Every day at ${timeText}` : 'Every day'
  }

  if (days.length === 1) {
    const dayNames: Record<number, string> = {
      1: 'Monday',
      2: 'Tuesday',
      3: 'Wednesday',
      4: 'Thursday',
      5: 'Friday',
      6: 'Saturday',
      7: 'Sunday',
      0: 'Sunday',
    }
    const dayName = dayNames[days[0]]
    if (dayName) {
      return timeText ? `Every ${dayName} at ${timeText}` : `Every ${dayName}`
    }
  }

  return timeText ? `${scheduleOrCron.frequency || 'Custom'} at ${timeText}` : scheduleOrCron.frequency || 'Custom'
}
