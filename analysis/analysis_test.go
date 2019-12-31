package analysis // import "github.com/andrewarchi/nebula/analysis"

import (
	"math/big"
	"reflect"
	"testing"

	"github.com/andrewarchi/nebula/bigint"
	"github.com/andrewarchi/nebula/ir"
	"github.com/andrewarchi/nebula/token"
)

func TestTransforms(t *testing.T) {
	// push 1    ; 0
	// push 3    ; 1
	// push 10   ; 2
	// push 2    ; 3
	// mul       ; 4
	// add       ; 5
	// swap      ; 6
	// push 'C'  ; 7
	// dup       ; 8
	// copy 2    ; 9
	// sub       ; 10
	// push -32  ; 11
	// push 'a'  ; 12
	// add       ; 13
	// printc    ; 14
	// printc    ; 15
	// printc    ; 16
	// printi    ; 17
	// printi    ; 18

	tokens := []token.Token{
		{Type: token.Push, Arg: big.NewInt(1)},   // 0
		{Type: token.Push, Arg: big.NewInt(3)},   // 1
		{Type: token.Push, Arg: big.NewInt(10)},  // 2
		{Type: token.Push, Arg: big.NewInt(2)},   // 3
		{Type: token.Mul},                        // 4
		{Type: token.Add},                        // 5
		{Type: token.Swap},                       // 6
		{Type: token.Push, Arg: big.NewInt('C')}, // 7
		{Type: token.Dup},                        // 8
		{Type: token.Copy, Arg: big.NewInt(2)},   // 9
		{Type: token.Sub},                        // 10
		{Type: token.Push, Arg: big.NewInt(-32)}, // 11
		{Type: token.Push, Arg: big.NewInt('a')}, // 12
		{Type: token.Add},                        // 13
		{Type: token.Printc},                     // 14
		{Type: token.Printc},                     // 15
		{Type: token.Printc},                     // 16
		{Type: token.Printi},                     // 17
		{Type: token.Printi},                     // 18
	}

	v1 := ir.Val(&ir.ConstVal{Val: big.NewInt(1)})
	v2 := ir.Val(&ir.ConstVal{Val: big.NewInt(2)})
	v3 := ir.Val(&ir.ConstVal{Val: big.NewInt(3)})
	v10 := ir.Val(&ir.ConstVal{Val: big.NewInt(10)})
	v20 := ir.Val(&ir.ConstVal{Val: big.NewInt(20)})
	v23 := ir.Val(&ir.ConstVal{Val: big.NewInt(23)})
	vn32 := ir.Val(&ir.ConstVal{Val: big.NewInt(-32)})
	vA := ir.Val(&ir.ConstVal{Val: big.NewInt('A')})
	vB := ir.Val(&ir.ConstVal{Val: big.NewInt('B')})
	vC := ir.Val(&ir.ConstVal{Val: big.NewInt('C')})
	va := ir.Val(&ir.ConstVal{Val: big.NewInt('a')})
	vABC123 := ir.Val(&ir.StringVal{Val: "ABC123"})

	var stack ir.Stack
	stack.Push(&v1)   // 0
	stack.Push(&v3)   // 1
	stack.Push(&v10)  // 2
	stack.Push(&v2)   // 3
	stack.Pop()       // 4
	stack.Pop()       // 4
	stack.Push(&v20)  // 4
	stack.Pop()       // 5
	stack.Pop()       // 5
	stack.Push(&v23)  // 5
	stack.Swap()      // 6
	stack.Push(&vC)   // 7
	stack.Dup()       // 8
	stack.Copy(2)     // 9
	stack.Pop()       // 10
	stack.Pop()       // 10
	stack.Push(&vB)   // 10
	stack.Push(&vn32) // 11
	stack.Push(&va)   // 12
	stack.Pop()       // 13
	stack.Pop()       // 13
	stack.Push(&vA)   // 13
	stack.Pop()       // 14
	stack.Pop()       // 15
	stack.Pop()       // 16
	stack.Pop()       // 17
	stack.Pop()       // 18

	if len(stack.Vals) != 0 || stack.Pops != 0 || stack.Access != 0 {
		t.Errorf("stack should be empty and not underflow, got %v", stack)
	}

	constVals := bigint.NewMap(nil)
	constVals.Put(big.NewInt(1), &v1)
	constVals.Put(big.NewInt(3), &v3)
	constVals.Put(big.NewInt(10), &v10)
	constVals.Put(big.NewInt(2), &v2)
	constVals.Put(big.NewInt(20), &v20)
	constVals.Put(big.NewInt(23), &v23)
	constVals.Put(big.NewInt('C'), &vC)
	constVals.Put(big.NewInt('B'), &vB)
	constVals.Put(big.NewInt(-32), &vn32)
	constVals.Put(big.NewInt('a'), &va)
	constVals.Put(big.NewInt('A'), &vA)

	blockConst := &ir.BasicBlock{
		Stack: stack,
		Nodes: []ir.Node{
			&ir.PrintStmt{Op: token.Printc, Val: &vA},
			&ir.PrintStmt{Op: token.Printc, Val: &vB},
			&ir.PrintStmt{Op: token.Printc, Val: &vC},
			&ir.PrintStmt{Op: token.Printi, Val: &v1},
			&ir.PrintStmt{Op: token.Printi, Val: &v23},
		},
		Terminator: &ir.EndStmt{},
		Entries:    []*ir.BasicBlock{ir.EntryBlock},
		Callers:    []*ir.BasicBlock{ir.EntryBlock},
	}
	programConst := &ir.Program{
		Name:        "test",
		Blocks:      []*ir.BasicBlock{blockConst},
		Entry:       blockConst,
		ConstVals:   *constVals,
		NextBlockID: 1,
		NextStackID: 0,
	}

	program, err := ir.Parse(tokens, nil, "test")
	if err != nil {
		t.Errorf("unexpected parse error: %v", err)
	}
	if !reflect.DeepEqual(program, programConst) {
		t.Errorf("constant arithmetic folding not equal\ngot:\n%v\nwant:\n%v", program, programConst)
	}

	blockStr := &ir.BasicBlock{
		Nodes: []ir.Node{
			&ir.PrintStmt{Op: token.Prints, Val: &vABC123},
		},
		Terminator: &ir.EndStmt{},
		Stack:      stack,
		Entries:    []*ir.BasicBlock{ir.EntryBlock},
		Callers:    []*ir.BasicBlock{ir.EntryBlock},
	}
	programStr := &ir.Program{
		Name:        "test",
		Blocks:      []*ir.BasicBlock{blockStr},
		Entry:       blockStr,
		ConstVals:   *constVals,
		NextBlockID: 1,
		NextStackID: 0,
	}

	ConcatStrings(program)
	if !reflect.DeepEqual(program, programStr) {
		t.Errorf("string concat not equal\ngot:\n%v\nwant:\n%v", program, programStr)
	}
}