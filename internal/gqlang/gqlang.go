// Copyright 2019 Ross Light
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// SPDX-License-Identifier: Apache-2.0

// Package gqlang provides a parser for the GraphQL language.
package gqlang

import (
	"fmt"

	"golang.org/x/xerrors"
)

// Document is a parsed GraphQL source.
// https://graphql.github.io/graphql-spec/June2018/#sec-Language.Document
type Document struct {
	Definitions []Definition
}

// Definition is a top-level GraphQL construct like an operation, a fragment, or
// a type.
// https://graphql.github.io/graphql-spec/June2018/#sec-Language.Document
type Definition interface {
	StartPos() Pos
	EndPos() Pos
	IsExecutableDefinition() bool
}

// Operation is a query, a mutation, or a subscription.
// https://graphql.github.io/graphql-spec/June2018/#sec-Language.Operations
type Operation struct {
	Start        Pos
	Type         OperationType
	Name         *Name
	SelectionSet *SelectionSet
}

// StartPos returns op.Start.
func (op *Operation) StartPos() Pos {
	return op.Start
}

// EndPos returns op.SelectionSet.RBrace.
func (op *Operation) EndPos() Pos {
	return op.SelectionSet.RBrace
}

// IsExecutableDefinition returns true.
func (op *Operation) IsExecutableDefinition() bool {
	return true
}

// OperationType is one of query, mutation, or subscription.
type OperationType int

// Types of operation.
const (
	Query OperationType = iota
	Mutation
	Subscription
)

// String returns the keyword that corresponds to the operation type.
func (typ OperationType) String() string {
	switch typ {
	case Query:
		return "query"
	case Mutation:
		return "mutation"
	case Subscription:
		return "subscription"
	default:
		return fmt.Sprintf("OperationType(%d)", int(typ))
	}
}

// SelectionSet is the set of information an operation requests.
// https://graphql.github.io/graphql-spec/June2018/#sec-Selection-Sets
type SelectionSet struct {
	LBrace Pos
	Sel    []*Selection
	RBrace Pos
}

// A Selection is either a field or a fragment.
// https://graphql.github.io/graphql-spec/June2018/#sec-Selection-Sets
type Selection struct {
	Field *Field
}

// A Field is a discrete piece of information available to request within a
// selection set.
// https://graphql.github.io/graphql-spec/June2018/#sec-Language.Fields
type Field struct {
	Alias        *Name
	Name         *Name
	Arguments    *Arguments
	SelectionSet *SelectionSet
}

// Arguments is a set of named arguments on a field.
// https://graphql.github.io/graphql-spec/June2018/#sec-Language.Arguments
type Arguments struct {
	LParen Pos
	Args   []*Argument
	RParen Pos
}

// Argument is a single element in Arguments.
// https://graphql.github.io/graphql-spec/June2018/#sec-Language.Arguments
type Argument struct {
	Name  *Name
	Colon Pos
	Value *ScalarValue
}

// ScalarValue is a primitive literal like a string or integer.
type ScalarValue struct {
	Start Pos
	Type  ScalarType
	Raw   string
}

// String returns sval.Raw.
func (sval *ScalarValue) String() string {
	return sval.Raw
}

// AsBool reads the scalar's boolean value.
func (sval *ScalarValue) AsBool() bool {
	if sval.Type != BooleanScalar {
		return false
	}
	return sval.Raw == "true"
}

// ScalarType indicates the type of a ScalarValue.
type ScalarType int

// Scalar types.
const (
	NullScalar ScalarType = iota
	BooleanScalar
	EnumScalar
	IntScalar
	FloatScalar
	StringScalar
)

// A Name is an identifier.
// https://graphql.github.io/graphql-spec/June2018/#sec-Names
type Name struct {
	Value string
	Start Pos
}

// String returns the name.
func (n *Name) String() string {
	if n == nil {
		return ""
	}
	return n.Value
}

type parser struct {
	tokens []token
	eofPos Pos
}

// Parse parses a GraphQL document into an abstract syntax tree.
func Parse(input string) (*Document, []error) {
	p := &parser{
		tokens: lex(input),
		eofPos: Pos(len(input)),
	}
	var errs []error
	for _, tok := range p.tokens {
		if tok.kind == unknown {
			errs = append(errs, &posError{
				input: input,
				pos:   tok.start,
				err:   xerrors.Errorf("unrecognized symbol %q", tok.source),
			})
		}
		// TODO(soon): Check for improperly terminated strings.
	}
	if len(errs) > 0 {
		return nil, errs
	}
	doc := new(Document)
	for len(p.tokens) > 0 {
		defn, defnErrs := p.definition()
		for _, err := range defnErrs {
			fillErrorInput(err, input)
		}
		errs = append(errs, defnErrs...)
		if defn == nil {
			break
		}
		doc.Definitions = append(doc.Definitions, defn)
	}
	return doc, errs
}

func (p *parser) next() token {
	tok := p.tokens[0]
	p.tokens = p.tokens[1:]
	return tok
}

func (p *parser) definition() (Definition, []error) {
	if len(p.tokens) == 0 {
		return nil, nil
	}
	return p.operation()
}

func (p *parser) operation() (*Operation, []error) {
	op := &Operation{
		Start: p.tokens[0].start,
	}
	switch first := p.tokens[0]; first.kind {
	case name:
		switch first.source {
		case "query":
			op.Type = Query
		case "mutation":
			op.Type = Mutation
		case "subscription":
			op.Type = Subscription
		default:
			return nil, []error{&posError{
				pos: first.start,
				err: xerrors.Errorf("operation: expected query, mutation, subscription, or '{', found %q", first),
			}}
		}
		p.next()
		if len(p.tokens) == 0 {
			return nil, []error{&posError{
				pos: p.eofPos,
				err: xerrors.New("operation: expected name or selection set, got EOF"),
			}}
		}
		if p.tokens[0].kind == name {
			var err error
			op.Name, err = p.name()
			if err != nil {
				return nil, []error{xerrors.Errorf("operation: %w", err)}
			}
		}
	case lbrace:
		// Shorthand syntax.
		op.Type = Query
	default:
		return nil, []error{&posError{
			pos: first.start,
			err: xerrors.Errorf("operation: expected query, mutation, subscription, or '{', found %q", first),
		}}
	}
	var errs []error
	op.SelectionSet, errs = p.selectionSet()
	for i := range errs {
		if op.Name != nil && op.Name.Value != "" {
			errs[i] = xerrors.Errorf("operation %s: %w", op.Name.Value)
		} else {
			errs[i] = xerrors.Errorf("operation: %w", errs[i])
		}
	}
	return op, errs
}

func (p *parser) selectionSet() (*SelectionSet, []error) {
	if len(p.tokens) == 0 {
		return nil, []error{xerrors.New("selection set: expected '{', got EOF")}
	}
	if p.tokens[0].kind != lbrace {
		return nil, []error{&posError{
			pos: p.tokens[0].start,
			err: xerrors.Errorf("selection set: expected '{', got %q", p.tokens[0]),
		}}
	}
	lbrace := p.next()
	set := &SelectionSet{
		LBrace: lbrace.start,
		RBrace: -1,
	}
	var errs []error
	for {
		if len(p.tokens) == 0 {
			errs = append(errs, &posError{
				pos: p.eofPos,
				err: xerrors.New("selection set: expected field or '}', got EOF"),
			})
			break
		}
		if p.tokens[0].kind == rbrace {
			rbrace := p.next()
			set.RBrace = rbrace.start
			if len(set.Sel) == 0 {
				errs = append(errs, &posError{
					pos: rbrace.start,
					err: xerrors.New("selection set: empty"),
				})
			}
			break
		}
		field, fieldErrs := p.field()
		for _, err := range fieldErrs {
			errs = append(errs, xerrors.Errorf("selection set: %w", err))
		}
		if field != nil {
			set.Sel = append(set.Sel, &Selection{
				Field: field,
			})
		}
	}
	return set, errs
}

func (p *parser) field() (*Field, []error) {
	f := new(Field)
	var err error
	f.Name, err = p.name()
	if err != nil {
		return nil, []error{xerrors.Errorf("field: %w", err)}
	}
	if len(p.tokens) == 0 {
		return f, nil
	}
	var errs []error
	if p.tokens[0].kind == lparen {
		var argsErrs []error
		f.Arguments, argsErrs = p.arguments()
		for _, err := range argsErrs {
			errs = append(errs, xerrors.Errorf("field %s: %w", f.Name.Value, err))
		}
		if len(p.tokens) == 0 {
			return f, nil
		}
	}
	if p.tokens[0].kind == lbrace {
		var selErrs []error
		f.SelectionSet, selErrs = p.selectionSet()
		for _, err := range selErrs {
			errs = append(errs, xerrors.Errorf("field %s: %w", f.Name.Value, err))
		}
	}
	return f, errs
}

func (p *parser) arguments() (*Arguments, []error) {
	if len(p.tokens) == 0 {
		return nil, []error{&posError{
			pos: p.eofPos,
			err: xerrors.New("arguments: expected '(', got EOF"),
		}}
	}
	if p.tokens[0].kind != lparen {
		return nil, []error{&posError{
			pos: p.tokens[0].start,
			err: xerrors.Errorf("arguments: expected '(', found %q", p.tokens[0]),
		}}
	}
	lparen := p.next()
	args := &Arguments{
		LParen: lparen.start,
		RParen: -1,
	}
	var errs []error
	for {
		if len(p.tokens) == 0 {
			errs = append(errs, &posError{
				pos: p.eofPos,
				err: xerrors.New("arguments: expected name or ')', got EOF"),
			})
			break
		}
		if p.tokens[0].kind == rparen {
			rparen := p.next()
			args.RParen = rparen.start
			if len(args.Args) == 0 {
				errs = append(errs, &posError{
					pos: rparen.start,
					err: xerrors.New("arguments: empty"),
				})
			}
			break
		}
		arg, argErrs := p.argument()
		for _, err := range argErrs {
			errs = append(errs, xerrors.Errorf("argument #%d: %w", len(args.Args)+1, err))
		}
		if arg != nil {
			args.Args = append(args.Args, arg)
		}
	}
	return args, errs
}

func (p *parser) argument() (*Argument, []error) {
	// Not prepending "argument:" to errors, since arguments() will prepend
	// "argument #X:".

	argName, err := p.name()
	if err != nil {
		return nil, []error{err}
	}
	if len(p.tokens) == 0 {
		return nil, []error{&posError{
			pos: p.eofPos,
			err: xerrors.New("expected ':', got EOF"),
		}}
	}
	if p.tokens[0].kind != colon {
		return nil, []error{&posError{
			pos: p.tokens[0].start,
			err: xerrors.Errorf("expected ':', got %q", p.tokens[0]),
		}}
	}
	colon := p.next()
	value, valueErrs := p.value()
	return &Argument{
		Name:  argName,
		Colon: colon.start,
		Value: value,
	}, valueErrs
}

func (p *parser) value() (*ScalarValue, []error) {
	if len(p.tokens) == 0 {
		return nil, []error{&posError{
			pos: p.eofPos,
			err: xerrors.New("value: expected scalar, got EOF"),
		}}
	}
	switch tok := p.tokens[0]; tok.kind {
	case intValue:
		p.next()
		return &ScalarValue{
			Start: tok.start,
			Type:  IntScalar,
			Raw:   tok.source,
		}, nil
	case floatValue:
		p.next()
		return &ScalarValue{
			Start: tok.start,
			Type:  FloatScalar,
			Raw:   tok.source,
		}, nil
	case stringValue:
		p.next()
		return &ScalarValue{
			Start: tok.start,
			Type:  StringScalar,
			Raw:   tok.source,
		}, nil
	case name:
		p.next()
		val := &ScalarValue{
			Start: tok.start,
			Raw:   tok.source,
		}
		switch tok.source {
		case "null":
			val.Type = NullScalar
		case "false", "true":
			val.Type = BooleanScalar
		default:
			val.Type = EnumScalar
		}
		return val, nil
	default:
		return nil, []error{&posError{
			pos: tok.start,
			err: xerrors.New("value: expected scalar, got %q"),
		}}
	}
}

func (p *parser) name() (*Name, error) {
	if len(p.tokens) == 0 {
		return nil, &posError{
			pos: p.eofPos,
			err: xerrors.New("expected name, got EOF"),
		}
	}
	tok := p.tokens[0]
	if tok.kind != name {
		return nil, &posError{
			pos: tok.start,
			err: xerrors.Errorf("expected name, found %q", tok),
		}
	}
	p.next()
	return &Name{
		Start: tok.start,
		Value: tok.source,
	}, nil
}

type posError struct {
	input string
	pos   Pos
	err   error
}

func (e *posError) Error() string {
	return e.err.Error()
}

func (e *posError) Unwrap() error {
	return e.err
}

// ErrorPos attempts to extract an error's Pos.
func ErrorPos(e error) (pos Pos, ok bool) {
	var pe *posError
	if !xerrors.As(e, &pe) {
		return 0, false
	}
	return pe.pos, true
}

// ErrorPosition attempts to extract an error's Position.
func ErrorPosition(e error) (p Position, ok bool) {
	var pe *posError
	if !xerrors.As(e, &pe) {
		return Position{}, false
	}
	return pe.pos.ToPosition(pe.input), true
}

func fillErrorInput(e error, input string) {
	var pe *posError
	if !xerrors.As(e, &pe) {
		return
	}
	pe.input = input
}
