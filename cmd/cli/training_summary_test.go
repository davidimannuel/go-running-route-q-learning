package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestTrainingTerminalSummaryAndDebug(t *testing.T) {
	for _, debug := range []bool{false, true} {
		workdir := newWorkdir(t, trainingYAML)
		saveSnapshot(t, workdir+"/output", loopSnapshot())
		args := []string{"--workdir", workdir}
		if debug {
			args = append(args, "--debug")
		}
		var stdout, logs bytes.Buffer
		if err := runTrain(context.Background(), args, &stdout, &logs); err != nil {
			t.Fatal(err)
		}
		text := stdout.String()
		for _, want := range []string{"Scenario B", "2 seeds x 12 episodes", "Epsilon 1.00 -> 0.05", "[1/2] seed=", "[2/2] seed=", "Summary: greedy loops 2/2", "Saved:"} {
			if !strings.Contains(text, want) {
				t.Fatalf("missing %q in summary: %s", want, text)
			}
		}
		if strings.Contains(text, "Training progress") || strings.Contains(text, `"level"`) {
			t.Fatalf("detailed logs leaked to summary: %s", text)
		}
		if !debug && logs.Len() != 0 {
			t.Fatalf("default training logged details: %s", logs.String())
		}
		if debug && (!strings.Contains(logs.String(), "Training progress") || !strings.Contains(logs.String(), `"level":"DEBUG"`)) {
			t.Fatalf("debug details missing: %s", logs.String())
		}
	}
}
