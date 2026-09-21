package main

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/normen/whatscli/messages"
)

// decode reads the single NDJSON line the handler emitted.
func decode(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("saída não é JSON válido (%q): %v", buf.String(), err)
	}
	return out
}

func TestJsonUiSetQRCode(t *testing.T) {
	var buf bytes.Buffer
	j := &JsonUiHandler{enc: json.NewEncoder(&buf)}
	j.SetQRCode(messages.QRCode{
		Event:   messages.QRCodeShow,
		Code:    "pair-code",
		Matrix:  []string{"10", "01"},
		PngPath: "/tmp/qr.png",
		Message: "leia o QR code",
	})
	out := decode(t, &buf)
	if out["type"] != "qr" || out["event"] != "code" {
		t.Fatalf("evento errado: %v", out)
	}
	matrix, ok := out["matrix"].([]any)
	if !ok || len(matrix) != 2 || matrix[0] != "10" {
		t.Fatalf("matriz não chegou ao frontend: %v", out["matrix"])
	}
	if out["png"] != "/tmp/qr.png" {
		t.Fatalf("png ausente: %v", out["png"])
	}
}

// The frontend decides between "reconectar" and "ler um novo QR" from these
// flags, so they have to survive the bridge.
func TestJsonUiSetStatusCarriesLoginState(t *testing.T) {
	var buf bytes.Buffer
	j := &JsonUiHandler{enc: json.NewEncoder(&buf)}
	j.SetStatus(messages.SessionStatus{NeedsLogin: true})
	out := decode(t, &buf)
	if out["needsLogin"] != true || out["loggedIn"] != false || out["connecting"] != false {
		t.Fatalf("estado de login incompleto: %v", out)
	}
}
