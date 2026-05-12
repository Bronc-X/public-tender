package main

import "testing"

func TestResolveDBPathUsesEnvOverride(t *testing.T) {
	t.Setenv("APP_DB_PATH", "/tmp/custom-app.db")

	if got := resolveDBPath(); got != "/tmp/custom-app.db" {
		t.Fatalf("resolveDBPath() = %q, want env override", got)
	}
}

func TestResolveDBPathDefaultsToProjectDataDB(t *testing.T) {
	t.Setenv("APP_DB_PATH", "")

	if got := resolveDBPath(); got != "data/app.db" {
		t.Fatalf("resolveDBPath() = %q, want data/app.db", got)
	}
}
