package auth

import (
	"testing"
	"time"
)

func TestIssueAITokenCarriesScopeAndBindings(t *testing.T) {
	t.Setenv("JWT_SIGNING_KEY", "test-signing-key")

	token, err := IssueAIToken("user-1", "session-1", "jti-1", time.Hour)
	if err != nil {
		t.Fatalf("IssueAIToken failed: %v", err)
	}
	claims, err := VerifyToken(token)
	if err != nil {
		t.Fatalf("VerifyToken failed: %v", err)
	}
	if got := Scope(claims); got != ScopeAI {
		t.Fatalf("scope = %q, want %q", got, ScopeAI)
	}
	if claims["sid"] != "session-1" {
		t.Fatalf("sid = %v, want session-1", claims["sid"])
	}
	if claims["jti"] != "jti-1" {
		t.Fatalf("jti = %v, want jti-1", claims["jti"])
	}
	if claims["sub"] != "user-1" {
		t.Fatalf("sub = %v, want user-1", claims["sub"])
	}
}

func TestScopeEmptyForNonAIToken(t *testing.T) {
	t.Setenv("JWT_SIGNING_KEY", "test-signing-key")

	token, err := IssueLoginToken("user-1", "u@example.com", time.Hour)
	if err != nil {
		t.Fatalf("IssueLoginToken failed: %v", err)
	}
	claims, err := VerifyToken(token)
	if err != nil {
		t.Fatalf("VerifyToken failed: %v", err)
	}
	if got := Scope(claims); got != "" {
		t.Fatalf("scope = %q, want empty for a login token", got)
	}
}
