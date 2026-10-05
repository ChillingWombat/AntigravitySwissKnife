import { describe, it } from 'node:test'
import assert from 'node:assert/strict'
import { formatTime, formatCronToHuman, formatSchedule } from './schedule.ts'

describe('schedule utility', () => {
  describe('formatTime', () => {
    it('formats times into ordinary 12-hour AM/PM format', () => {
      assert.equal(formatTime('08:00'), '8:00 AM')
      assert.equal(formatTime('00:00'), '12:00 AM')
      assert.equal(formatTime('12:00'), '12:00 PM')
      assert.equal(formatTime('17:00'), '5:00 PM')
      assert.equal(formatTime('18:30'), '6:30 PM')
      assert.equal(formatTime('07:05'), '7:05 AM')
    })
  })

  describe('formatCronToHuman', () => {
    it('formats daily cron to ordinary time day text format', () => {
      assert.equal(formatCronToHuman('0 8 * * *'), 'Every day at 8:00 AM')
      assert.equal(formatCronToHuman('0 2 * * *'), 'Every day at 2:00 AM')
    })

    it('formats weekdays cron', () => {
      assert.equal(formatCronToHuman('0 17 * * 1-5'), 'Weekdays at 5:00 PM')
      assert.equal(formatCronToHuman('30 7 * * 1-5'), 'Weekdays at 7:30 AM')
      assert.equal(formatCronToHuman('0 18 * * 1-5'), 'Weekdays at 6:00 PM')
    })

    it('formats hourly cron', () => {
      assert.equal(formatCronToHuman('0 * * * *'), 'Every hour')
      assert.equal(formatCronToHuman('*/15 * * * *'), 'Every 15 minutes')
      assert.equal(formatCronToHuman('0 */2 * * *'), 'Every 2 hours')
    })

    it('formats specific days of week', () => {
      assert.equal(formatCronToHuman('0 9 * * 1'), 'Every Monday at 9:00 AM')
      assert.equal(formatCronToHuman('0 11 * * 2'), 'Every Tuesday at 11:00 AM')
      assert.equal(formatCronToHuman('0 10 * * 3'), 'Every Wednesday at 10:00 AM')
      assert.equal(formatCronToHuman('0 9 * * 4'), 'Every Thursday at 9:00 AM')
      assert.equal(formatCronToHuman('0 17 * * 5'), 'Every Friday at 5:00 PM')
      assert.equal(formatCronToHuman('30 8 * * 1'), 'Every Monday at 8:30 AM')
      assert.equal(formatCronToHuman('0 8 * * 0,6'), 'Weekends at 8:00 AM')
    })

    it('formats day of month', () => {
      assert.equal(formatCronToHuman('0 8 1 * *'), '1st of every month at 8:00 AM')
      assert.equal(formatCronToHuman('0 8 15 * *'), '15th of every month at 8:00 AM')
    })
  })

  describe('formatSchedule', () => {
    it('formats TemplateSchedule objects', () => {
      assert.equal(
        formatSchedule({
          frequency: 'daily',
          time_of_day: '08:00',
          days_of_week: [1, 2, 3, 4, 5, 6, 7],
          cron_expression: '0 8 * * *',
        }),
        'Every day at 8:00 AM'
      )

      assert.equal(
        formatSchedule({
          frequency: 'daily',
          time_of_day: '17:00',
          days_of_week: [1, 2, 3, 4, 5],
          cron_expression: '0 17 * * 1-5',
        }),
        'Weekdays at 5:00 PM'
      )

      assert.equal(
        formatSchedule({
          frequency: 'hourly',
          time_of_day: '00:00',
          days_of_week: [1, 2, 3, 4, 5, 6, 7],
          cron_expression: '0 * * * *',
        }),
        'Every hour'
      )

      assert.equal(
        formatSchedule({
          frequency: 'weekly',
          time_of_day: '09:00',
          days_of_week: [1],
          cron_expression: '0 9 * * 1',
        }),
        'Every Monday at 9:00 AM'
      )
    })

    it('prefers schedule_text if provided', () => {
      assert.equal(
        formatSchedule({
          schedule_text: 'Custom text at 10:00 AM',
          cron_expression: '0 10 * * *',
        }),
        'Custom text at 10:00 AM'
      )
    })
  })
})
