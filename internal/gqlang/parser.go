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

const (
	maxParseDepth = 50
	maxSize       = 16 << 10 // 16 KiB
)

var errTooDeep = xerrors.New("syntax tree too deep")

type parser struct {
	tokens []token
	eofPos Pos
}

// Parse parses a GraphQL document into an abstract syntax tree.
func Parse(input string) (*Document, []error) {
	if len(input) > maxSize {
		return nil, []error{xerrors.New("parse: document too large")}
	}
	p := &parser{
		tokens: lex(input),
		eofPos: Pos(len(input)),
	}
	var errs []error
	for _, tok := range p.tokens {
		switch tok.kind {
		case unknown:
			errs = append(errs, &posError{
				input: input,
				pos:   tok.start,
				err:   xerrors.Errorf("parse: unrecognized symbol %q", tok.source),
			})
		case stringValue:
			for _, err := range validateStringToken(input, tok) {
				errs = append(errs, xerrors.Errorf("parse: %w", err))
			}
		}
	}
	if len(errs) > 0 {
		return nil, errs
	}
	doc := new(Document)
	for len(p.tokens) > 0 {
		defn, defnErrs := p.definition(0)
		for _, err := range defnErrs {
			fillErrorInput(err, input)
			errs = append(errs, xerrors.Errorf("parse: %w", err))
		}
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

func (p *parser) definition(depth int) (*Definition, []error) {
	if len(p.tokens) == 0 {
		return nil, nil
	}
	if depth > maxParseDepth {
		return nil, []error{errTooDeep}
	}
	if (p.tokens[0].kind == name && (p.tokens[0].source == "query" || p.tokens[0].source == "mutation" || p.tokens[0].source == "subscription")) || p.tokens[0].kind == lbrace {
		// Operations do not permit a description before them.
		op, errs := p.operation(depth + 1)
		return op.asDefinition(), errs
	}
	if p.tokens[0].kind == name && p.tokens[0].source == "fragment" {
		// Fragments do not permit a description before them.
		frag, errs := p.fragmentDefinition(depth + 1)
		return frag.asDefinition(), errs
	}
	var keywordTok token
	switch p.tokens[0].kind {
	case name:
		keywordTok = p.tokens[0]
	case stringValue:
		if len(p.tokens) == 1 {
			return nil, []error{&posError{
				pos: p.eofPos,
				err: xerrors.New("type definition: expected keyword, got EOF"),
			}}
		}
		if p.tokens[1].kind != name {
			return nil, []error{&posError{
				pos: p.tokens[1].start,
				err: xerrors.Errorf("type definition: expected keyword, found %q", p.tokens[1]),
			}}
		}
		keywordTok = p.tokens[1]
	default:
		return nil, []error{&posError{
			pos: p.tokens[0].start,
			err: xerrors.Errorf("type definition: expected string or keyword, found %q", p.tokens[0]),
		}}
	}
	switch keywordTok.source {
	case "scalar":
		def, errs := p.scalarTypeDefinition(depth + 1)
		return def.asTypeDefinition().asDefinition(), errs
	case "type":
		def, errs := p.objectTypeDefinition(depth + 1)
		return def.asTypeDefinition().asDefinition(), errs
	case "enum":
		def, errs := p.enumTypeDefinition(depth + 1)
		return def.asTypeDefinition().asDefinition(), errs
	case "input":
		def, errs := p.inputObjectTypeDefinition(depth + 1)
		return def.asTypeDefinition().asDefinition(), errs
	default:
		return nil, []error{&posError{
			pos: keywordTok.start,
			err: xerrors.Errorf("definition: expected keyword, found %q", keywordTok),
		}}
	}
}

func (p *parser) operation(depth int) (*Operation, []error) {
	if depth > maxParseDepth {
		return nil, []error{errTooDeep}
	}
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
			op.VariableDefinitions, varDefErrs = p.variableDefinitions(depth + 1)
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
	op.SelectionSet, selSetErrs = p.selectionSet(depth + 1)
	for _, err := range selSetErrs {
		if op.Name != nil && op.Name.Value != "" {
			errs = append(errs, xerrors.Errorf("operation %s: %w", op.Name.Value, err))
		} else {
			errs = append(errs, xerrors.Errorf("operation: %w", err))
		}
	}
	return op, errs
}

func (p *parser) selectionSet(depth int) (*SelectionSet, []error) {
	if depth > maxParseDepth {
		return nil, []error{errTooDeep}
	}
	set := new(SelectionSet)
	var errs []error
	set.LBrace, set.RBrace, errs = p.group(lbrace, rbrace, "selection", func() []error {
		if len(p.tokens) > 0 && p.tokens[0].kind == ellipsis {
			sel, errs := p.fragment(depth + 1)
			if sel != nil {
				set.Sel = append(set.Sel, sel)
			}
			return errs
		}
		field, errs := p.field(depth + 1)
		if field != nil {
			set.Sel = append(set.Sel, field.asSelection())
		}
		return errs
	})
	for i := range errs {
		errs[i] = xerrors.Errorf("selection set: %w", errs[i])
	}
	if set.LBrace == -1 {
		return nil, errs
	}
	if set.RBrace >= 0 && len(set.Sel) == 0 {
		errs = append(errs, &posError{
			pos: set.RBrace,
			err: xerrors.New("selection set: empty"),
		})
	}
	return set, errs
}

func (p *parser) field(depth int) (*Field, []error) {
	if depth > maxParseDepth {
		return nil, []error{errTooDeep}
	}
	f := new(Field)
	var err error
	f.Name, err = p.name()
	if err != nil {
		return nil, []error{xerrors.Errorf("field: %w", err)}
	}
	if len(p.tokens) == 0 {
		return f, nil
	}
	if p.tokens[0].kind == colon {
		f.Alias = f.Name
		f.Name = nil
		p.next()
		f.Name, err = p.name()
		if err != nil {
			return f, []error{xerrors.Errorf("field: %w", err)}
		}
	}
	var errs []error
	if p.tokens[0].kind == lparen {
		var argsErrs []error
		f.Arguments, argsErrs = p.arguments(depth+1, false)
		for _, err := range argsErrs {
			errs = append(errs, xerrors.Errorf("field %s: %w", f.Name.Value, err))
		}
		if len(p.tokens) == 0 {
			return f, errs
		}
	}
	if p.tokens[0].kind == lbrace {
		var selErrs []error
		f.SelectionSet, selErrs = p.selectionSet(depth + 1)
		for _, err := range selErrs {
			errs = append(errs, xerrors.Errorf("field %s: %w", f.Name.Value, err))
		}
	}
	return f, errs
}

func (p *parser) arguments(depth int, isConst bool) (*Arguments, []error) {
	if depth > maxParseDepth {
		return nil, []error{errTooDeep}
	}
	args := new(Arguments)
	var errs []error
	args.LParen, args.RParen, errs = p.group(lparen, rparen, "argument", func() []error {
		arg, errs := p.argument(depth+1, isConst)
		if arg != nil {
			args.Args = append(args.Args, arg)
		}
		return errs
	})
	for i := range errs {
		errs[i] = xerrors.Errorf("arguments: %w", errs[i])
	}
	if args.LParen == -1 {
		return nil, errs
	}
	if args.RParen >= 0 && len(args.Args) == 0 {
		errs = append(errs, &posError{
			pos: args.RParen,
			err: xerrors.New("arguments: empty"),
		})
	}
	return args, errs
}

func (p *parser) argument(depth int, isConst bool) (*Argument, []error) {
	// Not prepending "argument:" to errors, since arguments() will prepend
	// "argument #X:".

	if depth > maxParseDepth {
		return nil, []error{errTooDeep}
	}
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
	value, valueErrs := p.value(depth+1, isConst)
	return &Argument{
		Name:  argName,
		Colon: colon.start,
		Value: value,
	}, valueErrs
}

func (p *parser) value(depth int, isConst bool) (*InputValue, []error) {
	if depth > maxParseDepth {
		return nil, []error{errTooDeep}
	}
	if len(p.tokens) == 0 {
		return nil, []error{&posError{
			pos: p.eofPos,
			err: xerrors.New("value: got EOF"),
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
		if tok.source == "null" {
			return &InputValue{
				Null: &Name{
					Start: tok.start,
					Value: tok.source,
				},
			}, nil
		}
		val := &InputValue{Scalar: &ScalarValue{
			Start: tok.start,
			Raw:   tok.source,
		}}
		switch tok.source {
		case "false", "true":
			val.Scalar.Type = BooleanScalar
		default:
			val.Scalar.Type = EnumScalar
		}
		return val, nil
	case lbracket:
		lval, errs := p.listValue(depth+1, isConst)
		return &InputValue{List: lval}, errs
	case lbrace:
		ioval, errs := p.objectValue(depth+1, isConst)
		return &InputValue{InputObject: ioval}, errs
	default:
		return nil, []error{&posError{
			pos: tok.start,
			err: xerrors.Errorf("value: got %q", tok),
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

func (p *parser) listValue(depth int, isConst bool) (*ListValue, []error) {
	if depth > maxParseDepth {
		return nil, []error{errTooDeep}
	}
	lval := new(ListValue)
	var errs []error
	lval.LBracket, lval.RBracket, errs = p.group(lbracket, rbracket, "list value", func() []error {
		elem, elemErrs := p.value(depth+1, isConst)
		lval.Values = append(lval.Values, elem)
		return elemErrs
	})
	if lval.LBracket == -1 {
		return nil, errs
	}
	return lval, errs
}

func (p *parser) objectValue(depth int, isConst bool) (*InputObjectValue, []error) {
	if depth > maxParseDepth {
		return nil, []error{errTooDeep}
	}
	ioval := new(InputObjectValue)
	var errs []error
	ioval.LBrace, ioval.RBrace, errs = p.group(lbrace, rbrace, "object field", func() []error {
		if len(p.tokens) == 0 {
			return []error{&posError{
				pos: p.eofPos,
				err: xerrors.New("expected name, got EOF"),
			}}
		}
		field := new(InputObjectField)
		var err error
		field.Name, err = p.name()
		if err != nil {
			return []error{err}
		}
		if len(p.tokens) == 0 {
			return []error{&posError{
				pos: p.eofPos,
				err: xerrors.New("expected ':', got EOF"),
			}}
		}
		tok := p.tokens[0]
		if tok.kind != colon {
			return []error{&posError{
				pos: p.eofPos,
				err: xerrors.Errorf("expected ':', found %q", tok),
			}}
		}
		field.Colon = tok.start
		p.next()
		var fieldErrs []error
		field.Value, fieldErrs = p.value(depth+1, isConst)
		ioval.Fields = append(ioval.Fields, field)
		return fieldErrs
	})
	for i, err := range errs {
		errs[i] = xerrors.Errorf("object value: %w", err)
	}
	if ioval.LBrace == -1 {
		return nil, errs
	}
	return ioval, errs
}

func (p *parser) variableDefinitions(depth int) (*VariableDefinitions, []error) {
	if depth > maxParseDepth {
		return nil, []error{errTooDeep}
	}
	varDefs := new(VariableDefinitions)
	var errs []error
	varDefs.LParen, varDefs.RParen, errs = p.group(lparen, rparen, "variable definition", func() []error {
		def, errs := p.variableDefinition(depth + 1)
		if def != nil {
			varDefs.Defs = append(varDefs.Defs, def)
		}
		return errs
	})
	for i := range errs {
		errs[i] = xerrors.Errorf("variable definitions: %w", errs[i])
	}
	if varDefs.LParen == -1 {
		return nil, errs
	}
	if varDefs.RParen >= 0 && len(varDefs.Defs) == 0 {
		errs = append(errs, &posError{
			pos: varDefs.RParen,
			err: xerrors.New("variable definitions: empty"),
		})
	}
	return varDefs, errs
}

func (p *parser) variableDefinition(depth int) (*VariableDefinition, []error) {
	// Not prepending "variable definition:" to errors, since
	// variableDefinitions() will prepend "variable definition #X:".

	if depth > maxParseDepth {
		return nil, []error{errTooDeep}
	}
	def := &VariableDefinition{
		Colon: -1,
	}
	var err error
	def.Var, err = p.variable()
	if err != nil {
		return nil, []error{err}
	}
	if len(p.tokens) == 0 {
		return def, []error{&posError{
			pos: p.eofPos,
			err: xerrors.New("expected ':', got EOF"),
		}}
	}
	if p.tokens[0].kind != colon {
		return def, []error{&posError{
			pos: p.tokens[0].start,
			err: xerrors.Errorf("expected ':', got %q", p.tokens[0]),
		}}
	}
	colon := p.next()
	def.Colon = colon.start
	var errs []error
	def.Type, errs = p.typeRef(depth + 1)
	if len(errs) > 0 {
		return def, errs
	}
	def.Default, errs = p.optionalDefaultValue(depth + 1)
	return def, errs
}

func (p *parser) optionalDefaultValue(depth int) (*DefaultValue, []error) {
	if depth > maxParseDepth {
		return nil, []error{errTooDeep}
	}
	if len(p.tokens) == 0 {
		return nil, nil
	}
	if p.tokens[0].kind != equals {
		return nil, nil
	}
	defaultValue := &DefaultValue{
		Eq: p.next().start,
	}
	var errs []error
	defaultValue.Value, errs = p.value(depth+1, true)
	for i := range errs {
		errs[i] = xerrors.Errorf("default value: %w", errs[i])
	}
	return defaultValue, errs
}

func (p *parser) typeRef(depth int) (*TypeRef, []error) {
	if len(p.tokens) == 0 {
		return nil, []error{&posError{
			pos: p.eofPos,
			err: xerrors.New("type: expected name or '[', got EOF"),
		}}
	}
	if depth > maxParseDepth {
		return nil, []error{errTooDeep}
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
		list.Type, errs = p.typeRef(depth + 1)
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

func (p *parser) directives(depth int, isConst bool) (Directives, []error) {
	if depth > maxParseDepth {
		return nil, []error{errTooDeep}
	}
	var results Directives
	var errs []error
	for len(p.tokens) > 0 && p.tokens[0].kind == atSign {
		d := &Directive{
			At: p.next().start,
		}
		var err error
		d.Name, err = p.name()
		if err != nil {
			errs = append(errs, xerrors.Errorf("directive: %w", err))
			return results, errs
		}
		results = append(results, d)
		if len(p.tokens) == 0 {
			break
		}
		if p.tokens[0].kind != lparen {
			continue
		}
		var argErrs []error
		d.Arguments, argErrs = p.arguments(depth+1, isConst)
		for _, err := range argErrs {
			errs = append(errs, xerrors.Errorf("@%s directive: %w", d.Name.Value, err))
		}
	}
	return results, errs
}

// fragment parses either a FragmentSpread or an InlineFragment.
// See https://graphql.github.io/graphql-spec/June2018/#Selection
func (p *parser) fragment(depth int) (*Selection, []error) {
	// Don't prepend "selection:", since this method is only called in the context
	// of selection set, which already prepends "selection:".

	if len(p.tokens) == 0 {
		return nil, []error{&posError{
			pos: p.eofPos,
			err: xerrors.New("expected '...', got EOF"),
		}}
	}
	if depth > maxParseDepth {
		return nil, []error{errTooDeep}
	}
	if p.tokens[0].kind != ellipsis {
		return nil, []error{&posError{
			pos: p.tokens[0].start,
			err: xerrors.Errorf("expected '...', found %q", p.tokens[0]),
		}}
	}
	if len(p.tokens) == 1 {
		p.next() // Advance past the ellipsis to continue parsing selection set.
		return nil, []error{&posError{
			pos: p.eofPos,
			err: xerrors.New("expected name, 'on', or '{', got EOF"),
		}}
	}
	switch p.tokens[1].kind {
	case name:
		if p.tokens[1].source == "on" {
			// Type conditions are only present on inline fragments.
			frag, errs := p.inlineFragment(depth + 1)
			return frag.asSelection(), errs
		}
		spread, errs := p.fragmentSpread(depth + 1)
		return spread.asSelection(), errs
	case lbrace:
		// Selection sets are only present on inline fragments.
		frag, errs := p.inlineFragment(depth + 1)
		return frag.asSelection(), errs
	default:
		p.next() // Advance past the ellipsis to continue parsing selection set.
		return nil, []error{&posError{
			pos: p.eofPos,
			err: xerrors.New("expected name, 'on', or '{', got EOF"),
		}}
	}
}

func (p *parser) fragmentSpread(depth int) (*FragmentSpread, []error) {
	if len(p.tokens) == 0 {
		return nil, []error{&posError{
			pos: p.eofPos,
			err: xerrors.New("fragment spread: expected '...', got EOF"),
		}}
	}
	if depth > maxParseDepth {
		return nil, []error{errTooDeep}
	}
	if p.tokens[0].kind != ellipsis {
		return nil, []error{&posError{
			pos: p.tokens[0].start,
			err: xerrors.Errorf("fragment spread: expected '...', found %q", p.tokens[0]),
		}}
	}
	spread := &FragmentSpread{
		Ellipsis: p.next().start,
	}
	var err error
	spread.Name, err = p.name()
	if err != nil {
		return nil, []error{xerrors.Errorf("fragment spread: %w", err)}
	}
	return spread, nil
}

func (p *parser) inlineFragment(depth int) (*InlineFragment, []error) {
	if len(p.tokens) == 0 {
		return nil, []error{&posError{
			pos: p.eofPos,
			err: xerrors.New("inline fragment: expected '...', got EOF"),
		}}
	}
	if depth > maxParseDepth {
		return nil, []error{errTooDeep}
	}
	if p.tokens[0].kind != ellipsis {
		return nil, []error{&posError{
			pos: p.tokens[0].start,
			err: xerrors.Errorf("inline fragment: expected '...', found %q", p.tokens[0]),
		}}
	}
	frag := &InlineFragment{
		Ellipsis: p.next().start,
	}
	if len(p.tokens) == 0 {
		return nil, []error{&posError{
			pos: p.eofPos,
			err: xerrors.New("inline fragment: expected 'on' or '{', got EOF"),
		}}
	}
	if p.tokens[0].kind == name && p.tokens[0].source == "on" {
		var err error
		frag.Type, err = p.typeCondition(depth + 1)
		if err != nil {
			return nil, []error{xerrors.Errorf("inline fragment: %w", err)}
		}
	} else if p.tokens[0].kind != lbrace {
		// Would be caught by parsing selection set, but give a better error message.
		return nil, []error{&posError{
			pos: p.tokens[0].start,
			err: xerrors.Errorf("inline fragment: expected 'on' or '{', found %q", p.tokens[0]),
		}}
	}
	var errs []error
	frag.SelectionSet, errs = p.selectionSet(depth + 1)
	for i, err := range errs {
		errs[i] = xerrors.Errorf("inline fragment: %w", err)
	}
	return frag, errs
}

func (p *parser) fragmentDefinition(depth int) (*FragmentDefinition, []error) {
	defn := new(FragmentDefinition)
	if len(p.tokens) == 0 {
		return nil, []error{&posError{
			pos: p.eofPos,
			err: xerrors.New("fragment definition: expected 'fragment', got EOF"),
		}}
	}
	if depth > maxParseDepth {
		return nil, []error{errTooDeep}
	}
	if p.tokens[0].kind != name || p.tokens[0].source != "fragment" {
		return nil, []error{&posError{
			pos: p.tokens[0].start,
			err: xerrors.Errorf("fragment definition: expected 'fragment', found %q", p.tokens[0]),
		}}
	}
	defn.Keyword = p.next().start
	var err error
	defn.Name, err = p.name()
	if err != nil {
		return nil, []error{xerrors.Errorf("fragment definition: %w", err)}
	}
	if defn.Name.Value == "on" {
		return nil, []error{xerrors.New("fragment definition: expected name, found 'on'")}
	}
	defn.Type, err = p.typeCondition(depth + 1)
	if err != nil {
		return nil, []error{xerrors.Errorf("fragment definition %s: %w", defn.Name.Value, err)}
	}
	var errs []error
	defn.SelectionSet, errs = p.selectionSet(depth + 1)
	for i, err := range errs {
		errs[i] = xerrors.Errorf("fragment definition %s: %w", defn.Name.Value, err)
	}
	return defn, errs
}

func (p *parser) typeCondition(depth int) (*TypeCondition, error) {
	cond := new(TypeCondition)
	if len(p.tokens) == 0 {
		return nil, &posError{
			pos: p.eofPos,
			err: xerrors.New("type condition: expected 'on', got EOF"),
		}
	}
	if depth > maxParseDepth {
		return nil, errTooDeep
	}
	if p.tokens[0].kind != name || p.tokens[0].source != "on" {
		return nil, &posError{
			pos: p.tokens[0].start,
			err: xerrors.Errorf("type condition: expected 'on', found %q", p.tokens[0]),
		}
	}
	cond.On = p.next().start
	var err error
	cond.Name, err = p.name()
	return cond, err
}

func (p *parser) optionalDescription() *Description {
	if len(p.tokens) == 0 {
		return nil
	}
	tok := p.tokens[0]
	if tok.kind != stringValue {
		return nil
	}
	p.next()
	return &Description{
		Start: tok.start,
		Raw:   tok.source,
	}
}

func (p *parser) scalarTypeDefinition(depth int) (*ScalarTypeDefinition, []error) {
	def := new(ScalarTypeDefinition)
	def.Description = p.optionalDescription()
	if len(p.tokens) == 0 {
		return nil, []error{&posError{
			pos: p.eofPos,
			err: xerrors.New("scalar type definition: expected 'scalar', got EOF"),
		}}
	}
	if depth > maxParseDepth {
		return nil, []error{errTooDeep}
	}
	if p.tokens[0].kind != name || p.tokens[0].source != "scalar" {
		return nil, []error{&posError{
			pos: p.tokens[0].start,
			err: xerrors.Errorf("scalar type definition: expected 'scalar', found %q", p.tokens[0]),
		}}
	}
	def.Keyword = p.next().start
	var err error
	def.Name, err = p.name()
	if err != nil {
		return def, []error{xerrors.Errorf("scalar type definition: %w", err)}
	}
	return def, nil
}

func (p *parser) objectTypeDefinition(depth int) (*ObjectTypeDefinition, []error) {
	def := new(ObjectTypeDefinition)
	def.Description = p.optionalDescription()
	if len(p.tokens) == 0 {
		return nil, []error{&posError{
			pos: p.eofPos,
			err: xerrors.New("object type definition: expected 'type', got EOF"),
		}}
	}
	if depth > maxParseDepth {
		return nil, []error{errTooDeep}
	}
	if p.tokens[0].kind != name || p.tokens[0].source != "type" {
		return nil, []error{&posError{
			pos: p.tokens[0].start,
			err: xerrors.Errorf("object type definition: expected 'type', found %q", p.tokens[0]),
		}}
	}
	def.Keyword = p.next().start
	var err error
	def.Name, err = p.name()
	if err != nil {
		return def, []error{xerrors.Errorf("object type definition: %w", err)}
	}
	var errs []error
	def.Fields, errs = p.fieldsDefinition(depth + 1)
	for i := range errs {
		errs[i] = xerrors.Errorf("object type definition %s: %w", def.Name.Value, errs[i])
	}
	return def, errs
}

func (p *parser) fieldsDefinition(depth int) (*FieldsDefinition, []error) {
	if depth > maxParseDepth {
		return nil, []error{errTooDeep}
	}
	fields := new(FieldsDefinition)
	var errs []error
	fields.LBrace, fields.RBrace, errs = p.group(lbrace, rbrace, "field definition", func() []error {
		def, errs := p.fieldDefinition(depth + 1)
		if def != nil {
			fields.Defs = append(fields.Defs, def)
		}
		return errs
	})
	for i := range errs {
		errs[i] = xerrors.Errorf("fields definition: %w", errs[i])
	}
	if fields.LBrace == -1 {
		return nil, errs
	}
	if fields.RBrace >= 0 && len(fields.Defs) == 0 {
		errs = append(errs, &posError{
			pos: fields.RBrace,
			err: xerrors.New("fields definition: empty"),
		})
	}
	return fields, errs
}

func (p *parser) fieldDefinition(depth int) (*FieldDefinition, []error) {
	// Not prepending "field definition:" to errors, since
	// fieldDefinitions() will prepend "field definition #X:".

	if depth > maxParseDepth {
		return nil, []error{errTooDeep}
	}
	field := new(FieldDefinition)
	field.Description = p.optionalDescription()
	var err error
	field.Name, err = p.name()
	if err != nil {
		return nil, []error{err}
	}
	if len(p.tokens) == 0 {
		return field, []error{&posError{
			pos: p.eofPos,
			err: xerrors.New("expected '(' or ':', got EOF"),
		}}
	}
	var errs []error
	if p.tokens[0].kind == lparen {
		var argsErrs []error
		field.Args, argsErrs = p.argumentsDefinition(depth + 1)
		errs = append(errs, argsErrs...)
		if len(p.tokens) == 0 {
			return field, append(errs, &posError{
				pos: p.eofPos,
				err: xerrors.New("expected ':', got EOF"),
			})
		}
	}
	if p.tokens[0].kind != colon {
		return field, append(errs, &posError{
			pos: p.tokens[0].start,
			err: xerrors.Errorf("expected ':', found %q", p.tokens[0]),
		})
	}
	field.Colon = p.next().start
	var typeErrs []error
	field.Type, typeErrs = p.typeRef(depth + 1)
	errs = append(errs, typeErrs...)
	var directiveErrs []error
	field.Directives, directiveErrs = p.directives(depth+1, true)
	errs = append(errs, directiveErrs...)
	return field, errs
}

func (p *parser) argumentsDefinition(depth int) (*ArgumentsDefinition, []error) {
	if depth > maxParseDepth {
		return nil, []error{errTooDeep}
	}
	args := new(ArgumentsDefinition)
	var errs []error
	args.LParen, args.RParen, errs = p.group(lparen, rparen, "input value definition", func() []error {
		def, errs := p.inputValueDefinition(depth + 1)
		if def != nil {
			args.Args = append(args.Args, def)
		}
		return errs
	})
	for i := range errs {
		errs[i] = xerrors.Errorf("arguments definition: %w", errs[i])
	}
	if args.LParen == -1 {
		return nil, errs
	}
	if args.RParen >= 0 && len(args.Args) == 0 {
		errs = append(errs, &posError{
			pos: args.RParen,
			err: xerrors.New("input fields definition: empty"),
		})
	}
	return args, errs
}

func (p *parser) enumTypeDefinition(depth int) (*EnumTypeDefinition, []error) {
	if depth > maxParseDepth {
		return nil, []error{errTooDeep}
	}
	defn := new(EnumTypeDefinition)
	defn.Description = p.optionalDescription()
	if len(p.tokens) == 0 {
		return nil, []error{&posError{
			pos: p.eofPos,
			err: xerrors.New("enum type definition: expected 'enum', got EOF"),
		}}
	}
	if p.tokens[0].kind != name || p.tokens[0].source != "enum" {
		return nil, []error{&posError{
			pos: p.tokens[0].start,
			err: xerrors.Errorf("enum type definition: expected 'enum', found %q", p.tokens[0]),
		}}
	}
	defn.Keyword = p.next().start
	var err error
	defn.Name, err = p.name()
	if err != nil {
		return defn, []error{xerrors.Errorf("enum type definition: %w", err)}
	}
	var errs []error
	defn.Values, errs = p.enumValuesDefinition(depth + 1)
	for i, err := range errs {
		errs[i] = xerrors.Errorf("enum type definition %s: %w", defn.Name.Value, err)
	}
	return defn, errs
}

func (p *parser) enumValuesDefinition(depth int) (*EnumValuesDefinition, []error) {
	if depth > maxParseDepth {
		return nil, []error{errTooDeep}
	}
	defn := new(EnumValuesDefinition)
	var errs []error
	defn.LBrace, defn.RBrace, errs = p.group(lbrace, rbrace, "enum value definition", func() []error {
		valueDefn := new(EnumValueDefinition)
		valueDefn.Description = p.optionalDescription()
		var err error
		valueDefn.Value, err = p.name()
		if err != nil {
			return []error{err}
		}
		defn.Values = append(defn.Values, valueDefn)

		var errs []error
		if v := valueDefn.Value.Value; v == "null" || v == "true" || v == "false" {
			errs = append(errs, xerrors.Errorf("expected enum name, found reserved name %q", v))
		}
		var directiveErrs []error
		valueDefn.Directives, directiveErrs = p.directives(depth+1, true)
		errs = append(errs, directiveErrs...)
		return errs
	})
	if defn.LBrace == -1 {
		return nil, errs
	}
	if defn.RBrace >= 0 && len(defn.Values) == 0 {
		errs = append(errs, &posError{
			pos: defn.RBrace,
			err: xerrors.New("enum values definition: empty"),
		})
	}
	return defn, errs
}

func (p *parser) inputObjectTypeDefinition(depth int) (*InputObjectTypeDefinition, []error) {
	if depth > maxParseDepth {
		return nil, []error{errTooDeep}
	}
	def := new(InputObjectTypeDefinition)
	def.Description = p.optionalDescription()
	if len(p.tokens) == 0 {
		return nil, []error{&posError{
			pos: p.eofPos,
			err: xerrors.New("input object type definition: expected 'input', got EOF"),
		}}
	}
	if p.tokens[0].kind != name || p.tokens[0].source != "input" {
		return nil, []error{&posError{
			pos: p.tokens[0].start,
			err: xerrors.Errorf("input object type definition: expected 'input', found %q", p.tokens[0]),
		}}
	}
	def.Keyword = p.next().start
	var err error
	def.Name, err = p.name()
	if err != nil {
		return def, []error{xerrors.Errorf("input object type definition: %w", err)}
	}
	var errs []error
	def.Fields, errs = p.inputFieldsDefinition(depth + 1)
	for i := range errs {
		errs[i] = xerrors.Errorf("input object type definition %s: %w", def.Name.Value, errs[i])
	}
	return def, errs
}

func (p *parser) inputFieldsDefinition(depth int) (*InputFieldsDefinition, []error) {
	if depth > maxParseDepth {
		return nil, []error{errTooDeep}
	}
	fields := new(InputFieldsDefinition)
	var errs []error
	fields.LBrace, fields.RBrace, errs = p.group(lbrace, rbrace, "input value definition", func() []error {
		def, errs := p.inputValueDefinition(depth + 1)
		if def != nil {
			fields.Defs = append(fields.Defs, def)
		}
		return errs
	})
	for i := range errs {
		errs[i] = xerrors.Errorf("input fields definition: %w", errs[i])
	}
	if fields.LBrace == -1 {
		return nil, errs
	}
	if fields.RBrace >= 0 && len(fields.Defs) == 0 {
		errs = append(errs, &posError{
			pos: fields.RBrace,
			err: xerrors.New("input fields definition: empty"),
		})
	}
	return fields, errs
}

func (p *parser) inputValueDefinition(depth int) (*InputValueDefinition, []error) {
	// Not prepending "input value definition:" to errors, since
	// argumentsDefinition() and inputFieldDefinitions() will prepend
	// "input value definition #X:".

	if depth > maxParseDepth {
		return nil, []error{errTooDeep}
	}
	field := new(InputValueDefinition)
	field.Description = p.optionalDescription()
	var err error
	field.Name, err = p.name()
	if err != nil {
		return nil, []error{err}
	}
	if len(p.tokens) == 0 {
		return field, []error{&posError{
			pos: p.eofPos,
			err: xerrors.New("expected ':', got EOF"),
		}}
	}
	if p.tokens[0].kind != colon {
		return field, []error{&posError{
			pos: p.tokens[0].start,
			err: xerrors.Errorf("expected ':', found %q", p.tokens[0]),
		}}
	}
	field.Colon = p.next().start
	var errs []error
	field.Type, errs = p.typeRef(depth + 1)
	if len(errs) > 0 {
		return field, errs
	}
	field.Default, errs = p.optionalDefaultValue(depth + 1)
	return field, errs
}

// group parses a list of the given rule started and ended by the given token kind.
func (p *parser) group(ldelim, rdelim tokenKind, ruleName string, rule func() []error) (start, end Pos, _ []error) {
	if len(p.tokens) == 0 {
		return -1, -1, []error{&posError{
			pos: p.eofPos,
			err: xerrors.Errorf("expected '%s', got EOF", punctuatorStrings[ldelim]),
		}}
	}
	if p.tokens[0].kind != ldelim {
		return -1, -1, []error{&posError{
			pos: p.tokens[0].start,
			err: xerrors.Errorf("expected '%s', found %q", punctuatorStrings[ldelim], p.tokens[0]),
		}}
	}
	start, end = p.next().start, -1
	var errs []error
	for i := 1; ; i++ {
		if len(p.tokens) == 0 {
			errs = append(errs, &posError{
				pos: p.eofPos,
				err: xerrors.Errorf("expected %s or '%s', got EOF", ruleName, punctuatorStrings[rdelim]),
			})
			break
		}
		if p.tokens[0].kind == rdelim {
			end = p.next().start
			break
		}
		nleft := len(p.tokens)
		ruleErrs := rule()
		for _, err := range ruleErrs {
			errs = append(errs, xerrors.Errorf("%s #%d: %w", ruleName, i, err))
		}
		if len(p.tokens) == nleft {
			end = p.skipTo(rdelim)
			break
		}
	}
	return start, end, errs
}

func (p *parser) skipTo(rdelim tokenKind) Pos {
	stk := []tokenKind{rdelim}
	for ; len(p.tokens) > 0 && len(stk) > 0; p.tokens = p.tokens[1:] {
		switch kind := p.tokens[0].kind; kind {
		case lparen:
			stk = append(stk, rparen)
		case lbrace:
			stk = append(stk, rbrace)
		case lbracket:
			stk = append(stk, rbracket)
		case rparen, rbrace, rbracket:
			for len(stk) > 0 && stk[len(stk)-1] != kind {
				stk = stk[:len(stk)-1]
			}
			if len(stk) == 1 {
				// Matches top of stack.
				return p.tokens[0].start
			}
		}
	}
	return -1
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
