package httpapi

import (
	"github.com/tayga/tms/internal/storage"
	"strings"
)

var mailColors = map[string]bool{"red": true, "orange": true, "yellow": true, "green": true, "blue": true, "purple": true}

const mailColorPrefix = "$TaygaColor-"

func withMailColor(flags []string, color string) []string {
	result := make([]string, 0, len(flags)+1)
	for _, flag := range flags {
		if !strings.HasPrefix(strings.ToLower(flag), strings.ToLower(mailColorPrefix)) {
			result = append(result, flag)
		}
	}
	if color != "" {
		result = append(result, mailColorPrefix+color)
	}
	return result
}
func messageColor(flags string) string {
	for _, flag := range storage.ParseFlags(flags) {
		if strings.HasPrefix(flag, mailColorPrefix) {
			color := strings.TrimPrefix(flag, mailColorPrefix)
			if mailColors[color] {
				return color
			}
		}
	}
	return ""
}
