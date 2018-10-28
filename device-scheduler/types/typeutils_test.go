package types

import (
	"testing"
)

func TestSortedTreeNode(t *testing.T) {
	root := &SortedTreeNode{Val: 10, Child: nil}
	child0 := AddToSortedTreeNode(root, 4)
	child1 := AddToSortedTreeNode(root, 8)
	AddToSortedTreeNode(child0, 3)
	AddToSortedTreeNode(child0, 1)
	AddToSortedTreeNode(child1, 1)
	AddToSortedTreeNode(child1, 4)
	AddToSortedTreeNode(child1, 3)
	//fmt.Printf("Tree: %v", root)
	//PrintTreeNode(root)
	expectedTree := &SortedTreeNode{Val: 10, Child: []*SortedTreeNode{
		{Val: 8, Child: []*SortedTreeNode{
			{Val: 4, Child: nil},
			{Val: 3, Child: nil},
			{Val: 1, Child: nil},
		}},
		{Val: 4, Child: []*SortedTreeNode{
			{Val: 3, Child: nil},
			{Val: 1, Child: nil},
		}},
	}}
	if !CompareTreeNode(root, expectedTree) {
		PrintTreeNode(root)
		PrintTreeNode(expectedTree)
		t.Errorf("Trees not equal\n")
	}
}
