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

package gqlang

import "golang.org/x/xerrors"

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

func (p *parser) definition() (*Definition, []error) {
	if len(p.tokens) == 0 {
		return nil, nil
	}
	op, errs := p.operation()
	return op.asDefinition(), errs
}

func (p *parser) operation() (*Operation, []error) {
	op := &Operation{
		Start: p.tokens[0].start,
	}
	var errs []error
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
				err: xerrors.New("operation: expected name, variable definitions, or selection set, got EOF"),
			}}
		}
		if p.tokens[0].kind == name {
			var err error
			op.Name, err = p.name()
			if err != nil {
				return nil, []error{xerrors.Errorf("operation: %w", err)}
			}
			if len(p.tokens) == 0 {
				return nil, []error{&posError{
					pos: p.eofPos,
					err: xerrors.New("operation: expected variable definitions or selection set, got EOF"),
				}}
			}
		}
		if p.tokens[0].kind == lparen {
			var varDefErrs []error
			op.VariableDefinitions, varDefErrs = p.variableDefinitions()
			for _, err := range varDefErrs {
				if op.Name != nil && op.Name.Value != "" {
					errs = append(errs, xerrors.Errorf("operation %s: %w", op.Name.Value, err))
				} else {
					errs = append(errs, xerrors.Errorf("operation: %w", err))
				}
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
	var selSetErrs []error
	op.SelectionSet, selSetErrs = p.selectionSet()
	for _, err := range selSetErrs {
		if op.Name != nil && op.Name.Value != "" {
			errs = append(errs, xerrors.Errorf("operation %s: %w", op.Name.Value, err))
		} else {
			errs = append(errs, xerrors.Errorf("operation: %w", err))
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
		f.Arguments, argsErrs = p.arguments(false)
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

func (p *parser) arguments(isConst bool) (*Arguments, []error) {
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
		arg, argErrs := p.argument(isConst)
		for _, err := range argErrs {
			errs = append(errs, xerrors.Errorf("argument #%d: %w", len(args.Args)+1, err))
		}
		if arg != nil {
			args.Args = append(args.Args, arg)
		}
	}
	return args, errs
}

func (p *parser) argument(isConst bool) (*Argument, []error) {
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
	value, valueErrs := p.value(isConst)
	return &Argument{
		Name:  argName,
		Colon: colon.start,
		Value: value,
	}, valueErrs
}

func (p *parser) value(isConst bool) (*InputValue, []error) {
	if len(p.tokens) == 0 {
		return nil, []error{&posError{
			pos: p.eofPos,
			err: xerrors.New("value: expected scalar, got EOF"),
		}}
	}
	switch tok := p.tokens[0]; tok.kind {
	case dollar:
		v, err := p.variable()
		if err != nil {
			return nil, []error{xerrors.Errorf("value: %w", err)}
		}
		val := &InputValue{VariableRef: v}
		if isConst {
			return val, []error{&posError{
				pos: v.Dollar,
				err: xerrors.New("value: found variable in constant context"),
			}}
		}
		return val, nil
	case intValue:
		p.next()
		return &InputValue{Scalar: &ScalarValue{
			Start: tok.start,
			Type:  IntScalar,
			Raw:   tok.source,
		}}, nil
	case floatValue:
		p.next()
		return &InputValue{Scalar: &ScalarValue{
			Start: tok.start,
			Type:  FloatScalar,
			Raw:   tok.source,
		}}, nil
	case stringValue:
		p.next()
		return &InputValue{Scalar: &ScalarValue{
			Start: tok.start,
			Type:  StringScalar,
			Raw:   tok.source,
		}}, nil
	case name:
		p.next()
		val := &InputValue{Scalar: &ScalarValue{
			Start: tok.start,
			Raw:   tok.source,
		}}
		switch tok.source {
		case "null":
			val.Scalar.Type = NullScalar
		case "false", "true":
			val.Scalar.Type = BooleanScalar
		default:
			val.Scalar.Type = EnumScalar
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

func (p *parser) variable() (*Variable, error) {
	if len(p.tokens) == 0 {
		return nil, &posError{
			pos: p.eofPos,
			err: xerrors.New("variable: expected '$', got EOF"),
		}
	}
	tok := p.tokens[0]
	if tok.kind != dollar {
		return nil, &posError{
			pos: tok.start,
			err: xerrors.Errorf("variable: expected '$', found %q", tok),
		}
	}
	p.next()
	varName, err := p.name()
	if err != nil {
		return nil, xerrors.Errorf("variable: %w", err)
	}
	return &Variable{
		Dollar: tok.start,
		Name:   varName,
	}, nil
}

func (p *parser) variableDefinitions() (*VariableDefinitions, []error) {
	if len(p.tokens) == 0 {
		return nil, []error{&posError{
			pos: p.eofPos,
			err: xerrors.New("variable definitions: expected '(', got EOF"),
		}}
	}
	if p.tokens[0].kind != lparen {
		return nil, []error{&posError{
			pos: p.tokens[0].start,
			err: xerrors.Errorf("variable definitions: expected '(', found %q", p.tokens[0]),
		}}
	}
	lparen := p.next()
	varDefs := &VariableDefinitions{
		LParen: lparen.start,
		RParen: -1,
	}
	var errs []error
	for {
		if len(p.tokens) == 0 {
			errs = append(errs, &posError{
				pos: p.eofPos,
				err: xerrors.New("variable definitions: expected '$' or ')', got EOF"),
			})
			break
		}
		if p.tokens[0].kind == rparen {
			rparen := p.next()
			varDefs.RParen = rparen.start
			if len(varDefs.Defs) == 0 {
				errs = append(errs, &posError{
					pos: rparen.start,
					err: xerrors.New("variable definitions: empty"),
				})
			}
			break
		}
		def, defErrs := p.variableDefinition()
		for _, err := range defErrs {
			errs = append(errs, xerrors.Errorf("variable definition #%d: %w", len(varDefs.Defs)+1, err))
		}
		if def != nil {
			varDefs.Defs = append(varDefs.Defs, def)
		}
	}
	return varDefs, errs
}

func (p *parser) variableDefinition() (*VariableDefinition, []error) {
	// Not prepending "variable definition:" to errors, since arguments() will
	// prepend "variable definition #X:".

	def := &VariableDefinition{
		Colon: -1,
	}
	var err error
	def.Var, err = p.variable()
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
	def.Colon = colon.start
	var errs []error
	def.Type, errs = p.typeRef()
	// TODO(soon): Default value.
	return def, errs
}

func (p *parser) typeRef() (*TypeRef, []error) {
	if len(p.tokens) == 0 {
		return nil, []error{&posError{
			pos: p.eofPos,
			err: xerrors.New("type: expected name or '[', got EOF"),
		}}
	}
	switch tok := p.tokens[0]; tok.kind {
	case name:
		n, err := p.name()
		if err != nil {
			return nil, []error{xerrors.Errorf("type: %w", err)}
		}
		if len(p.tokens) == 0 || p.tokens[0].kind != nonNull {
			return &TypeRef{Named: n}, nil
		}
		bang := p.next()
		return &TypeRef{NonNull: &NonNullType{
			Named: n,
			Pos:   bang.start,
		}}, nil
	case lbracket:
		p.next()
		list := &ListType{
			LBracket: tok.start,
			RBracket: -1,
		}
		var errs []error
		list.Type, errs = p.typeRef()
		for i := range errs {
			errs[i] = xerrors.Errorf("list type: %w", errs[i])
		}
		if len(p.tokens) == 0 {
			errs = append(errs, &posError{
				pos: p.eofPos,
				err: xerrors.New("list type: expected ']', got EOF"),
			})
			return &TypeRef{List: list}, errs
		}
		if p.tokens[0].kind != rbracket {
			errs = append(errs, &posError{
				pos: p.tokens[0].start,
				err: xerrors.Errorf("list type: expected ']', found %q", p.tokens[0]),
			})
			return &TypeRef{List: list}, errs
		}
		list.RBracket = p.next().start
		if len(p.tokens) == 0 || p.tokens[0].kind != nonNull {
			return &TypeRef{List: list}, nil
		}
		bang := p.next()
		return &TypeRef{NonNull: &NonNullType{
			List: list,
			Pos:  bang.start,
		}}, nil
	default:
		return nil, []error{&posError{
			pos: tok.start,
			err: xerrors.Errorf("type: expected name or '[', found %q", tok),
		}}
	}
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
