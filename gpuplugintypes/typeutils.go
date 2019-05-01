package gpuplugintypes

import (
	"bytes"
	"fmt"

	"github.com/Microsoft/KubeDevice-API/pkg/utils"
)

func findNodeInsertionPoint(node *SortedTreeNode, valToAdd int, score float64) int {
	insertionPoint := len(node.Child) // if nothing found, insert at end
	for index, childNode := range node.Child {
		if childNode.Val < valToAdd || ((childNode.Val == valToAdd) && (childNode.Score < score)) {
			insertionPoint = index
			break
		}
	}
	node.Child = append(node.Child, nil)
	for i := len(node.Child) - 1; i > insertionPoint; i-- {
		node.Child[i] = node.Child[i-1]
	}
	return insertionPoint
}

// AddToSortedTreeNode adds value as child of node
// values are in descending order
func AddToSortedTreeNodeWithScore(node *SortedTreeNode, valToAdd int, score float64) *SortedTreeNode {
	insertionPoint := findNodeInsertionPoint(node, valToAdd, score)
	node.Child[insertionPoint] = &SortedTreeNode{Val: valToAdd, Score: score, Child: nil}
	return node.Child[insertionPoint]
}

func AddNodeToSortedTreeNode(node *SortedTreeNode, nodeToAdd *SortedTreeNode) {
	insertionPoint := findNodeInsertionPoint(node, nodeToAdd.Val, nodeToAdd.Score)
	node.Child[insertionPoint] = nodeToAdd
}

func AddToSortedTreeNode(node *SortedTreeNode, valToAdd int) *SortedTreeNode {
	return AddToSortedTreeNodeWithScore(node, valToAdd, 0.0)
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

func logTreeNode(buffer *bytes.Buffer, node *SortedTreeNode, level int) {
	for i := 0; i < 3*level; i++ {
		buffer.WriteString(" ")
	}
	buffer.WriteString(fmt.Sprintf("%d\n", node.Val))
	for _, child := range node.Child {
		logTreeNode(buffer, child, level+1)
	}
}

func LogTreeNode(loglevel int, node *SortedTreeNode) {
	if utils.Logb(loglevel) {
		var buffer bytes.Buffer
		logTreeNode(&buffer, node, 0)
		utils.Logf(loglevel, buffer.String())
	}
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
