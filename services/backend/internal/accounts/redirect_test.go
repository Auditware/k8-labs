package accounts

import "testing"

func TestIsAllowedRedirect(t *testing.T) {
	cfg := Config{AllowedOrigin: "https://k8labs.example.com"}

	allowed := []string{
		"http://localhost:7373/login",
		"http://localhost:5173/login",
		"http://localhost:8080/login",
		"http://127.0.0.1:7373/login",
		"http://127.0.0.1:5173/login",
		"https://k8labs.example.com/login",
		"https://k8labs.example.com",
	}
	for _, uri := range allowed {
		if !cfg.IsAllowedRedirect(uri) {
			t.Errorf("expected %q to be allowed", uri)
		}
	}

	blocked := []string{
		"https://evil.com/login",
		"http://localhost:7373.evil.com/login",
		"http://localhost:5173.evil.com/login",
		"https://k8labs.example.com.evil.com/login",
		"http://localhost:9999/login",
		"",
	}
	for _, uri := range blocked {
		if cfg.IsAllowedRedirect(uri) {
			t.Errorf("expected %q to be blocked", uri)
		}
	}
}

func TestIsAllowedRedirectWithoutConfiguredOrigin(t *testing.T) {
	cfg := Config{}
	if !cfg.IsAllowedRedirect("http://localhost:5173/login") {
		t.Error("local dev origins should be allowed even with no AllowedOrigin configured")
	}
	if cfg.IsAllowedRedirect("https://k8labs.example.com/login") {
		t.Error("unconfigured origin should not be allowed")
	}
}
