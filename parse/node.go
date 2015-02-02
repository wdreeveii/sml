// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Parse nodes.

package parse

import (
	"bytes"
	"fmt"
	"strconv"
	//"strings"
)

var textFormat = "%s" // Changed to "%q" in tests for better error messages.

// A Node is an element in the parse tree. The interface is trivial.
// The interface contains an unexported method so that only
// types local to this package can satisfy it.
type Node interface {
	Reduce() (Node, error)
	Type() NodeType
	String() string
	// Copy does a deep copy of the Node and all its components.
	// To avoid type assertions, some XxxNodes also have specialized
	// CopyXxx methods that return *XxxNode.
	Copy() Node
	Position() Pos // byte position of start of node in full original input string
	// tree returns the containing *Tree.
	// It is unexported so all implementations of Node are in this package.
	tree() *Tree
}

// NodeType identifies the type of a parse tree node.
type NodeType int

// Pos represents a byte position in the original input text from which
// this template was parsed.
type Pos int

func (p Pos) Position() Pos {
	return p
}

// Type returns itself and provides an easy default implementation
// for embedding in a Node. Embedded in all non-trivial Nodes.
func (t NodeType) Type() NodeType {
	return t
}

const (
	NodeText         NodeType = iota // Plain text.
	NodeList                         // A list of Nodes.
	NodeNumber                       // A numerical constant.
	NodeDiff                         // A diff operator
	NodeIntersection                 // An intersection operator
	NodeUnion                        // A union operator
	NodeObject                       // A object declaration
)

// Nodes.

// ListNode holds a sequence of nodes.
type ListNode struct {
	NodeType
	Pos
	tr    *Tree
	Nodes []Node // The element nodes in lexical order.
}

func (t *Tree) newList(pos Pos) *ListNode {
	return &ListNode{tr: t, NodeType: NodeList, Pos: pos}
}

func (t *ListNode) Reduce() (ret Node, err error) {
	var list *ListNode = t.CopyList()
	for i, v := range list.Nodes {
		list.Nodes[i], err = v.Reduce()
		if err != nil {
			return
		}
	}
	ret = list
	return
}

func (l *ListNode) append(n Node) {
	l.Nodes = append(l.Nodes, n)
}

func (l *ListNode) tree() *Tree {
	return l.tr
}

func (l *ListNode) String() string {
	b := new(bytes.Buffer)
	for _, n := range l.Nodes {
		fmt.Fprint(b, "(", n, ")")
	}
	return b.String()
}

func (l *ListNode) CopyList() *ListNode {
	if l == nil {
		return l
	}
	n := l.tr.newList(l.Pos)
	for _, elem := range l.Nodes {
		n.append(elem.Copy())
	}
	return n
}

func (l *ListNode) Copy() Node {
	return l.CopyList()
}

// NumberNode holds a number: signed or unsigned integer, float, or complex.
// The value is parsed and stored under all the types that can represent the value.
// This simulates in a small amount of code the behavior of Go's ideal constants.
type NumberNode struct {
	NodeType
	Pos
	tr         *Tree
	IsInt      bool       // Number has an integral value.
	IsUint     bool       // Number has an unsigned integral value.
	IsFloat    bool       // Number has a floating-point value.
	IsComplex  bool       // Number is complex.
	Int64      int64      // The signed integer value.
	Uint64     uint64     // The unsigned integer value.
	Float64    float64    // The floating-point value.
	Complex128 complex128 // The complex value.
	Text       string     // The original textual representation from the input.
}

func (t *Tree) newNumber(pos Pos, text string, typ itemType) (*NumberNode, error) {
	n := &NumberNode{tr: t, NodeType: NodeNumber, Pos: pos, Text: text}
	switch typ {
	case itemComplex:
		// fmt.Sscan can parse the pair, so let it do the work.
		if _, err := fmt.Sscan(text, &n.Complex128); err != nil {
			return nil, err
		}
		n.IsComplex = true
		n.simplifyComplex()
		return n, nil
	}
	// Imaginary constants can only be complex unless they are zero.
	if len(text) > 0 && text[len(text)-1] == 'i' {
		f, err := strconv.ParseFloat(text[:len(text)-1], 64)
		if err == nil {
			n.IsComplex = true
			n.Complex128 = complex(0, f)
			n.simplifyComplex()
			return n, nil
		}
	}
	// Do integer test first so we get 0x123 etc.
	u, err := strconv.ParseUint(text, 0, 64) // will fail for -0; fixed below.
	if err == nil {
		n.IsUint = true
		n.Uint64 = u
	}
	i, err := strconv.ParseInt(text, 0, 64)
	if err == nil {
		n.IsInt = true
		n.Int64 = i
		if i == 0 {
			n.IsUint = true // in case of -0.
			n.Uint64 = u
		}
	}
	// If an integer extraction succeeded, promote the float.
	if n.IsInt {
		n.IsFloat = true
		n.Float64 = float64(n.Int64)
	} else if n.IsUint {
		n.IsFloat = true
		n.Float64 = float64(n.Uint64)
	} else {
		f, err := strconv.ParseFloat(text, 64)
		if err == nil {
			n.IsFloat = true
			n.Float64 = f
			// If a floating-point extraction succeeded, extract the int if needed.
			if !n.IsInt && float64(int64(f)) == f {
				n.IsInt = true
				n.Int64 = int64(f)
			}
			if !n.IsUint && float64(uint64(f)) == f {
				n.IsUint = true
				n.Uint64 = uint64(f)
			}
		}
	}
	if !n.IsInt && !n.IsUint && !n.IsFloat {
		return nil, fmt.Errorf("illegal number syntax: %q", text)
	}
	return n, nil
}

// simplifyComplex pulls out any other types that are represented by the complex number.
// These all require that the imaginary part be zero.
func (n *NumberNode) simplifyComplex() {
	n.IsFloat = imag(n.Complex128) == 0
	if n.IsFloat {
		n.Float64 = real(n.Complex128)
		n.IsInt = float64(int64(n.Float64)) == n.Float64
		if n.IsInt {
			n.Int64 = int64(n.Float64)
		}
		n.IsUint = float64(uint64(n.Float64)) == n.Float64
		if n.IsUint {
			n.Uint64 = uint64(n.Float64)
		}
	}
}
func (t *NumberNode) Reduce() (ret Node, err error) {
	ret = t.Copy()
	return
}
func (n *NumberNode) String() string {
	return n.Text
}

func (n *NumberNode) tree() *Tree {
	return n.tr
}

func (n *NumberNode) Copy() Node {
	nn := new(NumberNode)
	*nn = *n // Easy, fast, correct.
	return nn
}

// DiffNode holds a diff operation.
type DiffNode struct {
	NodeType
	Pos
	tr        *Tree
	Lefthand  Node
	Righthand Node
}

func (t *Tree) newDiff(pos Pos, lefthand Node, righthand Node) *DiffNode {
	return &DiffNode{tr: t, NodeType: NodeDiff, Pos: pos, Lefthand: lefthand, Righthand: righthand}
}
func (t *DiffNode) Reduce() (ret Node, err error) {
	var list *ListNode
	list = t.tr.newList(t.Pos)
	tmp, err := t.Lefthand.Reduce()
	if err != nil {
		return
	}
	list.append(tmp)
	tmp, err = t.Righthand.Reduce()
	if err != nil {
		return
	}
	list.append(tmp)
	ret = list
	return
}
func (t *DiffNode) String() string {
	return fmt.Sprintf("%v - %v", t.Lefthand, t.Righthand)
}

func (t *DiffNode) tree() *Tree {
	return t.tr
}

func (t *DiffNode) Copy() Node {
	return &DiffNode{tr: t.tr, NodeType: NodeDiff, Pos: t.Pos, Lefthand: t.Lefthand.Copy(), Righthand: t.Righthand.Copy()}
}

// IntersectionNode holds a diff operation.
type IntersectionNode struct {
	NodeType
	Pos
	tr        *Tree
	Lefthand  Node
	Righthand Node
}

func (t *Tree) newIntersection(pos Pos, lefthand Node, righthand Node) *IntersectionNode {
	return &IntersectionNode{tr: t, NodeType: NodeIntersection, Pos: pos, Lefthand: lefthand, Righthand: righthand}
}

func (t *IntersectionNode) Reduce() (ret Node, err error) {
	var list *ListNode
	list = t.tr.newList(t.Pos)
	tmp, err := t.Lefthand.Reduce()
	if err != nil {
		return
	}
	list.append(tmp)
	tmp, err = t.Righthand.Reduce()
	if err != nil {
		return
	}
	list.append(tmp)
	ret = list
	return
}

func (t *IntersectionNode) String() string {
	return fmt.Sprintf("%v && %v", t.Lefthand, t.Righthand)
}

func (t *IntersectionNode) tree() *Tree {
	return t.tr
}

func (t *IntersectionNode) Copy() Node {
	return &IntersectionNode{tr: t.tr, NodeType: NodeIntersection, Pos: t.Pos, Lefthand: t.Lefthand.Copy(), Righthand: t.Righthand.Copy()}
}

// UnionNode holds a diff operation.
type UnionNode struct {
	NodeType
	Pos
	tr        *Tree
	Lefthand  Node
	Righthand Node
}

func (t *Tree) newUnion(pos Pos, lefthand Node, righthand Node) *UnionNode {
	return &UnionNode{tr: t, NodeType: NodeUnion, Pos: pos, Lefthand: lefthand, Righthand: righthand}
}

func (t *UnionNode) Reduce() (ret Node, err error) {
	var list *ListNode
	list = t.tr.newList(t.Pos)
	tmp, err := t.Lefthand.Reduce()
	if err != nil {
		return
	}
	list.append(tmp)
	tmp, err = t.Righthand.Reduce()
	if err != nil {
		return
	}
	list.append(tmp)
	ret = list
	return
}

func (t *UnionNode) String() string {
	return fmt.Sprintf("%v || %v", t.Lefthand, t.Righthand)
}

func (t *UnionNode) tree() *Tree {
	return t.tr
}

func (t *UnionNode) Copy() Node {
	return &UnionNode{tr: t.tr, NodeType: NodeUnion, Pos: t.Pos, Lefthand: t.Lefthand.Copy(), Righthand: t.Righthand.Copy()}
}

// ObjectNode holds a diff operation.
type ObjectNode struct {
	NodeType
	Pos
	tr             *Tree
	Ident          string
	Params         []Node
	LocationParams []Node
}

func (t *Tree) newObject(pos Pos, ident string, params []Node, location_params []Node) *ObjectNode {
	return &ObjectNode{tr: t, NodeType: NodeObject, Pos: pos, Ident: ident, Params: params, LocationParams: location_params}
}
func (t *ObjectNode) Reduce() (list Node, err error) {
	list = t.Copy()
	return
}
func (t *ObjectNode) String() string {
	return fmt.Sprintf("%v %v @ %v", t.Ident, t.Params, t.LocationParams)
}

func (t *ObjectNode) tree() *Tree {
	return t.tr
}

func (t *ObjectNode) Copy() Node {
	var params_copy []Node
	var loc_params_copy []Node
	for _, elem := range t.Params {
		params_copy = append(params_copy, elem.Copy())
	}
	for _, elem := range t.LocationParams {
		loc_params_copy = append(loc_params_copy, elem.Copy())
	}
	return &ObjectNode{tr: t.tr, NodeType: NodeObject, Pos: t.Pos, Ident: t.Ident, Params: params_copy, LocationParams: loc_params_copy}
}
