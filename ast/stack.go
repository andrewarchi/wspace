package ast

import "fmt"

// Stack represents the Whitespace stack for registerization. Values
// from outside the current basic block are represented as negative
// numbers with the upper bound of Low.
type Stack struct {
	Vals []int
	Next int
	Low  int
}

// NewStack constructs a stack.
func NewStack() *Stack {
	return &Stack{nil, 0, -1}
}

// Push pushes an item to the stack and returns the id of the inserted
// item.
func (s *Stack) Push() *StackVal {
	n := s.Next
	s.Vals = append(s.Vals, s.Next)
	s.Next++
	return &StackVal{n}
}

// Pop pops an item from the stack and returns the id of the removed
// item.
func (s *Stack) Pop() *StackVal {
	var val int
	if len(s.Vals) == 0 {
		val = s.Low
		s.Low--
	} else {
		val = s.Vals[len(s.Vals)-1]
		s.Vals = s.Vals[:len(s.Vals)-1]
	}
	return &StackVal{val}
}

// Dup pushes a copy of the top item to the stack without creating an
// id.
func (s *Stack) Dup() *StackVal {
	val := s.Top()
	s.Vals = append(s.Vals, val.Val)
	return val
}

// Copy pushes a copy of the nth item to the stack without creating an
// id.
func (s *Stack) Copy(n int) *StackVal {
	if n < 0 {
		panic(fmt.Sprintf("ast: copy index must be positive: %d", n))
	}
	val := s.Nth(n)
	s.Vals = append(s.Vals, val.Val)
	return val
}

// Swap swaps the top two items on the stack.
func (s *Stack) Swap() {
	l := len(s.Vals)
	switch l {
	case 0:
		s.Vals = append(s.Vals, s.Low, s.Low-1)
		s.Low--
	case 1:
		s.Vals = append(s.Vals, s.Low)
		s.Low--
	default:
		s.Vals[l-2], s.Vals[l-1] = s.Vals[l-1], s.Vals[l-2]
	}
}

// Slide slides n items off the stack, leaving the top item.
func (s *Stack) Slide(n int) {
	l := len(s.Vals)
	switch {
	case n < 0:
		panic(fmt.Sprintf("ast: slide count must be positive: %d", n))
	case n == 0:
		return
	case l == 0:
		s.Vals = append(s.Vals, s.Low)
		s.Low -= n
	case n < l:
		s.Vals = append(s.Vals[:l-n-1], s.Vals[l-1])
	default:
		s.Vals = append(s.Vals[:0], s.Vals[l-1])
		s.Low -= n - l
	}
}

// Top returns the id of the top item on the stack.
func (s *Stack) Top() *StackVal {
	if len(s.Vals) != 0 {
		return &StackVal{s.Vals[len(s.Vals)-1]}
	}
	return &StackVal{s.Low}
}

// Nth returns the id of the nth item on the stack.
func (s *Stack) Nth(n int) *StackVal {
	if n < len(s.Vals) {
		return &StackVal{s.Vals[len(s.Vals)-n-1]}
	}
	return &StackVal{s.Low - n + len(s.Vals)}
}
