package ws // import "github.com/andrewarchi/nebula/ws"

import (
	"fmt"
	"go/token"
	"math/big"

	"github.com/andrewarchi/nebula/bigint"
	"github.com/andrewarchi/nebula/ir"
)

// ConvertSSA converts tokens into Nebula IR in SSA form.
func (p *Program) ConvertSSA() (*ir.Program, error) {
	if needsImplicitEnd(p.Tokens) {
		p.Tokens = append(p.Tokens, Token{Type: End})
	}
	irp, branches, labels, err := p.createBlocks()
	if err != nil {
		return nil, err
	}
	if err := irp.ConnectEdges(branches, labels); err != nil {
		return irp, err
	}
	return irp, nil
}

func needsImplicitEnd(tokens []Token) bool {
	if len(tokens) == 0 {
		return true
	}
	switch tokens[len(tokens)-1].Type {
	case Jmp, Ret, End:
		return false
	}
	return true
}

func (p *Program) createBlocks() (*ir.Program, []*big.Int, *bigint.Map, error) {
	irp := &ir.Program{
		Name:      p.File.Name(),
		ConstVals: bigint.NewMap(),
		File:      p.File,
	}
	var branches []*big.Int
	labels := bigint.NewMap()           // map[*big.Int]int
	labelUses := getLabelUses(p.Tokens) // map[*big.Int]struct{}
	prevLabel := ""
	labelIndex := 0

	for i := 0; i < len(p.Tokens); i++ {
		var block ir.BasicBlock
		block.ID = len(irp.Blocks)
		block.Stack.LoadHandler = block.AppendNode
		if len(irp.Blocks) > 0 {
			prev := irp.Blocks[len(irp.Blocks)-1]
			prev.Next = &block
			block.Prev = prev
		}

		if p.Tokens[i].Type != Label && i != 0 && prevLabel != "" {
			labelIndex++
			block.Labels = append(block.Labels, ir.Label{ID: nil, Name: fmt.Sprintf("%s%d", prevLabel, labelIndex)})
		}
		for p.Tokens[i].Type == Label {
			label := p.Tokens[i].Arg
			if labels.Put(label, len(irp.Blocks)) {
				return nil, nil, nil, fmt.Errorf("ir: label is not unique: %s", label)
			}
			var name string
			if p.LabelNames != nil {
				if n, ok := p.LabelNames.Get(label); ok {
					name = n.(string)
				}
			}
			prevLabel = name
			labelIndex = 0
			block.Labels = append(block.Labels, ir.Label{ID: label, Name: name})
			i++
		}

		checkStack := ir.NewCheckStackStmt(-1, -1) // TODO source position
		block.AppendNode(checkStack)

		var branch *big.Int
		for ; i < len(p.Tokens); i++ {
			branch = appendInstruction(irp, &block, p.Tokens[i], labelUses)
			if block.Terminator != nil {
				if p.Tokens[i].Type == Label {
					i--
				}
				break
			}
		}

		if block.Stack.Access > 0 {
			checkStack.StackSize = block.Stack.Access
		} else {
			block.Nodes = block.Nodes[1:]
		}

		irp.Blocks = append(irp.Blocks, &block)
		branches = append(branches, branch)
	}
	irp.Entry = irp.Blocks[0]
	irp.NextBlockID = len(irp.Blocks)
	return irp, branches, labels, nil
}

func getLabelUses(tokens []Token) *bigint.Map {
	labelUses := bigint.NewMap() // map[*big.Int]struct{}
	for _, token := range tokens {
		switch token.Type {
		case Call, Jmp, Jz, Jn:
			labelUses.Put(token.Arg, nil)
		}
	}
	return labelUses
}

func appendInstruction(p *ir.Program, block *ir.BasicBlock, tok Token, labelUses *bigint.Map) *big.Int {
	stack := &block.Stack
	switch tok.Type {
	case Push:
		stack.Push(p.LookupConst(tok.Arg, tok.Start))
	case Dup:
		stack.Dup()
	case Copy:
		n, ok := bigint.ToInt(tok.Arg)
		if !ok {
			panic(fmt.Sprintf("ws: copy argument overflow: %v", tok.Arg))
		} else if n < 0 {
			panic(fmt.Sprintf("ws: copy argument negative: %v", tok.Arg))
		}
		stack.Copy(n)
	case Swap:
		stack.Swap()
	case Drop:
		stack.Drop()
	case Slide:
		n, ok := bigint.ToInt(tok.Arg)
		if !ok {
			panic(fmt.Sprintf("ws: slide argument overflow: %v", tok.Arg))
		} else if n < 0 {
			panic(fmt.Sprintf("ws: slide argument negative: %v", tok.Arg))
		}
		stack.Slide(n)

	case Add:
		appendBinary(block, stack, ir.Add, tok.Start)
	case Sub:
		appendBinary(block, stack, ir.Sub, tok.Start)
	case Mul:
		appendBinary(block, stack, ir.Mul, tok.Start)
	case Div:
		appendBinary(block, stack, ir.Div, tok.Start)
	case Mod:
		appendBinary(block, stack, ir.Mod, tok.Start)

	case Store:
		val, addr := stack.Pop(), stack.Pop()
		block.AppendNode(ir.NewStoreHeapStmt(addr, val, tok.Start))
	case Retrieve:
		addr := stack.Pop()
		load := ir.NewLoadHeapExpr(addr, tok.Start)
		stack.Push(load)
		block.AppendNode(load)

	case Label:
		if _, ok := labelUses.Get(tok.Arg); ok { // split blocks at used labels
			block.Terminator = ir.NewJmpTerm(ir.Fallthrough, nil, tok.Start)
			return tok.Arg
		}
	case Call:
		block.Terminator = ir.NewCallTerm(nil, nil, tok.Start)
		return tok.Arg
	case Jmp:
		block.Terminator = ir.NewJmpTerm(ir.Jmp, nil, tok.Start)
		return tok.Arg
	case Jz:
		block.Terminator = ir.NewJmpCondTerm(ir.Jz, stack.Pop(), nil, nil, tok.Start)
		return tok.Arg
	case Jn:
		block.Terminator = ir.NewJmpCondTerm(ir.Jn, stack.Pop(), nil, nil, tok.Start)
		return tok.Arg
	case Ret:
		block.Terminator = ir.NewRetTerm(tok.Start)
	case End:
		block.Terminator = ir.NewExitTerm(tok.Start)

	case Printc:
		block.AppendNode(ir.NewPrintStmt(ir.Printc, stack.Pop(), tok.Start))
		block.AppendNode(ir.NewFlushStmt(tok.Start))
	case Printi:
		block.AppendNode(ir.NewPrintStmt(ir.Printi, stack.Pop(), tok.Start))
		block.AppendNode(ir.NewFlushStmt(tok.Start))
	case Readc:
		appendRead(block, stack, ir.Readc, tok.Start)
	case Readi:
		appendRead(block, stack, ir.Readi, tok.Start)

	default:
		panic(fmt.Sprintf("ws: unrecognized token type: %v", tok.Type))
	}
	return nil
}

func appendBinary(block *ir.BasicBlock, stack *ir.Stack, op ir.BinaryOp, pos token.Pos) {
	rhs, lhs := stack.Pop(), stack.Pop()
	bin := ir.NewBinaryExpr(op, lhs, rhs, pos)
	stack.Push(bin)
	block.AppendNode(bin)
}

func appendRead(block *ir.BasicBlock, stack *ir.Stack, op ir.ReadOp, pos token.Pos) {
	addr := stack.Pop()
	read := ir.NewReadExpr(op, pos)
	store := ir.NewStoreHeapStmt(addr, read, pos)
	block.AppendNode(read)
	block.AppendNode(store)
}
