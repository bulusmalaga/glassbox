// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── helpers ───────────────────────────────────────────────────────────────────

// resetTraceCompareFlags restores the trace compare command's global flag
// state to known defaults so each sub-test starts from a clean baseline.
func resetTraceCompareFlags() {
	traceCompareBaselineNameFlag = "Baseline"
	traceCompareCurrentNameFlag = "Current"
	traceSaveOutputFlag = ""
	traceCompareOutputFormatFlag = "table"
	traceCompareConfigFileFlag = ""
	traceCompareCPUThresholdFlag = 10.0
	traceCompareMemThresholdFlag = 10.0
	traceCompareFailOnViolation = false
	traceCompareInfoModeFlag = false
}

// writeTraceFile writes the given JSON content to a file in dir and returns
// the full path. Used to set up on-disk trace fixtures for runTraceCompare.
func writeTraceFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write trace fixture %q: %v", path, err)
	}
	return path
}

// ── Empty file: validation error (issue #1230) ────────────────────────────────
//
// A zero-byte file is not valid JSON and must be rejected with a clear
// validation error rather than crashing or producing confusing output.

func TestRunTraceCompare_EmptyFile_ReturnsValidationError(t *testing.T) {
	t.Cleanup(resetTraceCompareFlags)
	dir := t.TempDir()

	emptyFile := filepath.Join(dir, "empty.json")
	if err := os.WriteFile(emptyFile, []byte{}, 0644); err != nil {
		t.Fatalf("failed to create empty file: %v", err)
	}
	nonEmpty := writeTraceFile(t, dir, "nonempty.json",
		`{"transaction_hash":"tx","states":[{"step":0,"operation":"contract_call"}]}`)

	err := runTraceCompare(nil, []string{emptyFile, nonEmpty})
	if err == nil {
		t.Fatal("expected validation error for empty trace file, got nil")
	}
	// Must be a validation error mentioning the failed load, not a panic.
	if !strings.Contains(err.Error(), "failed to load") {
		t.Errorf("error should mention failed load, got: %q", err.Error())
	}
}

// ── Empty-vs-empty: deterministic, no panic (issue #1230) ─────────────────────
//
// Two valid JSON files with zero execution states must compare without panic
// and produce deterministic output (no diffs, no regression).

func TestRunTraceCompare_EmptyVsEmpty_NoPanic(t *testing.T) {
	t.Cleanup(resetTraceCompareFlags)
	traceCompareOutputFormatFlag = "json"
	dir := t.TempDir()

	empty1 := writeTraceFile(t, dir, "empty1.json",
		`{"transaction_hash":"empty1","states":[]}`)
	empty2 := writeTraceFile(t, dir, "empty2.json",
		`{"transaction_hash":"empty2","states":[]}`)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("runTraceCompare panicked on empty-vs-empty: %v", r)
		}
	}()

	err := runTraceCompare(nil, []string{empty1, empty2})
	if err != nil {
		t.Fatalf("expected no error for empty-vs-empty, got: %v", err)
	}
}

// ── Empty-vs-nonempty: deterministic, no panic (issue #1230) ──────────────────
//
// One empty trace and one non-empty trace must compare without panic and
// produce intentional, deterministic output. The empty side represents a
// trace with zero execution steps (e.g. a simulation that produced no events).

func TestRunTraceCompare_EmptyVsNonEmpty_NoPanic(t *testing.T) {
	t.Cleanup(resetTraceCompareFlags)
	traceCompareOutputFormatFlag = "json"
	traceCompareInfoModeFlag = true // never fail on differences
	dir := t.TempDir()

	empty := writeTraceFile(t, dir, "empty.json",
		`{"transaction_hash":"empty","states":[]}`)
	nonEmpty := writeTraceFile(t, dir, "nonempty.json",
		`{"transaction_hash":"nonempty","states":[{"step":0,"operation":"contract_call","contract_id":"CABC","function":"init","cost":{"cpu":2000}}]}`)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("runTraceCompare panicked on empty-vs-nonempty: %v", r)
		}
	}()

	err := runTraceCompare(nil, []string{empty, nonEmpty})
	// In info mode the command should succeed even though differences exist.
	if err != nil {
		t.Fatalf("expected no error in info mode for empty-vs-nonempty, got: %v", err)
	}
}

// ── Empty-vs-nonempty in default mode: no panic (issue #1230) ─────────────────
//
// Without info mode the command still must not panic. With the default
// FailOnThresholdViolation=false it returns nil even when a regression is
// flagged; the important assertion is that the comparison is stable.

func TestRunTraceCompare_EmptyVsNonEmpty_NoPanic_DefaultMode(t *testing.T) {
	t.Cleanup(resetTraceCompareFlags)
	traceCompareOutputFormatFlag = "json"
	dir := t.TempDir()

	empty := writeTraceFile(t, dir, "empty.json",
		`{"transaction_hash":"empty","states":[]}`)
	nonEmpty := writeTraceFile(t, dir, "nonempty.json",
		`{"transaction_hash":"nonempty","states":[{"step":0,"operation":"contract_call","contract_id":"CABC","function":"init","cost":{"cpu":2000}}]}`)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("runTraceCompare panicked on empty-vs-nonempty (default mode): %v", r)
		}
	}()

	// We deliberately don't assert on the return value here — with the default
	// config (FailOnThresholdViolation=false) the command returns nil even when
	// a regression is flagged. The key assertion is that no panic occurs.
	_ = runTraceCompare(nil, []string{empty, nonEmpty})
}

// ── Nonempty-vs-empty (reversed): no panic (issue #1230) ──────────────────────
//
// The reverse direction (baseline has content, current is empty) must also be
// stable. This catches the case where a regression caused the current trace
// to lose all execution steps.

func TestRunTraceCompare_NonEmptyVsEmpty_NoPanic(t *testing.T) {
	t.Cleanup(resetTraceCompareFlags)
	traceCompareOutputFormatFlag = "json"
	traceCompareInfoModeFlag = true
	dir := t.TempDir()

	nonEmpty := writeTraceFile(t, dir, "nonempty.json",
		`{"transaction_hash":"nonempty","states":[{"step":0,"operation":"contract_call","contract_id":"CABC","function":"init","cost":{"cpu":2000}}]}`)
	empty := writeTraceFile(t, dir, "empty.json",
		`{"transaction_hash":"empty","states":[]}`)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("runTraceCompare panicked on nonempty-vs-empty: %v", r)
		}
	}()

	err := runTraceCompare(nil, []string{nonEmpty, empty})
	if err != nil {
		t.Fatalf("expected no error in info mode for nonempty-vs-empty, got: %v", err)
	}
}
