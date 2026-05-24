package utils

import (
	"strings"
	"testing"
)

func TestValidateUsername(t *testing.T) {
	tests := []struct {
		name     string
		username string
		want     bool
	}{
		{"empty", "", false},
		{"too short (2 chars)", "ab", false},
		{"exactly 3 chars", "abc", true},
		{"normal length", "testuser", true},
		{"50 chars boundary", strings.Repeat("a", 50), true},
		{"51 chars over limit", strings.Repeat("a", 51), false},
		{"with spaces around", "  abc  ", true},
		{"only spaces", "   ", false},
		{"Chinese characters", "用户名", true},
		{"special chars", "user@name", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ValidateUsername(tt.username)
			if got != tt.want {
				t.Errorf("ValidateUsername(%q) = %v, want %v", tt.username, got, tt.want)
			}
		})
	}
}

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		want     bool
	}{
		{"empty", "", false},
		{"5 chars too short", "12345", false},
		{"exactly 6 chars", "123456", true},
		{"long password", strings.Repeat("a", 100), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ValidatePassword(tt.password)
			if got != tt.want {
				t.Errorf("ValidatePassword(%q) = %v, want %v", tt.password, got, tt.want)
			}
		})
	}
}

func TestValidateContentTitle(t *testing.T) {
	tests := []struct {
		name  string
		title string
		want  bool
	}{
		{"empty", "", false},
		{"only spaces", "   ", false},
		{"single char", "a", true},
		{"normal", "Hello World", true},
		{"200 chars boundary", strings.Repeat("a", 200), true},
		{"201 chars over", strings.Repeat("a", 201), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ValidateContentTitle(tt.title)
			if got != tt.want {
				t.Errorf("ValidateContentTitle(%q) = %v, want %v", tt.title, got, tt.want)
			}
		})
	}
}

func TestValidateTextContent(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{"empty", "", true},
		{"normal", "some content", true},
		{"10000 chars boundary", strings.Repeat("a", 10000), true},
		{"10001 chars over", strings.Repeat("a", 10001), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ValidateTextContent(tt.content)
			if got != tt.want {
				t.Errorf("ValidateTextContent(len=%d) = %v, want %v", len(tt.content), got, tt.want)
			}
		})
	}
}

func TestSanitizeHTML(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty", "", ""},
		{"plain text", "hello world", "hello world"},
		{"script tag", "<script>alert('xss')</script>", "&lt;script&gt;alert(&#39;xss&#39;)&lt;/script&gt;"},
		{"ampersand", "a & b", "a &amp; b"},
		{"quote", `"hello"`, "&#34;hello&#34;"},
		{"angle brackets", "<div>", "&lt;div&gt;"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SanitizeHTML(tt.input)
			if got != tt.want {
				t.Errorf("SanitizeHTML(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestGenerateRandomString(t *testing.T) {
	t.Run("valid_length", func(t *testing.T) {
		for _, n := range []int{1, 8, 16, 32, 64} {
			s := GenerateRandomString(n)
			if len(s) != n {
				t.Errorf("GenerateRandomString(%d) length = %d, want %d", n, len(s), n)
			}
			if s == "" && n > 0 {
				t.Errorf("GenerateRandomString(%d) returned empty", n)
			}
		}
	})

	t.Run("uniqueness", func(t *testing.T) {
		seen := make(map[string]bool)
		for i := 0; i < 100; i++ {
			s := GenerateRandomString(16)
			if seen[s] {
				t.Fatal("GenerateRandomString produced duplicate value")
			}
			seen[s] = true
		}
	})

	t.Run("zero_length", func(t *testing.T) {
		s := GenerateRandomString(0)
		if s != "" {
			t.Errorf("GenerateRandomString(0) = %q, want empty", s)
		}
	})
}

func TestRedisKeyConstants(t *testing.T) {
	if redisKeySession != "session:" {
		t.Errorf("redisKeySession = %q, want %q", redisKeySession, "session:")
	}
	if redisKeyViewsDate != "views:date:%s:%d" {
		t.Errorf("redisKeyViewsDate = %q, want %q", redisKeyViewsDate, "views:date:%s:%d")
	}
	if redisKeyRecommendZSet != "recommend:zset" {
		t.Errorf("redisKeyRecommendZSet = %q, want %q", redisKeyRecommendZSet, "recommend:zset")
	}
	if redisKeyRecommendTemp != "recommend:zset:temp" {
		t.Errorf("redisKeyRecommendTemp = %q, want %q", redisKeyRecommendTemp, "recommend:zset:temp")
	}
	if redisKeyUserInfo != "user_info:" {
		t.Errorf("redisKeyUserInfo = %q, want %q", redisKeyUserInfo, "user_info:")
	}
}
