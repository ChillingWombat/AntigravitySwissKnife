package templates

import (
	"fmt"
	"strconv"
	"strings"
)

// FormatTime converts 24h "HH:MM" into ordinary 12h format e.g. "8:00 AM", "5:30 PM", "12:00 AM".
func FormatTime(timeOfDay string) string {
	parts := strings.Split(strings.TrimSpace(timeOfDay), ":")
	if len(parts) != 2 {
		return timeOfDay
	}
	h, errH := strconv.Atoi(parts[0])
	m, errM := strconv.Atoi(parts[1])
	if errH != nil || errM != nil {
		return timeOfDay
	}
	period := "AM"
	if h >= 12 {
		period = "PM"
	}
	displayH := h
	if h == 0 {
		displayH = 12
	} else if h > 12 {
		displayH = h - 12
	}
	return fmt.Sprintf("%d:%02d %s", displayH, m, period)
}

func getOrdinalSuffix(day int) string {
	if day >= 11 && day <= 13 {
		return fmt.Sprintf("%dth", day)
	}
	switch day % 10 {
	case 1:
		return fmt.Sprintf("%dst", day)
	case 2:
		return fmt.Sprintf("%dnd", day)
	case 3:
		return fmt.Sprintf("%drd", day)
	default:
		return fmt.Sprintf("%dth", day)
	}
}

// CronToHuman converts a standard cron expression into ordinary time & day text format.
// Examples:
// - "0 8 * * *" -> "Every day at 8:00 AM"
// - "0 17 * * 1-5" -> "Weekdays at 5:00 PM"
// - "30 7 * * 1-5" -> "Weekdays at 7:30 AM"
// - "0 * * * *" -> "Every hour"
// - "0 9 * * 1" -> "Every Monday at 9:00 AM"
// - "*/15 * * * *" -> "Every 15 minutes"
func CronToHuman(cron string) string {
	expr := strings.TrimSpace(cron)
	if expr == "" {
		return ""
	}
	parts := strings.Fields(expr)
	if len(parts) != 5 {
		return expr
	}

	minStr, hourStr, domStr, _, dowStr := parts[0], parts[1], parts[2], parts[3], parts[4]

	// 1. Minute intervals: "*/15 * * * *"
	if hourStr == "*" && domStr == "*" && dowStr == "*" {
		if minStr == "0" || minStr == "*" {
			return "Every hour"
		}
		if strings.HasPrefix(minStr, "*/") {
			return fmt.Sprintf("Every %s minutes", minStr[2:])
		}
		if m, err := strconv.Atoi(minStr); err == nil {
			return fmt.Sprintf("Hourly at :%02d", m)
		}
	}

	// 2. Hour intervals: "0 */2 * * *"
	if strings.HasPrefix(hourStr, "*/") && domStr == "*" && dowStr == "*" {
		return fmt.Sprintf("Every %s hours", hourStr[2:])
	}

	// Specific hour & minute
	h, errH := strconv.Atoi(hourStr)
	m, errM := strconv.Atoi(minStr)
	if errH != nil || errM != nil {
		return expr
	}

	timeText := FormatTime(fmt.Sprintf("%02d:%02d", h, m))

	// Day of Month specified: e.g. "0 8 1 * *"
	if domStr != "*" {
		if d, err := strconv.Atoi(domStr); err == nil {
			return fmt.Sprintf("%s of every month at %s", getOrdinalSuffix(d), timeText)
		}
		return fmt.Sprintf("Day %s of every month at %s", domStr, timeText)
	}

	// Day of Week specified
	switch strings.ToUpper(dowStr) {
	case "*":
		return fmt.Sprintf("Every day at %s", timeText)
	case "1-5", "1,2,3,4,5", "MON-FRI":
		return fmt.Sprintf("Weekdays at %s", timeText)
	case "0,6", "6,0", "6-7", "SAT,SUN":
		return fmt.Sprintf("Weekends at %s", timeText)
	case "1", "MON":
		return fmt.Sprintf("Every Monday at %s", timeText)
	case "2", "TUE":
		return fmt.Sprintf("Every Tuesday at %s", timeText)
	case "3", "WED":
		return fmt.Sprintf("Every Wednesday at %s", timeText)
	case "4", "THU":
		return fmt.Sprintf("Every Thursday at %s", timeText)
	case "5", "FRI":
		return fmt.Sprintf("Every Friday at %s", timeText)
	case "6", "SAT":
		return fmt.Sprintf("Every Saturday at %s", timeText)
	case "0", "7", "SUN":
		return fmt.Sprintf("Every Sunday at %s", timeText)
	case "1,3,5":
		return fmt.Sprintf("Mon, Wed, Fri at %s", timeText)
	case "2,4":
		return fmt.Sprintf("Tue, Thu at %s", timeText)
	default:
		return fmt.Sprintf("Days (%s) at %s", dowStr, timeText)
	}
}

// FormatSchedule formats a TemplateSchedule into ordinary time day text format.
func FormatSchedule(s TemplateSchedule) string {
	if s.ScheduleText != "" {
		return s.ScheduleText
	}
	if s.CronExpression != "" {
		return CronToHuman(s.CronExpression)
	}
	if s.Frequency == "hourly" {
		return "Every hour"
	}
	timeText := FormatTime(s.TimeOfDay)
	if len(s.DaysOfWeek) == 5 && s.DaysOfWeek[0] == 1 && s.DaysOfWeek[4] == 5 {
		return fmt.Sprintf("Weekdays at %s", timeText)
	}
	if len(s.DaysOfWeek) == 7 || len(s.DaysOfWeek) == 0 {
		return fmt.Sprintf("Every day at %s", timeText)
	}
	if len(s.DaysOfWeek) == 1 {
		dayNames := map[int]string{
			1: "Monday", 2: "Tuesday", 3: "Wednesday", 4: "Thursday",
			5: "Friday", 6: "Saturday", 7: "Sunday", 0: "Sunday",
		}
		if name, ok := dayNames[s.DaysOfWeek[0]]; ok {
			return fmt.Sprintf("Every %s at %s", name, timeText)
		}
	}
	return fmt.Sprintf("%s at %s", s.Frequency, timeText)
}
