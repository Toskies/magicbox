package debug

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"runtime/trace"
)

type DebugArtifacts struct {
	StackPath            string
	GoroutineProfilePath string
	HeapProfilePath      string
	CPUProfilePath       string
	TracePath            string
}

func WriteDebugArtifacts(dir string, users []UserRecord) (DebugArtifacts, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return DebugArtifacts{}, fmt.Errorf("create artifact dir: %w", err)
	}

	artifacts := DebugArtifacts{
		StackPath:            filepath.Join(dir, "goroutine-stacks.txt"),
		GoroutineProfilePath: filepath.Join(dir, "goroutine.prof"),
		HeapProfilePath:      filepath.Join(dir, "heap.prof"),
		CPUProfilePath:       filepath.Join(dir, "cpu.prof"),
		TracePath:            filepath.Join(dir, "trace.out"),
	}

	if err := writeGoroutineStacks(artifacts.StackPath); err != nil {
		return DebugArtifacts{}, err
	}
	if err := writeProfile("goroutine", artifacts.GoroutineProfilePath); err != nil {
		return DebugArtifacts{}, err
	}
	if err := writeHeapProfile(artifacts.HeapProfilePath); err != nil {
		return DebugArtifacts{}, err
	}
	if err := recordCPUProfile(artifacts.CPUProfilePath, func() {
		for i := 0; i < 3000; i++ {
			_ = BuildSummaryFast(users)
		}
	}); err != nil {
		return DebugArtifacts{}, err
	}
	if err := recordTrace(artifacts.TracePath, func() {
		for i := 0; i < 1500; i++ {
			_ = BuildSummarySlow(users)
		}
	}); err != nil {
		return DebugArtifacts{}, err
	}

	return artifacts, nil
}

func writeGoroutineStacks(path string) error {
	buf := make([]byte, 1<<20)
	n := runtime.Stack(buf, true)
	if n == 0 {
		return fmt.Errorf("runtime.Stack returned no data")
	}
	return os.WriteFile(path, buf[:n], 0o644)
}

func writeProfile(name string, path string) error {
	profile := pprof.Lookup(name)
	if profile == nil {
		return fmt.Errorf("profile %q is unavailable", name)
	}

	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create %s profile: %w", name, err)
	}
	defer file.Close()

	if err := profile.WriteTo(file, 0); err != nil {
		return fmt.Errorf("write %s profile: %w", name, err)
	}

	return nil
}

func writeHeapProfile(path string) error {
	runtime.GC()

	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create heap profile: %w", err)
	}
	defer file.Close()

	if err := pprof.WriteHeapProfile(file); err != nil {
		return fmt.Errorf("write heap profile: %w", err)
	}

	return nil
}

func recordCPUProfile(path string, fn func()) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create cpu profile: %w", err)
	}
	defer file.Close()

	if err := pprof.StartCPUProfile(file); err != nil {
		return fmt.Errorf("start cpu profile: %w", err)
	}
	fn()
	pprof.StopCPUProfile()

	return nil
}

func recordTrace(path string, fn func()) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create trace file: %w", err)
	}
	defer file.Close()

	if err := trace.Start(file); err != nil {
		return fmt.Errorf("start trace: %w", err)
	}
	fn()
	trace.Stop()

	return nil
}
