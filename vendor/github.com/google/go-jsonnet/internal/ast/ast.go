/*
Copyright 2026 Google Inc. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package ast provides internal helpers related to AST nodes and ancillary structures.
package ast

import (
	"github.com/google/go-jsonnet/ast"
)

type Precedence int

const (
	MinPrecedence   Precedence = 1  // Var, Self, Parens, literals
	ApplyPrecedence Precedence = 2  // ast.Function calls and indexing.
	UnaryPrecedence Precedence = 4  // Logical and bitwise negation, unary + -
	MaxPrecedence   Precedence = 16 // ast.Local, If, ast.Import, ast.Function, Error
)

var bopPrecedence = map[ast.BinaryOp]Precedence{
	ast.BopMult:            5,
	ast.BopDiv:             5,
	ast.BopPercent:         5,
	ast.BopPlus:            6,
	ast.BopMinus:           6,
	ast.BopShiftL:          7,
	ast.BopShiftR:          7,
	ast.BopGreater:         8,
	ast.BopGreaterEq:       8,
	ast.BopLess:            8,
	ast.BopLessEq:          8,
	ast.BopIn:              8,
	ast.BopManifestEqual:   9,
	ast.BopManifestUnequal: 9,
	ast.BopBitwiseAnd:      10,
	ast.BopBitwiseXor:      11,
	ast.BopBitwiseOr:       12,
	ast.BopAnd:             13,
	ast.BopOr:              14,
}

func BinaryOpPrecedence(bop ast.BinaryOp) Precedence {
	return bopPrecedence[bop]
}

// ExprPrecedence gives the precedence level of an operation, so that it can be
// correctly parsed and unparsed/formatted.
func ExprPrecedence(node ast.Node) Precedence {
	switch node := node.(type) {
	case *ast.Apply,
		*ast.ApplyBrace,
		*ast.Index,
		*ast.SuperIndex,
		*ast.Slice:
		return ApplyPrecedence
	case *ast.Unary:
		return UnaryPrecedence
	case *ast.Binary:
		return bopPrecedence[node.Op]
	case *ast.InSuper:
		return bopPrecedence[ast.BopIn]
	case *ast.Local,
		*ast.Assert,
		*ast.Error,
		*ast.Import,
		*ast.ImportStr,
		*ast.ImportBin,
		*ast.Conditional:
		return MaxPrecedence
	}
	return MinPrecedence
}

func TighterPrecedence(p Precedence) Precedence {
	if p == MinPrecedence {
		return p
	} else {
		return (p - 1)
	}
}
