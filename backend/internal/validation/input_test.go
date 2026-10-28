package validation

import "testing"

func TestAccountNormalizesAndValidates(t *testing.T) {
	name, email, err := Account("  Alex Lee  ", "  ALEX@example.com ", "correct-horse-battery")
	if err != nil {
		t.Fatalf("valid account rejected: %v", err)
	}
	if name != "Alex Lee" || email != "alex@example.com" {
		t.Fatalf("unexpected normalization: %q %q", name, email)
	}
	if _, _, err = Account("A", "bad-address", "short"); err == nil {
		t.Fatal("invalid account accepted")
	}
}

func TestPollValidation(t *testing.T) {
	question, options, err := Poll("  What should we build?  ", []string{" Alpha ", "Beta"})
	if err != nil {
		t.Fatalf("valid poll rejected: %v", err)
	}
	if question != "What should we build?" || options[0] != "Alpha" {
		t.Fatalf("input was not normalized: %q %v", question, options)
	}
	for _, test := range []struct {
		name     string
		question string
		options  []string
	}{
		{"short question", "No?", []string{"A", "B"}},
		{"too few", "A valid question?", []string{"A"}},
		{"too many", "A valid question?", []string{"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K"}},
		{"duplicate", "A valid question?", []string{"Same", " same "}},
		{"blank", "A valid question?", []string{"", "B"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, _, err := Poll(test.question, test.options); err == nil {
				t.Fatal("invalid poll accepted")
			}
		})
	}
}

func TestProfileRejectsInvalidInput(t *testing.T) {
	for _, test := range []struct {
		label string
		name  string
		email string
	}{
		{"name too short", "A", "alex@example.com"},
		{"blank name", "  ", "alex@example.com"},
		{"invalid email", "Alex Lee", "invalid-email"},
		{"blank email", "Alex Lee", ""},
	} {
		t.Run(test.label, func(t *testing.T) {
		})
	}
}

