// Copyright 2026 Glassbox Users
// SPDX-License-Identifier: Apache-2.0

package trace

import (
	"testing"
	"time"
)

func TestCompareTraces_Identical(t *testing.T) {
	baseline := &ExecutionTrace{
		TransactionHash: "test123",
		StartTime:       time.Now(),
		EndTime:         time.Now().Add(time.Second),
		States: []ExecutionState{
			{
				Step:      0,
				Operation: "contract_call",
				ContractID: "CABC123",
				Function:  "init",
			},
		},
	}
	
	current := &ExecutionTrace{
		TransactionHash: "test123",
		StartTime:       baseline.StartTime,
		EndTime:         baseline.EndTime,
		States: []ExecutionState{
			{
				Step:      0,
				Operation: "contract_call",
				ContractID: "CABC123",
				Function:  "init",
			},
		},
	}
	
	result := CompareTraces(baseline, current, "", "")
	if result.HasDivergence {
		t.Fatalf("Expected identical traces, got divergence")
	}
}

func TestCompareTraces_DifferentStepCount(t *testing.T) {
	baseline := &ExecutionTrace{
		TransactionHash: "test123",
		States: []ExecutionState{
			{Step: 0, Operation: "contract_call"},
		},
	}
	
	current := &ExecutionTrace{
		TransactionHash: "test123",
		States: []ExecutionState{
			{Step: 0, Operation: "contract_call"},
			{Step: 1, Operation: "host_function"},
		},
	}
	
	result := CompareTraces(baseline, current, "", "")
	if !result.HasDivergence {
		t.Fatalf("Expected divergence due to different step counts, got none")
	}
	if len(result.StateDiffs) == 0 {
		t.Fatalf("Expected state diffs, got none")
	}
}

func TestCompareTraces_DifferentOperation(t *testing.T) {
	baseline := &ExecutionTrace{
		TransactionHash: "test123",
		States: []ExecutionState{
			{Step: 0, Operation: "contract_call", ContractID: "CABC123"},
		},
	}
	
	current := &ExecutionTrace{
		TransactionHash: "test123",
		States: []ExecutionState{
			{Step: 0, Operation: "host_function", ContractID: "CABC123"},
		},
	}
	
	result := CompareTraces(baseline, current, "", "")
	if !result.HasDivergence {
		t.Fatalf("Expected divergence due to different operations, got none")
	}
	foundOpDiff := false
	for _, diff := range result.StateDiffs {
		if diff.Field == "Operation" {
			foundOpDiff = true
			break
		}
	}
	if !foundOpDiff {
		t.Fatalf("Expected operation diff, but none found")
	}
}


// ── Empty input boundary tests (issue #1230) ─────────────────────────────────
//
// Empty traces (no execution states) and nil trace pointers must produce
// deliberate, deterministic output instead of panicking. These tests cover the
// non-normalized CompareTraces path used by the raw diff renderer.

func TestCompareTraces_EmptyVsEmpty(t *testing.T) {
	baseline := NewExecutionTrace("empty", 0)
	current := NewExecutionTrace("empty", 0)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("CompareTraces panicked on empty traces: %v", r)
		}
	}()

	result := CompareTraces(baseline, current, "", "")
	if result.HasDivergence {
		t.Fatalf("Expected no divergence for empty-vs-empty, got divergence")
	}
	if result.StatusDiff == nil || !result.StatusDiff.Match {
		t.Fatalf("Expected status match for empty-vs-empty")
	}
	if len(result.StateDiffs) != 0 {
		t.Fatalf("Expected no state diffs for empty-vs-empty, got %d", len(result.StateDiffs))
	}
}

func TestCompareTraces_EmptyVsNonEmpty(t *testing.T) {
	baseline := NewExecutionTrace("empty", 0)
	current := &ExecutionTrace{
		TransactionHash: "non-empty",
		States: []ExecutionState{
			{Step: 0, Operation: "contract_call", ContractID: "CABC", Function: "init"},
		},
	}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("CompareTraces panicked on empty-vs-nonempty: %v", r)
		}
	}()

	result := CompareTraces(baseline, current, "", "")
	if !result.HasDivergence {
		t.Fatalf("Expected divergence for empty-vs-nonempty, got none")
	}
	// The step count difference must be reported, not silently dropped.
	if result.StatusDiff.BaselineSteps != 0 || result.StatusDiff.CurrentSteps != 1 {
		t.Fatalf("Expected step counts 0 vs 1, got %d vs %d",
			result.StatusDiff.BaselineSteps, result.StatusDiff.CurrentSteps)
	}
}

func TestCompareTraces_NilBaseline_DoesNotPanic(t *testing.T) {
	current := &ExecutionTrace{
		TransactionHash: "non-empty",
		States: []ExecutionState{
			{Step: 0, Operation: "contract_call"},
		},
	}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("CompareTraces panicked on nil baseline: %v", r)
		}
	}()

	result := CompareTraces(nil, current, "", "")
	if !result.HasDivergence {
		t.Fatalf("Expected divergence for nil-baseline-vs-nonempty, got none")
	}
}

func TestCompareTraces_NilCurrent_DoesNotPanic(t *testing.T) {
	baseline := &ExecutionTrace{
		TransactionHash: "non-empty",
		States: []ExecutionState{
			{Step: 0, Operation: "contract_call"},
		},
	}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("CompareTraces panicked on nil current: %v", r)
		}
	}()

	result := CompareTraces(baseline, nil, "", "")
	if !result.HasDivergence {
		t.Fatalf("Expected divergence for nonempty-vs-nil-current, got none")
	}
}

func TestCompareTraces_BothNil_DoesNotPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("CompareTraces panicked on both nil: %v", r)
		}
	}()

	result := CompareTraces(nil, nil, "", "")
	// Both nil → treated as empty → no divergence.
	if result.HasDivergence {
		t.Fatalf("Expected no divergence for nil-vs-nil, got divergence")
	}
}
