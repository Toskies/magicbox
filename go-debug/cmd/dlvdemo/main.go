package main

import (
	"flag"
	"fmt"
	"log"

	debugpkg "coding/debug"
)

func main() {
	artifactsDir := flag.String("artifacts-dir", "", "write pprof and trace artifacts into this directory")
	flag.Parse()

	lines := []string{
		"101, Alice, 88",
		"102, Bob, 91",
		"103, Carol, 95",
	}

	users := make([]debugpkg.UserRecord, 0, len(lines))
	for _, line := range lines {
		user, err := debugpkg.ParseUserRecord(line)
		if err != nil {
			log.Fatalf("parse %q: %v", line, err)
		}
		users = append(users, user)
	}

	fmt.Print(debugpkg.BuildSummaryFast(users))

	if *artifactsDir == "" {
		return
	}

	artifacts, err := debugpkg.WriteDebugArtifacts(*artifactsDir, users)
	if err != nil {
		log.Fatalf("write artifacts: %v", err)
	}

	fmt.Printf("\nartifacts written to %s\n", *artifactsDir)
	fmt.Printf("stack: %s\n", artifacts.StackPath)
	fmt.Printf("goroutine profile: %s\n", artifacts.GoroutineProfilePath)
	fmt.Printf("heap profile: %s\n", artifacts.HeapProfilePath)
	fmt.Printf("cpu profile: %s\n", artifacts.CPUProfilePath)
	fmt.Printf("trace: %s\n", artifacts.TracePath)
}
