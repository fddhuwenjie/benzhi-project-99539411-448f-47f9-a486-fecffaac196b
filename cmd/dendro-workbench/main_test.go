package main

import "testing"

func TestParseConfigUsesLoopbackPORT(t *testing.T) {
	t.Setenv("PORT", "19444")
	cfg, err := parseConfig(nil)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.address != "127.0.0.1:19444" {
		t.Fatalf("unexpected address %s", cfg.address)
	}
}
func TestParseConfigRejectsPublicBind(t *testing.T) {
	t.Setenv("PORT", "")
	if _, err := parseConfig([]string{"-addr=0.0.0.0:19081"}); err == nil {
		t.Fatal("public bind should be rejected")
	}
}
