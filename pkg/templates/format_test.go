package templates

import (
	"testing"
)

func TestFormatTime(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"08:00", "8:00 AM"},
		{"00:00", "12:00 AM"},
		{"12:00", "12:00 PM"},
		{"17:00", "5:00 PM"},
		{"18:30", "6:30 PM"},
		{"07:05", "7:05 AM"},
		{"23:59", "11:59 PM"},
	}

	for _, tt := range tests {
		got := FormatTime(tt.input)
		if got != tt.expected {
			t.Errorf("FormatTime(%q) = %q, expected %q", tt.input, got, tt.expected)
		}
	}
}

func TestCronToHuman(t *testing.T) {
	tests := []struct {
		cron     string
		expected string
	}{
		{"0 8 * * *", "Every day at 8:00 AM"},
		{"0 17 * * 1-5", "Weekdays at 5:00 PM"},
		{"30 7 * * 1-5", "Weekdays at 7:30 AM"},
		{"0 18 * * 1-5", "Weekdays at 6:00 PM"},
		{"0 * * * *", "Every hour"},
		{"*/15 * * * *", "Every 15 minutes"},
		{"0 */2 * * *", "Every 2 hours"},
		{"0 9 * * 1", "Every Monday at 9:00 AM"},
		{"0 17 * * 5", "Every Friday at 5:00 PM"},
		{"0 10 * * 3", "Every Wednesday at 10:00 AM"},
		{"0 2 * * *", "Every day at 2:00 AM"},
		{"0 11 * * 2", "Every Tuesday at 11:00 AM"},
		{"30 8 * * 1", "Every Monday at 8:30 AM"},
		{"0 9 * * 4", "Every Thursday at 9:00 AM"},
		{"0 8 * * 0,6", "Weekends at 8:00 AM"},
		{"0 8 1 * *", "1st of every month at 8:00 AM"},
		{"0 8 15 * *", "15th of every month at 8:00 AM"},
	}

	for _, tt := range tests {
		got := CronToHuman(tt.cron)
		if got != tt.expected {
			t.Errorf("CronToHuman(%q) = %q, expected %q", tt.cron, got, tt.expected)
		}
	}
}

func TestFormatSchedule(t *testing.T) {
	sched := TemplateSchedule{
		Frequency:      "daily",
		TimeOfDay:      "08:00",
		DaysOfWeek:     []int{1, 2, 3, 4, 5, 6, 7},
		CronExpression: "0 8 * * *",
	}
	got := FormatSchedule(sched)
	if got != "Every day at 8:00 AM" {
		t.Errorf("FormatSchedule() = %q, expected 'Every day at 8:00 AM'", got)
	}

	schedWeekday := TemplateSchedule{
		Frequency:      "daily",
		TimeOfDay:      "17:00",
		DaysOfWeek:     []int{1, 2, 3, 4, 5},
		CronExpression: "0 17 * * 1-5",
	}
	gotWeekday := FormatSchedule(schedWeekday)
	if gotWeekday != "Weekdays at 5:00 PM" {
		t.Errorf("FormatSchedule() = %q, expected 'Weekdays at 5:00 PM'", gotWeekday)
	}
}
