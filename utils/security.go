package utils

import (
	"html"
	"strings"
)

func SanitizeHTML(input string) string {
	if input == "" {
		return input
	}
	return html.EscapeString(input)
}

func ValidateContentTitle(title string) bool {
	title = strings.TrimSpace(title)
	return len(title) >= 1 && len(title) <= 200
}

func ValidateTextContent(content string) bool {
	return len(content) <= 10000
}

func ValidateUsername(username string) bool {
	username = strings.TrimSpace(username)
	if len(username) < 3 || len(username) > 50 {
		return false
	}
	return true
}

func ValidatePassword(password string) bool {
	return len(password) >= 6
}
