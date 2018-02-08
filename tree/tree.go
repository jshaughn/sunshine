// tree.go
package tree

// Tree is basically a [sub-]tree node
type Tree struct {
	Name     string
	Version  string
	Parent   *Tree
	Children []*Tree
	Metadata map[string]interface{}
}
