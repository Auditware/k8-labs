package auth

import (
	"os"
	"testing"
	"time"
)

func TestIssueAndVerifyToken(t *testing.T) {
	t.Setenv("JWT_SIGNING_KEY", "test-signing-key")

	token, err := IssueSessionToken("user-1", "session-1", time.Hour)
	if err != nil {
		t.Fatalf("IssueSessionToken failed: %v", err)
	}

	claims, err := VerifyToken(token)
	if err != nil {
		t.Fatalf("VerifyToken failed: %v", err)
	}
	if claims["sub"] != "user-1" {
		t.Errorf("expected sub=user-1, got %v", claims["sub"])
	}
	if claims["sid"] != "session-1" {
		t.Errorf("expected sid=session-1, got %v", claims["sid"])
	}
}

func TestIssueSessionTokenMissingKey(t *testing.T) {
	os.Unsetenv("JWT_SIGNING_KEY")
	if _, err := IssueSessionToken("user-1", "session-1", time.Hour); err != ErrNoSigningKey {
		t.Errorf("expected ErrNoSigningKey, got %v", err)
	}
}

func TestVerifyTokenRejectsGarbage(t *testing.T) {
	t.Setenv("JWT_SIGNING_KEY", "test-signing-key")
	if _, err := VerifyToken("not-a-real-token"); err == nil {
		t.Error("expected an error for a malformed token, got nil")
	}
}
