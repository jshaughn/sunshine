// cx.go
package cx

import (
	"fmt"
	"strings"
	"time"

	"github.com/jshaughn/sunshine/tree"
)

type AspectMetadata struct {
	Name      string `json:"name"`
	Version   int64  `json:"version"`
	IdCounter int    `json:"idCounter"`
}

type Node struct {
	Id         int    `json:"@id"`
	Name       string `json:"n"`
	Represents string `json:"r"`
}

type NodeAttribute struct {
	NodeId int      `json:"po"`
	Name   string   `json:"n"`
	Values []string `json:"v"`
}

type Edge struct {
	Id     int `json:"@id"`
	Source int `json:"s"`
	Target int `json:"t"`
}

type EdgeAttribute struct {
	EdgeId int      `json:"po"`
	Name   string   `json:"n"`
	Values []string `json:"v"`
}

type Config struct {
	NodesAspect AspectMetadata
	EdgesAspect AspectMetadata
	Nodes       []Node          `json:"nodes"`
	Edges       []Edge          `json:"edges"`
	NodeAttrs   []NodeAttribute `json:"nodeAttributes"`
	EdgeAttrs   []EdgeAttribute `json:"edgeAttributes"`
}

func NewConfig(t *tree.Tree) (result Config) {
	now := time.Now().Unix()
	nodesAspect := AspectMetadata{
		Name:    "nodes",
		Version: now,
	}
	edgesAspect := AspectMetadata{
		Name:    "edges",
		Version: now,
	}
	nodes := []Node{}
	nodeAttrs := []NodeAttribute{}
	edges := []Edge{}
	edgeAttrs := []EdgeAttribute{}

	var nodeIdSequence int
	var edgeIdSequence int

	walk(t, &nodes, &nodeAttrs, &edges, &edgeAttrs, -1, &nodeIdSequence, &edgeIdSequence)
	nodesAspect.IdCounter = nodeIdSequence
	edgesAspect.IdCounter = edgeIdSequence

	result = Config{
		NodesAspect: nodesAspect,
		EdgesAspect: edgesAspect,
		Nodes:       nodes,
		Edges:       edges,
		NodeAttrs:   nodeAttrs,
		EdgeAttrs:   edgeAttrs,
	}
	return result
}

func walk(t *tree.Tree, nodes *[]Node, nodeAttrs *[]NodeAttribute, edges *[]Edge, edgeAttrs *[]EdgeAttribute, parentNodeId int, nodeIdSequence, edgeIdSequence *int) {

	nodeId := *nodeIdSequence
	*nodeIdSequence++
	n := Node{
		Id:         nodeId,
		Name:       fmt.Sprintf("%v (%v)", strings.Split(t.Name, ".")[0], t.Version),
		Represents: fmt.Sprintf("%v (%v)", t.Name, t.Version),
	}
	*nodes = append(*nodes, n)

	na := NodeAttribute{
		NodeId: nodeId,
		Name:   "Prometheus Graph",
		Values: []string{t.Metadata["link_prom_graph"].(string)},
	}
	*nodeAttrs = append(*nodeAttrs, na)

	if parentNodeId >= 0 {
		edgeId := *edgeIdSequence
		*edgeIdSequence++
		e := Edge{
			Id:     edgeId,
			Source: parentNodeId,
			Target: *nodeIdSequence,
		}
		*edges = append(*edges, e)
		addRpmEdgeAttr(edgeAttrs, edgeId, t, "req_per_min_2xx")
		addRpmEdgeAttr(edgeAttrs, edgeId, t, "req_per_min_3xx")
		addRpmEdgeAttr(edgeAttrs, edgeId, t, "req_per_min_4xx")
		addRpmEdgeAttr(edgeAttrs, edgeId, t, "req_per_min_5xx")

	}

	for _, c := range t.Children {
		walk(c, nodes, nodeAttrs, edges, edgeAttrs, nodeId, nodeIdSequence, edgeIdSequence)
	}
}

func addRpmEdgeAttr(edgeAttrs *[]EdgeAttribute, edgeId int, t *tree.Tree, key string) {
	rpm := t.Metadata[key].(float64)
	if rpm > 0.0 {
		ea := EdgeAttribute{
			EdgeId: edgeId,
			Name:   key,
			Values: []string{fmt.Sprintf("%.2f", rpm)},
		}
		*edgeAttrs = append(*edgeAttrs, ea)
	}
}
