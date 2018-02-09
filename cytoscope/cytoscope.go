// cytoscope.go
package cytoscope

import (
	"fmt"
	"strings"

	"github.com/jshaughn/sunshine/tree"
)

type NodeData struct {
	Id            string `json:"id"`
	Name          string `json:"name"`
	LinkPromGraph string `json:"link_prom_graph,omitempty"`
}

type EdgeData struct {
	Id     string `json:"id"`
	Source string `json:"source"`
	Target string `json:"target"`
	Rpm2xx string `json:"req_per_min_2XX,omitempty"`
	Rpm3xx string `json:"req_per_min_3XX,omitempty"`
	Rpm4xx string `json:"req_per_min_4XX,omitempty"`
	Rpm5xx string `json:"req_per_min_5XX,omitempty"`
}

type NodeWrapper struct {
	Data NodeData `json:"data"`
}

type EdgeWrapper struct {
	Data EdgeData `json:"data"`
}

type Elements struct {
	Nodes []NodeWrapper `json:"nodes"`
	Edges []EdgeWrapper `json:"edges"`
}

type Config struct {
	Elements Elements `json:"elements"`
}

func NewConfig(t *tree.Tree) (result Config) {
	nodes := []NodeWrapper{}
	edges := []EdgeWrapper{}
	var edgeIdSequence int

	walk(t, &nodes, &edges, &edgeIdSequence)

	elements := Elements{nodes, edges}
	result = Config{elements}
	return result
}

func walk(t *tree.Tree, nodes *[]NodeWrapper, edges *[]EdgeWrapper, edgeIdSequence *int) {
	nodeId := fmt.Sprintf("%v (%v)", t.Name, t.Version)
	nd := NodeData{
		Id:            nodeId,
		Name:          fmt.Sprintf("%v (%v)", strings.Split(t.Name, ".")[0], t.Version),
		LinkPromGraph: t.Metadata["link_prom_graph"].(string),
	}
	nw := NodeWrapper{
		Data: nd,
	}
	*nodes = append(*nodes, nw)

	if t.Parent != nil {
		edgeId := fmt.Sprintf("%v", *edgeIdSequence)
		*edgeIdSequence++
		ed := EdgeData{
			Id:     edgeId,
			Source: fmt.Sprintf("%v (%v)", t.Parent.Name, t.Parent.Version),
			Target: nodeId,
		}
		addRpmField(&ed.Rpm2xx, t, "req_per_min_2xx")
		addRpmField(&ed.Rpm3xx, t, "req_per_min_3xx")
		addRpmField(&ed.Rpm4xx, t, "req_per_min_4xx")
		addRpmField(&ed.Rpm5xx, t, "req_per_min_5xx")
		ew := EdgeWrapper{
			Data: ed,
		}
		*edges = append(*edges, ew)
	}

	for _, c := range t.Children {
		walk(c, nodes, edges, edgeIdSequence)
	}
}

func addRpmField(rpmField *string, t *tree.Tree, key string) {
	rpm := t.Metadata[key].(float64)
	if rpm > 0.0 {
		*rpmField = fmt.Sprintf("%.2f", rpm)
	}
}
