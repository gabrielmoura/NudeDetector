package asr

import "testing"

func TestReconcileChunkTextRemovesBoundaryOverlap(t *testing.T) {
	got := reconcileChunkText([]string{"Olá mundo como", "mundo como você está", "você está hoje"})
	if got != "Olá mundo como você está hoje" {
		t.Fatalf("reconcileChunkText() = %q", got)
	}
}

func TestReconcileChunkTextSkipsEmptyChunks(t *testing.T) {
	got := reconcileChunkText([]string{"", "Olá", "  "})
	if got != "Olá" {
		t.Fatalf("reconcileChunkText() = %q", got)
	}
}
