package types

import "fmt"

// AddToSortedTreeNode adds value as child of node
// values are in descending order
func AddToSortedTreeNode(node *SortedTreeNode, valToAdd int) *SortedTreeNode {
	insertionPoint := len(node.Child) // if nothing found, insert at end
	for index, childNode := range node.Child {
		if childNode.Val < valToAdd {
			insertionPoint = index
			break
		}
	}
	node.Child = append(node.Child, nil)
	for i := len(node.Child) - 1; i > insertionPoint; i-- {
		node.Child[i] = node.Child[i-1]
	}
	node.Child[insertionPoint] = &SortedTreeNode{Val: valToAdd, Child: nil}
	return node.Child[insertionPoint]
}

func printTreeNode(node *SortedTreeNode, level int) {
	for i := 0; i < 3*level; i++ {
		fmt.Printf(" ")
	}
	fmt.Printf("%d\n", node.Val)
	for _, child := range node.Child {
		printTreeNode(child, level+1)
	}
}

func PrintTreeNode(node *SortedTreeNode) {
	printTreeNode(node, 0)
}

// returns true if same
func CompareTreeNode(node1 *SortedTreeNode, node2 *SortedTreeNode) bool {
	if node1 == nil && node2 == nil {
		return true
	}
	if node1 == nil || node2 == nil {
		return false
	}
	if node1.Val != node2.Val {
		return false
	}
	if len(node1.Child) != len(node2.Child) {
		return false
	}
	allSame := true
	for i := 0; i < len(node1.Child); i++ {
		allSame = allSame && CompareTreeNode(node1.Child[i], node2.Child[i])
	}
	return allSame
}
