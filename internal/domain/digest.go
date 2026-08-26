package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

func CanonicalJSON(v any) ([]byte, error) { return json.Marshal(v) }

func Digest(v any) (string, error) {
	b, err := CanonicalJSON(v)
	if err != nil {
		return "", err
	}
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:]), nil
}

func CanonicalString(v any) string { b, _ := CanonicalJSON(v); return string(b) }

func StableID(parts ...string) string {
	h := sha256.New()
	for _, p := range parts {
		_, _ = h.Write([]byte(fmt.Sprintf("%d:%s", len(p), p)))
	}
	return hex.EncodeToString(h.Sum(nil))[:24]
}

func FinalizeEvent(event *AuditEvent) error {
	copy := *event
	copy.Digest = ""
	d, err := Digest(copy)
	if err != nil {
		return err
	}
	event.Digest = d
	return nil
}

func VerifyEvent(event AuditEvent) bool {
	expected := event.Digest
	event.Digest = ""
	d, err := Digest(event)
	return err == nil && d == expected
}
