package bridge

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

// WorkSecret mirrors Python bridge work secret payload.
type WorkSecret struct {
	Version             int    `json:"version"`
	SessionIngressToken string `json:"session_ingress_token"`
	APIBaseURL          string `json:"api_base_url"`
}

// EncodeWorkSecret encodes a work secret as base64url JSON without padding.
func EncodeWorkSecret(secret WorkSecret) (string, error) {
	data, err := json.Marshal(secret)
	if err != nil {
		return "", err
	}
	encoded := base64.RawURLEncoding.EncodeToString(data)
	return encoded, nil
}

// DecodeWorkSecret decodes and validates a work secret.
func DecodeWorkSecret(raw string) (WorkSecret, error) {
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimSpace(raw))
	if err != nil {
		return WorkSecret{}, err
	}
	var secret WorkSecret
	if err := json.Unmarshal(payload, &secret); err != nil {
		return WorkSecret{}, err
	}
	if secret.Version != 1 {
		return WorkSecret{}, fmt.Errorf("Unsupported work secret version: %d", secret.Version)
	}
	if strings.TrimSpace(secret.SessionIngressToken) == "" {
		return WorkSecret{}, fmt.Errorf("Invalid work secret: missing session_ingress_token")
	}
	if strings.TrimSpace(secret.APIBaseURL) == "" {
		return WorkSecret{}, fmt.Errorf("Invalid work secret: missing api_base_url")
	}
	return secret, nil
}

// BuildSDKURL builds a session ingress websocket URL.
func BuildSDKURL(apiBaseURL, sessionID string) string {
	base := strings.TrimSpace(apiBaseURL)
	sid := strings.TrimSpace(sessionID)
	isLocal := strings.Contains(base, "localhost") || strings.Contains(base, "127.0.0.1")
	protocol := "wss"
	version := "v1"
	if isLocal {
		protocol = "ws"
		version = "v2"
	}
	host := strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(base, "https://"), "http://"), "/")
	return fmt.Sprintf("%s://%s/%s/session_ingress/ws/%s", protocol, host, version, sid)
}
