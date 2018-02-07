// vizceral.go
package vizceral

import (
	"fmt"
	"strings"
	"time"

	"github.com/jshaughn/sunshine/tree"
)

type Metadata struct {
}

type Metrics struct {
	Danger  float64 `json:"danger,omitempty"`
	Warning float64 `json:"warning,omitempty"`
	Normal  float64 `json:"normal,omitempty"`
}

type Connection struct {
	Source   string   `json:"source"`
	Target   string   `json:"target"`
	Metadata Metadata `json:"metadata,omitempty"`
	Metrics  Metrics  `json:"metrics,omitempty"`
}

type Notice struct {
	Title    string `json:"title"`
	Link     string `json:"link,omitempty"`
	Severity int    `json:"severity,omitempty"`
}

type Node struct {
	Renderer    string       `json:"renderer,omitempty"`
	Name        string       `json:"name"`
	DisplayName string       `json:"displayName,omitempty"`
	Class       string       `json:"class,omitempty"`
	Updated     int64        `json:"updated,omitempty"`
	MaxVolume   float64      `json:"maxVolume,omitempty"`
	Metadata    Metadata     `json:"metadata,omitempty"`
	Nodes       []Node       `json:"nodes,omitempty"`
	Connections []Connection `json:"connections,omitempty"`
	Notices     []Notice     `json:"notices,omitempty"`
}

type Config Node

func NewConfig(t *tree.Tree) (result Config) {
	meshInternetNode := Node{
		Renderer: "focusedChild",
		Name:     "INTERNET",
	}
	meshNodes := []Node{meshInternetNode}
	var meshConnections []Connection
	var maxVolume float64

	walk(t, &meshNodes, &meshConnections, &maxVolume)

	meshNode := Node{
		Renderer:    "region",
		Name:        "istio-mesh",
		Updated:     time.Now().Unix(),
		MaxVolume:   maxVolume,
		Nodes:       meshNodes,
		Connections: meshConnections,
	}

	regionInternetNode := Node{
		Renderer: "region",
		Name:     "INTERNET",
	}
	regionInternetConnection := Connection{
		Source: "INTERNET",
		Target: "istio-mesh",
		Metrics: Metrics{
			// TODO, should break up MaxVolume by code
			Normal:  maxVolume * 0.95,
			Warning: maxVolume * 0.02,
			Danger:  maxVolume * 0.03,
		},
	}

	regionNodes := []Node{regionInternetNode, meshNode}
	regionConnections := []Connection{regionInternetConnection}

	result = Config{
		Renderer:    "global",
		Name:        "edge",
		Nodes:       regionNodes,
		Connections: regionConnections,
	}
	return result
}

func walk(t *tree.Tree, nodes *[]Node, connections *[]Connection, volume *float64) {
	name := fmt.Sprintf("%v (%v)", t.Name, t.Version)
	displayName := fmt.Sprintf("%v (%v)", strings.Split(t.Name, ".")[0], t.Version)
	n := Node{
		Renderer:    "focusedChild",
		Name:        name,
		DisplayName: displayName,
		Notices: []Notice{
			{
				Title: "Prometheus Graph",
				Link:  t.Query,
			}},
	}
	*nodes = append(*nodes, n)

	var parentName string
	if nil == t.Parent {
		parentName = "INTERNET"
	} else {
		parentName = fmt.Sprintf("%v (%v)", t.Parent.Name, t.Parent.Version)
	}
	// TODO: because bookdemo basically always works, introduce some errors (3% error, 2% warning)
	c := Connection{
		Source: parentName,
		Target: name,
		Metrics: Metrics{
			//Normal:  t.Normal,
			//Warning: t.Warning,
			//Danger:  t.Danger,
			Normal:  t.Normal * 0.95,
			Warning: t.Normal * 0.02,
			Danger:  t.Normal * 0.03,
		},
	}
	*connections = append(*connections, c)

	*volume += t.Normal
	*volume += t.Warning
	*volume += t.Danger

	for _, c := range t.Children {
		walk(c, nodes, connections, volume)
	}
}
