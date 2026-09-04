package services

import (
	"fmt"
	"strings"
)

func validatePriority(priority string) error {
	switch priority {
	case "low", "medium", "high", "critical":
		return nil
	}
	return fmt.Errorf("VALIDATION: invalid priority %q", priority)
}

// normalizeColor applies fallback on empty input and checks "#" + 6 characters;
// it does not verify hex digits. The fallback is also the example in the error.
func normalizeColor(color, fallback string) (string, error) {
	if color == "" {
		color = fallback
	}
	if !strings.HasPrefix(color, "#") || len(color) != 7 {
		return "", fmt.Errorf("VALIDATION: color must be a 7-character hex color (e.g. %s)", fallback)
	}
	return color, nil
}
