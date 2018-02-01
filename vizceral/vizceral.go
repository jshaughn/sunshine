// vizceral.go
package vizceral

import (
	"fmt"
	"strings"
	"time"

	"github.com/jshaughn/sunshine/tree"
)

type Metadata struct {
	Streaming int `json:"streaming,omitempty"`
}

type Metrics struct {
	Danger  int `json:"danger,omitempty"`
	Warning int `json:"warning,omitempty"`
	Normal  int `json:"normal,omitempty"`
}

type Connection struct {
	Source   string   `json:"source"`
	Target   string   `json:"target"`
	Metadata Metadata `json:"metadata,omitempty"`
	Metrics  Metrics  `json:"metrics,omitempty"`
}

type Node struct {
	Renderer    string       `json:"renderer,omitempty"`
	Name        string       `json:"name"`
	DisplayName string       `json:"displayName,omitempty"`
	Class       string       `json:"class,omitempty"`
	Updated     int64        `json:"updated,omitempty"`
	MaxVolume   int          `json:"maxVolume,omitempty"`
	Metadata    Metadata     `json:"metadata,omitempty"`
	Nodes       []Node       `json:"nodes,omitempty"`
	Connections []Connection `json:"connections,omitempty"`
}

type Config Node

func NewConfig(t *tree.Tree) (result Config) {
	meshInternetNode := Node{
		Renderer: "focusedChild",
		Name:     "INTERNET",
		Metadata: Metadata{
			Streaming: 1,
		},
	}
	meshNodes := []Node{meshInternetNode}
	var meshConnections []Connection

	walk(t, &meshNodes, &meshConnections)

	meshNode := Node{
		Renderer:  "region",
		Name:      "istio-mesh",
		MaxVolume: 10000,
		Metadata: Metadata{
			Streaming: 1,
		},
		Nodes:       meshNodes,
		Connections: meshConnections,
	}

	regionInternetNode := Node{
		Renderer: "region",
		Name:     "INTERNET",
		Metadata: Metadata{
			Streaming: 1,
		},
	}
	regionInternetConnection := Connection{
		Source: "INTERNET",
		Target: "istio-mesh",
		Metadata: Metadata{
			Streaming: 1,
		},
		Metrics: Metrics{
			Danger: 10,
			Normal: 1000,
		},
	}

	regionNodes := []Node{regionInternetNode, meshNode}
	regionConnections := []Connection{regionInternetConnection}

	result = Config{
		Renderer:    "global",
		Name:        "edge",
		Updated:     time.Now().Unix(),
		MaxVolume:   1000,
		Nodes:       regionNodes,
		Connections: regionConnections,
	}
	return result
}

func walk(t *tree.Tree, nodes *[]Node, connections *[]Connection) {
	name := fmt.Sprintf("%v (%v)", t.Name, t.Version)
	displayName := fmt.Sprintf("%v (%v)", strings.Split(t.Name, ".")[0], t.Version)
	n := Node{
		Renderer:    "focusedChild",
		Name:        name,
		DisplayName: displayName,
		Metadata: Metadata{
			Streaming: 1,
		},
	}
	*nodes = append(*nodes, n)

	var parentName string
	if nil == t.Parent {
		parentName = "INTERNET"
	} else {
		parentName = fmt.Sprintf("%v (%v)", t.Parent.Name, t.Parent.Version)
	}
	c := Connection{
		Source: parentName,
		Target: name,
		Metadata: Metadata{
			Streaming: 1,
		},
		Metrics: Metrics{
			Danger: 10,
			Normal: 1000,
		},
	}
	*connections = append(*connections, c)

	for _, c := range t.Children {
		walk(c, nodes, connections)
	}
}
