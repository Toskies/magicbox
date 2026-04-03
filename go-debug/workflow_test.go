package debug

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseUserRecord(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		want    UserRecord
		wantErr string
	}{
		{
			name: "ok",
			line: "101, Alice, 88",
			want: UserRecord{
				ID:    101,
				Name:  "Alice",
				Score: 88,
			},
		},
		{
			name:    "invalid_id",
			line:    "x, Alice, 88",
			wantErr: "invalid id",
		},
		{
			name:    "missing_score",
			line:    "101, Alice",
			wantErr: "expected 3 fields",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseUserRecord(tc.line)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error %q, got nil", tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error = %q, want substring %q", err.Error(), tc.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("ParseUserRecord(%q) error = %v", tc.line, err)
			}
			if got != tc.want {
				t.Fatalf("ParseUserRecord(%q) = %#v, want %#v", tc.line, got, tc.want)
			}
		})
	}
}

func TestBuildSummaryMatches(t *testing.T) {
	users := benchmarkUsers()

	gotSlow := BuildSummarySlow(users)
	gotFast := BuildSummaryFast(users)
	if gotSlow != gotFast {
		t.Fatalf("summary mismatch\nslow: %q\nfast: %q", gotSlow, gotFast)
	}
}

func TestLockedCounter(t *testing.T) {
	counter := NewLockedCounter()
	done := make(chan struct{})

	for i := 0; i < 4; i++ {
		go func() {
			for j := 0; j < 1000; j++ {
				counter.Inc()
			}
			done <- struct{}{}
		}()
	}

	for i := 0; i < 4; i++ {
		<-done
	}

	if got, want := counter.Value(), 4000; got != want {
		t.Fatalf("counter.Value() = %d, want %d", got, want)
	}
}

func TestUnsafeCounterRace(t *testing.T) {
	if os.Getenv("DEBUG_RUN_RACE_EXAMPLE") != "1" {
		t.Skip("set DEBUG_RUN_RACE_EXAMPLE=1 to trigger a race example with -race")
	}

	counter := NewUnsafeCounter()
	done := make(chan struct{})

	for i := 0; i < 4; i++ {
		go func() {
			for j := 0; j < 1000; j++ {
				counter.Inc()
			}
			done <- struct{}{}
		}()
	}

	for i := 0; i < 4; i++ {
		<-done
	}

	if counter.Value() == 0 {
		t.Fatal("race example did not execute")
	}
}

func TestWriteDebugArtifacts(t *testing.T) {
	dir := t.TempDir()

	artifacts, err := WriteDebugArtifacts(dir, benchmarkUsers())
	if err != nil {
		t.Fatalf("WriteDebugArtifacts() error = %v", err)
	}

	paths := []string{
		artifacts.StackPath,
		artifacts.GoroutineProfilePath,
		artifacts.HeapProfilePath,
		artifacts.CPUProfilePath,
		artifacts.TracePath,
	}

	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("expected artifact %q to exist: %v", path, err)
		}
		if info.Size() == 0 {
			t.Fatalf("artifact %q is empty", path)
		}
	}

	stackBytes, err := os.ReadFile(filepath.Clean(artifacts.StackPath))
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", artifacts.StackPath, err)
	}
	if !strings.Contains(string(stackBytes), "goroutine") {
		t.Fatalf("stack dump %q does not contain goroutine header", artifacts.StackPath)
	}
}

func BenchmarkBuildSummarySlow(b *testing.B) {
	users := benchmarkUsers()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = BuildSummarySlow(users)
	}
}

func BenchmarkBuildSummaryFast(b *testing.B) {
	users := benchmarkUsers()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = BuildSummaryFast(users)
	}
}
