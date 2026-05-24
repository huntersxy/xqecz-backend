package handlers

import (
	"encoding/hex"
	"testing"
)

func TestGenerateSessionID(t *testing.T) {
	t.Run("valid_length", func(t *testing.T) {
		id := generateSessionID()
		if len(id) != SessionIDLength*2 {
			t.Errorf("generateSessionID length = %d, want %d", len(id), SessionIDLength*2)
		}
	})

	t.Run("is_hex", func(t *testing.T) {
		id := generateSessionID()
		_, err := hex.DecodeString(id)
		if err != nil {
			t.Errorf("generateSessionID produced non-hex string: %v", err)
		}
	})

	t.Run("non_empty", func(t *testing.T) {
		id := generateSessionID()
		if id == "" {
			t.Error("generateSessionID returned empty string")
		}
	})

	t.Run("uniqueness", func(t *testing.T) {
		seen := make(map[string]bool)
		for i := 0; i < 100; i++ {
			id := generateSessionID()
			if seen[id] {
				t.Fatal("generateSessionID produced duplicate value")
			}
			seen[id] = true
		}
	})
}

func TestSessionConstants(t *testing.T) {
	if SessionIDLength != 32 {
		t.Errorf("SessionIDLength = %d, want 32", SessionIDLength)
	}
	if CookieMaxAge != 3600*24*30 {
		t.Errorf("CookieMaxAge = %d, want %d", CookieMaxAge, 3600*24*30)
	}
}
