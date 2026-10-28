package config

import "testing"

func TestLoadAllowedOriginsAreExactAndValidated(t *testing.T) {
	t.Setenv("PUBLIC_ORIGIN", "https://poll.example.test/")
	t.Setenv("ALLOWED_ORIGINS", " https://preview.example.test/ , http://localhost:3000,https://poll.example.test,https://evil.test/path")
	config := Load()
	if config.PublicOrigin != "https://poll.example.test" {
		t.Fatalf("unexpected primary origin: %q", config.PublicOrigin)
	}
	for _, allowed := range []string{"https://poll.example.test", "https://preview.example.test", "http://localhost:3000"} {
		if !config.AllowsOrigin(allowed) {
			t.Errorf("expected %q to be allowed", allowed)
		}
	}
	for _, rejected := range []string{"", "https://poll.example.test.evil.test", "https://evil.test/path", "https://poll.example.test.attacker"} {
		if config.AllowsOrigin(rejected) {
			t.Errorf("unexpectedly allowed %q", rejected)
		}
	}
	if len(config.AllowedOrigins) != 3 {
		t.Fatalf("expected 3 unique valid origins, got %v", config.AllowedOrigins)
	}
}
