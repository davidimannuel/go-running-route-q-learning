package analysis

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/davidimannuel/go-running-route-q-learning/internal/qlearning"
)

func TestReadNodeQTableFiltersWithoutAssumingOrdering(t *testing.T) {
	csv := strings.Join(qHeader, ",") + "\nC,42,5,0,9,0,-2\nC,9,42,0,5,0,100\nC,42,7,1,9,0,0\nC,42,7,1,9,1,3\nC,42,7,2,,,\n"
	got, err := ReadNodeQTable(context.Background(), strings.NewReader(csv), 42)
	want := qlearning.QTable{
		{Scenario: "C", CurrentNode: 42, PreviousNode: 5, ProgressBin: 0}: {{NextNode: 9}: -2},
		{Scenario: "C", CurrentNode: 42, PreviousNode: 7, ProgressBin: 1}: {{NextNode: 9}: 0, {NextNode: 9, Key: 1}: 3},
		{Scenario: "C", CurrentNode: 42, PreviousNode: 7, ProgressBin: 2}: {},
	}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("table=%v error=%v", got, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := ReadNodeQTable(ctx, strings.NewReader(csv), 42); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel=%v", err)
	}
	if _, err := ReadNodeQTable(context.Background(), strings.NewReader(strings.Join(qHeader, ",")+"\nC,42,7,1,9,0,NaN\n"), 42); err == nil {
		t.Fatal("non-finite Q accepted")
	}
}
