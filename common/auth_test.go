package common

import (
	"net/http"
	"testing"
)

func TestValidateBearerToken(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		authHeader   string
		expectedToken string
		wantStatus   int
		wantOK       bool
	}{
		{
			name:          "valid token",
			authHeader:    "Bearer secret-token",
			expectedToken: "secret-token",
			wantStatus:    http.StatusOK,
			wantOK:        true,
		},
		{
			name:          "case insensitive bearer",
			authHeader:    "bearer secret-token",
			expectedToken: "secret-token",
			wantStatus:    http.StatusOK,
			wantOK:        true,
		},
		{
			name:          "missing header",
			authHeader:    "",
			expectedToken: "secret-token",
			wantStatus:    http.StatusUnauthorized,
			wantOK:        false,
		},
		{
			name:          "invalid format",
			authHeader:    "Token secret-token",
			expectedToken: "secret-token",
			wantStatus:    http.StatusUnauthorized,
			wantOK:        false,
		},
		{
			name:          "wrong token",
			authHeader:    "Bearer wrong",
			expectedToken: "secret-token",
			wantStatus:    http.StatusUnauthorized,
			wantOK:        false,
		},
		{
			name:          "missing expected token",
			authHeader:    "Bearer secret-token",
			expectedToken: "",
			wantStatus:    http.StatusInternalServerError,
			wantOK:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			status, ok := ValidateBearerToken(tt.authHeader, tt.expectedToken)
			if status != tt.wantStatus {
				t.Errorf("status = %d, want %d", status, tt.wantStatus)
			}
			if ok != tt.wantOK {
				t.Errorf("ok = %v, want %v", ok, tt.wantOK)
			}
		})
	}
}

func TestValidateHelipadToken(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		authHeader    string
		expectedToken string
		wantStatus    int
		wantOK        bool
	}{
		{
			name:          "valid token",
			authHeader:    "Bearer helipad-secret",
			expectedToken: "helipad-secret",
			wantStatus:    http.StatusOK,
			wantOK:        true,
		},
		{
			name:          "missing header",
			authHeader:    "",
			expectedToken: "helipad-secret",
			wantStatus:    http.StatusUnauthorized,
			wantOK:        false,
		},
		{
			name:          "wrong token",
			authHeader:    "Bearer wrong",
			expectedToken: "helipad-secret",
			wantStatus:    http.StatusForbidden,
			wantOK:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			status, ok := ValidateHelipadToken(tt.authHeader, tt.expectedToken)
			if status != tt.wantStatus {
				t.Errorf("status = %d, want %d", status, tt.wantStatus)
			}
			if ok != tt.wantOK {
				t.Errorf("ok = %v, want %v", ok, tt.wantOK)
			}
		})
	}
}
