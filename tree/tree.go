// tree.go
package tree

type Tree struct {
	Query    string
	Name     string
	Version  string
	Normal   float64
	Warning  float64
	Danger   float64
	Parent   *Tree
	Children []*Tree
}
