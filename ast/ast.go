package ast

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/andrewarchi/wspace/bigint"
	"github.com/andrewarchi/wspace/token"
)

// AST is a set of interconnected basic blocks.
type AST []*BasicBlock

// BasicBlock is a list of consecutive non-branching instructions in a
// program followed by a branch.
type BasicBlock struct {
	Labels  []*big.Int
	Nodes   []Node
	Edge    FlowStmt
	Callers []*BasicBlock
}

// Node can be any expr or stmt type.
type Node interface {
	String() string
}

// Val can be StackVal, HeapVal, ConstVal, or AddrVal.
type Val = Node

// StackVal is a position on the stack.
type StackVal struct{ Val int }

// ConstVal is a constant value such as from push or an expression with
// constant operands.
type ConstVal struct{ Val *big.Int }

// StringVal is a sequence of characters.
type StringVal struct{ Val string }

// AddrVal marks a value as being a pointer to a value.
type AddrVal struct{ Val Val }

// UnaryExpr evaluates a unary operation and assigns the result to an
// address. Valid operations are push, store, and retrieve.
type UnaryExpr struct {
	Op     token.Type
	Assign Val
	Val    Val
}

// BinaryExpr evalutates a binary operation and assigns the result to an
// address. Valid operations are add, sub, mul, div, and mod.
type BinaryExpr struct {
	Op     token.Type
	Assign Val
	LHS    Val
	RHS    Val
}

// PrintStmt prints a value. Valid operations are printc and printi.
type PrintStmt struct {
	Op  token.Type
	Val Val
}

// ReadExpr reads a value to an address. Valid operations are readc and
// readi.
type ReadExpr struct {
	Op     token.Type
	Assign Val
}

// FlowStmt can be JmpStmt, JmpCondStmt, RetStmt, EndStmt.
type FlowStmt = Node

// JmpStmt unconditionally jumps to a block. Valid instructions are
// call, jmp, and fallthrough.
type JmpStmt struct {
	Op    token.Type
	Block *BasicBlock
}

// JmpCondStmt conditionally jumps to a block based on a value. Valid
// instructions are jz and jn.
type JmpCondStmt struct {
	Op         token.Type
	Val        Val
	TrueBlock  *BasicBlock
	FalseBlock *BasicBlock
}

// RetStmt represents a ret.
type RetStmt struct{}

// EndStmt represents an end.
type EndStmt struct{}

// Parse parses tokens into an AST of basic blocks.
func Parse(tokens []token.Token) (AST, error) {
	if needsImplicitEnd(tokens) {
		tokens = append(tokens, token.Token{Type: token.End})
	}
	ast, branches, labels, err := parseBlocks(tokens)
	if err != nil {
		return nil, err
	}
	if err := connectBlockEdges(ast, branches, labels); err != nil {
		return nil, err
	}
	return ast, nil
}

func needsImplicitEnd(tokens []token.Token) bool {
	if len(tokens) == 0 {
		return true
	}
	switch tokens[len(tokens)-1].Type {
	case token.Call, token.Jmp, token.Ret, token.End:
		return false
	}
	return true
}

func parseBlocks(tokens []token.Token) (AST, []*big.Int, *bigint.Map, error) {
	var ast AST
	var branches []*big.Int
	labels := bigint.NewMap(nil) // map[*big.Int]int
	for i := 0; i < len(tokens); i++ {
		var block BasicBlock
		for tokens[i].Type == token.Label {
			label := tokens[i].Arg
			if labels.Put(label, len(ast)) {
				return nil, nil, nil, fmt.Errorf("ast: label is not unique: %s", label)
			}
			block.Labels = append(block.Labels, label)
			i++
		}

		var branch *big.Int
		stack := NewStack()
		for ; i < len(tokens); i++ {
			block.Nodes, block.Edge, branch = tokenToNode(block.Nodes, tokens[i], stack)
			if block.Edge != nil {
				if tokens[i].Type == token.Label {
					i--
				}
				break
			}
		}

		ast = append(ast, &block)
		branches = append(branches, branch)
	}
	return ast, branches, labels, nil
}

func connectBlockEdges(ast AST, branches []*big.Int, labels *bigint.Map) error {
	for i, block := range ast {
		branch := branches[i]
		if branch != nil {
			label, ok := labels.Get(branch)
			if !ok {
				return fmt.Errorf("ast: block %s jumps to non-existant label: %v", block.Name(), branch)
			}
			callee := ast[label.(int)]
			callee.Callers = append(callee.Callers, block)

			switch edge := block.Edge.(type) {
			case *JmpStmt:
				edge.Block = callee
			case *JmpCondStmt:
				if i >= len(ast) {
					panic("ast: program ends with conditional jump")
				}
				edge.TrueBlock = callee
				edge.FalseBlock = ast[i+1]
			case *RetStmt, *EndStmt:
			default:
				panic(fmt.Sprintf("ast: invalid edge type: %T", block.Edge))
			}
		}
	}
	return nil
}

func tokenToNode(nodes []Node, tok token.Token, stack *Stack) ([]Node, FlowStmt, *big.Int) {
	switch tok.Type {
	case token.Push:
		return append(nodes, &UnaryExpr{
			Op:     token.Push,
			Assign: stack.Push(),
			Val:    &ConstVal{tok.Arg},
		}), nil, nil
	case token.Dup:
		stack.Dup()
	case token.Copy:
		n, ok := bigint.ToInt(tok.Arg)
		if !ok {
			panic(fmt.Sprintf("ast: copy argument out of range: %v", tok.Arg))
		}
		stack.Copy(n)
	case token.Swap:
		stack.Swap()
	case token.Drop:
		stack.Pop()
	case token.Slide:
		n, ok := bigint.ToInt(tok.Arg)
		if !ok {
			panic(fmt.Sprintf("ast: slide argument out of range: %v", tok.Arg))
		}
		stack.Slide(n)

	case token.Add, token.Sub, token.Mul, token.Div, token.Mod:
		rhs, lhs, assign := stack.Pop(), stack.Pop(), stack.Push()
		return append(nodes, &BinaryExpr{
			Op:     tok.Type,
			Assign: assign,
			LHS:    lhs,
			RHS:    rhs,
		}), nil, nil

	case token.Store:
		val, assign := stack.Pop(), stack.Pop()
		return append(nodes, &UnaryExpr{
			Op:     token.Store,
			Assign: &AddrVal{assign},
			Val:    val,
		}), nil, nil
	case token.Retrieve:
		val, assign := stack.Pop(), stack.Push()
		return append(nodes, &UnaryExpr{
			Op:     token.Retrieve,
			Assign: assign,
			Val:    val,
		}), nil, nil

	case token.Label:
		return nodes, &JmpStmt{Op: token.Fallthrough}, tok.Arg
	case token.Call, token.Jmp:
		return nodes, &JmpStmt{Op: tok.Type}, tok.Arg
	case token.Jz, token.Jn:
		return nodes, &JmpCondStmt{
			Op:  tok.Type,
			Val: stack.Pop(),
		}, tok.Arg
	case token.Ret:
		return nodes, &RetStmt{}, nil
	case token.End:
		return nodes, &EndStmt{}, nil
	case token.Fallthrough:
		panic("ast: unexpected fallthrough")

	case token.Printc, token.Printi:
		return append(nodes, &PrintStmt{
			Op:  tok.Type,
			Val: stack.Pop(),
		}), nil, nil
	case token.Readc, token.Readi:
		return append(nodes, &ReadExpr{
			Op:     tok.Type,
			Assign: &AddrVal{stack.Pop()},
		}), nil, nil

	default:
		panic(fmt.Sprintf("ast: illegal token: %v", tok.Type))
	}
	return nodes, nil, nil
}

// Name returns the name of the basic block from either the first label
// or the block address.
func (block *BasicBlock) Name() string {
	if block == nil {
		return "<nil>"
	}
	if len(block.Labels) != 0 {
		return fmt.Sprintf("label_%v", block.Labels[0])
	}
	return fmt.Sprintf("%p", block)
}

func (ast AST) String() string {
	var b strings.Builder
	for i, block := range ast {
		if i != 0 {
			b.WriteByte('\n')
		}
		b.WriteString(block.String())
		b.WriteByte('\n')
	}
	return b.String()
}

func (block *BasicBlock) String() string {
	var b strings.Builder
	if len(block.Labels) == 0 {
		fmt.Fprintf(&b, "%p:\n", block)
	}
	for _, label := range block.Labels {
		b.WriteString("label_")
		b.WriteString(label.String())
		b.WriteString(":\n")
	}
	for _, node := range block.Nodes {
		b.WriteString("    ")
		b.WriteString(node.String())
		b.WriteByte('\n')
	}
	b.WriteString("    ")
	b.WriteString(block.Edge.String())
	return b.String()
}

func (s *StackVal) String() string  { return fmt.Sprintf("%%%d", s.Val) }
func (c *ConstVal) String() string  { return fmt.Sprintf("%v", c.Val) }
func (s *StringVal) String() string { return fmt.Sprintf("%q", s.Val) }
func (a *AddrVal) String() string   { return fmt.Sprintf("*%v", a.Val) }
func (u *UnaryExpr) String() string { return fmt.Sprintf("%v = %v %v", u.Assign, u.Op, u.Val) }
func (b *BinaryExpr) String() string {
	return fmt.Sprintf("%v = %v %v %v", b.Assign, b.Op, b.LHS, b.RHS)
}
func (p *PrintStmt) String() string { return fmt.Sprintf("%v %v", p.Op, p.Val) }
func (r *ReadExpr) String() string  { return fmt.Sprintf("%v = %v", r.Assign, r.Op) }
func (j *JmpStmt) String() string   { return fmt.Sprintf("%v %s", j.Op, j.Block.Name()) }
func (j *JmpCondStmt) String() string {
	return fmt.Sprintf("%v %v %s %s", j.Op, j.Val, j.TrueBlock.Name(), j.FalseBlock.Name())
}
func (*RetStmt) String() string { return "ret" }
func (*EndStmt) String() string { return "end" }