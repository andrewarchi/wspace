package ast

import "github.com/andrewarchi/wspace/token"

// Dependent returns whether two non-branching nodes are dependent. True
// is returned when node B is dependent on node A. Nodes are dependent
// when both are I/O instructions, one is I/O and the other can throw,
// both assign to the same value, or one reads the value assigned to by
// the other. Dependent is reflexive.
func Dependent(a, b Node) bool {
	aIO, bIO := isIO(a), isIO(b)
	return aIO && bIO ||
		aIO && canThrow(b) || bIO && canThrow(a) ||
		references(a, b) || references(b, a)
}

func isIO(node Node) bool {
	switch node.(type) {
	case *PrintStmt, *ReadExpr:
		return true
	}
	return false
}

// canThrow returns whether the node is a division with a non-constant
// RHS.
func canThrow(node Node) bool {
	if n, ok := node.(*BinaryExpr); ok && n.Op == token.Div {
		_, ok := n.RHS.(*ConstVal)
		return ok
	}
	return false
}

// references returns whether node B references the assignment of
// node A.
func references(a, b Node) bool {
	assign := Assign(a)
	switch expr := b.(type) {
	case *UnaryExpr:
		return expr.Assign == assign || expr.Val == assign
	case *BinaryExpr:
		return expr.Assign == assign || expr.LHS == assign || expr.RHS == assign
	case *PrintStmt:
		return expr.Val == assign
	case *ReadExpr:
		return expr.Assign == assign
	}
	return false
}
