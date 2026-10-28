package config

import (
	"net/url"
	"os"
	"strings"
)

type Config struct {
	MongoURI, MongoDB, RedisURL, CookieDomain, PublicOrigin string
	AllowedOrigins                                          []string
	SecureCookies                                           bool
}

func Load() Config {
	publicOrigin := strings.TrimRight(env("PUBLIC_ORIGIN", "http://localhost:3000"), "/")
	return Config{
		MongoURI:       env("MONGO_URI", "mongodb://localhost:27017"),
		MongoDB:        env("MONGO_DB", "livepoll"),
		RedisURL:       env("REDIS_URL", "redis://localhost:6379/0"),
		CookieDomain:   os.Getenv("COOKIE_DOMAIN"),
		PublicOrigin:   publicOrigin,
		AllowedOrigins: parseOrigins(publicOrigin, os.Getenv("ALLOWED_ORIGINS")),
		SecureCookies:  env("COOKIE_SECURE", "false") == "true",
	}
}

func parseOrigins(primary, configured string) []string {
	values := append([]string{primary}, strings.Split(configured, ",")...)
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		origin := strings.TrimRight(strings.TrimSpace(value), "/")
		if !validOrigin(origin) {
			continue
		}
		if _, exists := seen[origin]; exists {
			continue
		}
		seen[origin] = struct{}{}
		result = append(result, origin)
	}
	return result
}

func validOrigin(origin string) bool {
	parsed, err := url.Parse(origin)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != "" && parsed.User == nil && parsed.RawQuery == "" && parsed.Fragment == "" && (parsed.Path == "" || parsed.Path == "/")
}

func (c Config) AllowsOrigin(origin string) bool {
	if origin == "" || !validOrigin(origin) {
		return false
	}
	if origin == c.PublicOrigin {
		return true
	}
	for _, allowed := range c.AllowedOrigins {
		if origin == allowed {
			return true
		}
	}
	return false
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
