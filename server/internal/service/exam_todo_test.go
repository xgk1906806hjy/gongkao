package service

import (
	"testing"
	"time"
)

func TestCoalesceAndRemindOffset(t *testing.T) {
	end := time.Date(2026, 10, 10, 10, 0, 0, 0, time.UTC)
	start := time.Date(2026, 10, 9, 10, 0, 0, 0, time.UTC)
	due := coalesceTime(&end, &start)
	if due == nil || !due.Equal(end) {
		t.Fatalf("prefer end, got %v", due)
	}
	due2 := coalesceTime(nil, &start)
	if due2 == nil || !due2.Equal(start) {
		t.Fatalf("fallback start, got %v", due2)
	}
	remind := end.Add(-10 * time.Minute)
	want := time.Date(2026, 10, 10, 9, 50, 0, 0, time.UTC)
	if !remind.Equal(want) {
		t.Fatalf("remind_at want %v got %v", want, remind)
	}
}