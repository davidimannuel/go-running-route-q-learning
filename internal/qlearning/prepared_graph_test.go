package qlearning

import (
	"strings"
	"testing"

	"github.com/davidimannuel/go-running-route-q-learning/internal/osm"
)

func TestPreparedGraphRejectsBlockedStartWithoutResnapping(t *testing.T) {
	graph := actionTestGraph(t)
	node := graph.Nodes[1]
	node.Tags = osm.Tags{"access": "no", "barrier": "gate"}
	graph.Nodes[1] = node
	id, distance, err := NearestStart(graph, node.Lat, node.Lon)
	if err != nil || id != 1 || distance != 0 {
		t.Fatalf("nearest start=(%d,%v,%v); must not silently choose another node", id, distance, err)
	}
	if _, err := NewPreparedGraph(graph, id, DefaultRewardConfig()); err == nil || !strings.Contains(err.Error(), "start node 1 has disallowed access") {
		t.Fatalf("blocked start error=%v", err)
	}
}
