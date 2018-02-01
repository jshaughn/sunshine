// tree.go
package tree

type Tree struct {
	Name     string
	Version  string
	Parent   *Tree
	Children []*Tree
}
