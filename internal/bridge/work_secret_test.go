package bridge

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

func TestEncodeDecodeWorkSecret(t *testing.T) {
	secret := WorkSecret{Version: 1, SessionIngressToken: "tok", APIBaseURL: "https://api.example.com"}
	encoded, err := EncodeWorkSecret(secret)
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}
	decoded, err := DecodeWorkSecret(encoded)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if decoded != secret {
		t.Fatalf("decoded mismatch: got %+v want %+v", decoded, secret)
	}
}

func TestDecodeWorkSecretValidation(t *testing.T) {
	t.Run("invalid version", func(t *testing.T) {
		raw, _ := json.Marshal(WorkSecret{Version: 2, SessionIngressToken: "tok", APIBaseURL: "https://api.example.com"})
		encoded := base64.RawURLEncoding.EncodeToString(raw)
		_, err := DecodeWorkSecret(encoded)
		if err == nil {
			t.Fatalf("expected error")
		}
	})
	_, err := DecodeWorkSecret("not-base64")
	if err == nil {
		t.Fatalf("expected decode error")
	}
}

func TestBuildSDKURL(t *testing.T) {
	if got := BuildSDKURL("https://api.example.com", "s1"); got != "wss://api.example.com/v1/session_ingress/ws/s1" {
		t.Fatalf("unexpected remote sdk url: %s", got)
	}
	if got := BuildSDKURL("http://localhost:8080", "s1"); got != "ws://localhost:8080/v2/session_ingress/ws/s1" {
		t.Fatalf("unexpected local sdk url: %s", got)
	}
}
