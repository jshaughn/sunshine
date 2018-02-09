// cytoscope.go
package cytoscope

import (
	"fmt"
	"strings"

	"github.com/jshaughn/sunshine/tree"
)

type NodeData struct {
	Id            string `json:"id"`
	Service       string `json:"service"`
	Version       string `json:"version"`
	Text          string `json:"text"`
	LinkPromGraph string `json:"link_prom_graph,omitempty"`
}

type EdgeData struct {
	Id     string `json:"id"`
	Source string `json:"source"`
	Target string `json:"target"`
	Text   string `json:"text"`
	Color  string `json:"color"`
	Rpm    string `json:"req_per_min,omitempty"`
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

	var nodeIdSequence int
	var edgeIdSequence int

	walk(t, &nodes, &edges, "", &nodeIdSequence, &edgeIdSequence)

	elements := Elements{nodes, edges}
	result = Config{elements}
	return result
}

func walk(t *tree.Tree, nodes *[]NodeWrapper, edges *[]EdgeWrapper, parentNodeId string, nodeIdSequence, edgeIdSequence *int) {
	nodeId := fmt.Sprintf("n%v", *nodeIdSequence)
	*nodeIdSequence++
	nd := NodeData{
		Id:            nodeId,
		Service:       t.Name,
		Version:       t.Version,
		Text:          fmt.Sprintf("%v (%v)", strings.Split(t.Name, ".")[0], t.Version),
		LinkPromGraph: t.Metadata["link_prom_graph"].(string),
	}
	nw := NodeWrapper{
		Data: nd,
	}
	*nodes = append(*nodes, nw)

	if parentNodeId != "" {
		edgeId := fmt.Sprintf("e%v", *edgeIdSequence)
		*edgeIdSequence++
		ed := EdgeData{
			Id:     edgeId,
			Source: parentNodeId,
			Target: nodeId,
		}
		addRpm(&ed, t)
		//addRpmField(&ed.Rpm2xx, t, "req_per_min_2xx")
		//addRpmField(&ed.Rpm3xx, t, "req_per_min_3xx")
		//addRpmField(&ed.Rpm4xx, t, "req_per_min_4xx")
		//addRpmField(&ed.Rpm5xx, t, "req_per_min_5xx")
		ew := EdgeWrapper{
			Data: ed,
		}
		*edges = append(*edges, ew)
	}

	for _, c := range t.Children {
		walk(c, nodes, edges, nodeId, nodeIdSequence, edgeIdSequence)
	}
}

func addRpm(ed *EdgeData, t *tree.Tree) {
	rpm := t.Metadata["req_per_min"].(float64)
	if rpm > 0.0 {
		rpmSuccess := t.Metadata["req_per_min_2xx"].(float64)
		errorRate := rpm - rpmSuccess/rpm*100
		// TODO remove, just introduce some error for edge 3 and edge 4, just to test
		if ed.Id == "e3" {
			errorRate = 1.5
		} else if ed.Id == "e4" {
			errorRate = 0.5
		}
		switch {
		case errorRate > 1.0:
			ed.Color = "red" // "#ff0000"
			ed.Text = fmt.Sprintf("rpm=%.2f (err=%.2f%%)", rpm, errorRate)
		case errorRate > 0.0:
			ed.Color = "orange" // #ff9900"
			ed.Text = fmt.Sprintf("rpm=%.2f (err=%.2f%%)", rpm, errorRate)
		default:
			ed.Color = "green" //#009933"
			ed.Text = fmt.Sprintf("rpm=%.2f", rpm)
		}
	} else {
		ed.Color = "black" // #000000"
		ed.Text = "rpm=0"
	}
}

func addRpmField(rpmField *string, t *tree.Tree, key string) {
	rpm := t.Metadata[key].(float64)
	if rpm > 0.0 {
		*rpmField = fmt.Sprintf("%.2f", rpm)
	}
}
