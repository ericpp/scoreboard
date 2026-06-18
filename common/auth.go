package common

import (
	"net/http"
	"strings"
)

func ValidateBearerToken(authHeader, expectedToken string) (status int, ok bool) {
	if authHeader == "" {
		return http.StatusUnauthorized, false
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		return http.StatusUnauthorized, false
	}

	token := parts[1]
	if expectedToken == "" {
		return http.StatusInternalServerError, false
	}

	if token != expectedToken {
		return http.StatusUnauthorized, false
	}

	return http.StatusOK, true
}

func ValidateHelipadToken(authHeader, expectedToken string) (status int, ok bool) {
	if authHeader == "" {
		return http.StatusUnauthorized, false
	}

	token := strings.Replace(authHeader, "Bearer ", "", 1)
	if token == "" || expectedToken == "" || token != expectedToken {
		return http.StatusForbidden, false
	}

	return http.StatusOK, true
}
