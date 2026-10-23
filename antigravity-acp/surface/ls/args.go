package ls

import "strings"

// tokenFromArgs extracts --csrf_token from a command line, accepting both the split
// and the inline form.
func tokenFromArgs(args []string) string {
	for i, arg := range args {
		arg = strings.TrimSpace(arg)
		if arg == "--csrf_token" && i+1 < len(args) {
			return strings.TrimSpace(args[i+1])
		}
		if strings.HasPrefix(arg, "--csrf_token=") {
			return strings.TrimSpace(strings.TrimPrefix(arg, "--csrf_token="))
		}
	}
	return ""
}
