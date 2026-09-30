package main

import (
	"net/http"
	"net/url"
	"strings"
)

// contentSecurityPolicy builds the CSP of every response. Scripts, fonts and
// frames come only from this origin; styles also allow inline declarations
// (Vuetify injects its theme and index.html declares the cascade layers). In jwt
// mode the SPA also calls the auth server (token mint, logout), so its origin is
// allowed in connect-src.
func contentSecurityPolicy(config serverConfig) string {
	connect := "'self'"
	if origin := authOrigin(config); origin != "" {
		connect += " " + origin
	}
	return strings.Join([]string{
		"default-src 'self'",
		"script-src 'self'",
		"style-src 'self' 'unsafe-inline'",
		"img-src 'self' data: blob:",
		"font-src 'self' data:",
		"connect-src " + connect,
		"object-src 'none'",
		"base-uri 'self'",
		"form-action 'self'",
		"frame-ancestors 'none'",
	}, "; ")
}

// authOrigin returns the scheme://host of the auth server in jwt mode, or ""
// when the SPA never calls it (dev mode) or the URL is not absolute.
func authOrigin(config serverConfig) string {
	if config.AuthMode != "jwt" {
		return ""
	}
	parsed, err := url.Parse(config.AuthServerURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	return parsed.Scheme + "://" + parsed.Host
}

// securityHeadersMiddleware sets the browser security headers on every response:
// the CSP, no MIME sniffing (uploaded blobs are served back), no framing, a
// referrer limited to the origin across sites, an isolated browsing context and
// no powerful features. HSTS is left to the TLS terminator (ingress), which alone
// knows whether the connection is HTTPS.
func securityHeadersMiddleware(csp string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		header := writer.Header()
		header.Set("Content-Security-Policy", csp)
		header.Set("X-Content-Type-Options", "nosniff")
		header.Set("X-Frame-Options", "DENY")
		header.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		header.Set("Cross-Origin-Opener-Policy", "same-origin")
		header.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=()")
		next.ServeHTTP(writer, request)
	})
}
