package validation

import (
	"net/mail"
	"regexp"
	"strings"
)

var pollID = regexp.MustCompile(`^[a-f0-9]{24}$`)

func Email(value string) bool {
	if len(value) > 254 {
		return false
	}
	address, err := mail.ParseAddress(value)
	if err != nil || address.Address != value || strings.Count(value, "@") != 1 {
		return false
	}
	return strings.Contains(strings.SplitN(value, "@", 2)[1], ".")
}

func PollID(value string) bool { return pollID.MatchString(value) }

func Account(name, email, password string) (string, string, error) {
	name = strings.TrimSpace(name)
	email = strings.ToLower(strings.TrimSpace(email))
	if len([]rune(name)) < 2 || len([]rune(name)) > 80 || !Email(email) || len([]byte(password)) < 10 || len([]byte(password)) > 72 {
		return "", "", ErrInvalidAccount
	}
	return name, email, nil
}

func Credentials(email, password string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if !Email(email) || len([]byte(password)) == 0 || len([]byte(password)) > 72 {
		return "", ErrInvalidCredentials
	}
	return email, nil
}

func Poll(question string, options []string) (string, []string, error) {
	question = strings.TrimSpace(question)
	if len([]rune(question)) < 5 || len([]rune(question)) > 240 || len(options) < 2 || len(options) > 10 {
		return "", nil, ErrInvalidPoll
	}
	seen := make(map[string]struct{}, len(options))
	clean := make([]string, 0, len(options))
	for _, raw := range options {
		option := strings.TrimSpace(raw)
		key := strings.ToLower(option)
		if len([]rune(option)) == 0 || len([]rune(option)) > 100 {
			return "", nil, ErrInvalidOption
		}
		if _, exists := seen[key]; exists {
			return "", nil, ErrDuplicateOption
		}
		seen[key] = struct{}{}
		clean = append(clean, option)
	}
	return question, clean, nil
}

var (
	ErrInvalidAccount     = &FieldError{Message: "name, email, or password does not meet requirements"}
	ErrInvalidCredentials = &FieldError{Message: "invalid email or password"}
	ErrInvalidPoll        = &FieldError{Message: "question must be 5–240 characters and include 2–10 options"}
	ErrInvalidOption      = &FieldError{Message: "options must be 1–100 characters"}
	ErrDuplicateOption    = &FieldError{Message: "answer options must be distinct"}
)

type FieldError struct{ Message string }

func (e *FieldError) Error() string { return e.Message }
