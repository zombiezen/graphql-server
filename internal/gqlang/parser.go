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
	if (p.tokens[0].kind == name && (p.tokens[0].source == "query" || p.tokens[0].source == "mutation" || p.tokens[0].source == "subscription")) || p.tokens[0].kind == lbrace {
		// Operations do not permit a description before them.
		op, errs := p.operation()
		return op.asDefinition(), errs
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
		def, errs := p.scalarTypeDefinition()
		return def.asTypeDefinition().asDefinition(), errs
	case "type":
		def, errs := p.objectTypeDefinition()
		return def.asTypeDefinition().asDefinition(), errs
	case "input":
		def, errs := p.inputObjectTypeDefinition()
		return def.asTypeDefinition().asDefinition(), errs
	default:
		return nil, []error{&posError{
			pos: keywordTok.start,
			err: xerrors.Errorf("definition: expected keyword, found %q", keywordTok),
		}}
	}
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
	set := new(SelectionSet)
	var errs []error
	set.LBrace, set.RBrace, errs = p.group(lbrace, rbrace, "selection", func() []error {
		field, errs := p.field()
		if field != nil {
			set.Sel = append(set.Sel, &Selection{
				Field: field,
			})
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
		f.Arguments, argsErrs = p.arguments(false)
		for _, err := range argsErrs {
			errs = append(errs, xerrors.Errorf("field %s: %w", f.Name.Value, err))
		}
		if len(p.tokens) == 0 {
			return f, errs
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
	args := new(Arguments)
	var errs []error
	args.LParen, args.RParen, errs = p.group(lparen, rparen, "argument", func() []error {
		arg, errs := p.argument(isConst)
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
	varDefs := new(VariableDefinitions)
	var errs []error
	varDefs.LParen, varDefs.RParen, errs = p.group(lparen, rparen, "variable definition", func() []error {
		def, errs := p.variableDefinition()
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

func (p *parser) variableDefinition() (*VariableDefinition, []error) {
	// Not prepending "variable definition:" to errors, since
	// variableDefinitions() will prepend "variable definition #X:".

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
	def.Type, errs = p.typeRef()
	if len(errs) > 0 {
		return def, errs
	}
	def.Default, errs = p.optionalDefaultValue()
	return def, errs
}

func (p *parser) optionalDefaultValue() (*DefaultValue, []error) {
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
	defaultValue.Value, errs = p.value(true)
	for i := range errs {
		errs[i] = xerrors.Errorf("default value: %w", errs[i])
	}
	return defaultValue, errs
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

func (p *parser) scalarTypeDefinition() (*ScalarTypeDefinition, []error) {
	def := new(ScalarTypeDefinition)
	def.Description = p.optionalDescription()
	if len(p.tokens) == 0 {
		return nil, []error{&posError{
			pos: p.eofPos,
			err: xerrors.New("scalar type definition: expected 'scalar', got EOF"),
		}}
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

func (p *parser) objectTypeDefinition() (*ObjectTypeDefinition, []error) {
	def := new(ObjectTypeDefinition)
	def.Description = p.optionalDescription()
	if len(p.tokens) == 0 {
		return nil, []error{&posError{
			pos: p.eofPos,
			err: xerrors.New("object type definition: expected 'type', got EOF"),
		}}
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
	def.Fields, errs = p.fieldsDefinition()
	for i := range errs {
		errs[i] = xerrors.Errorf("object type definition %s: %w", def.Name.Value, errs[i])
	}
	return def, errs
}

func (p *parser) fieldsDefinition() (*FieldsDefinition, []error) {
	fields := new(FieldsDefinition)
	var errs []error
	fields.LBrace, fields.RBrace, errs = p.group(lbrace, rbrace, "field definition", func() []error {
		def, errs := p.fieldDefinition()
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

func (p *parser) fieldDefinition() (*FieldDefinition, []error) {
	// Not prepending "field definition:" to errors, since
	// fieldDefinitions() will prepend "field definition #X:".

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
		field.Args, argsErrs = p.argumentsDefinition()
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
	field.Type, typeErrs = p.typeRef()
	errs = append(errs, typeErrs...)
	return field, errs
}

func (p *parser) argumentsDefinition() (*ArgumentsDefinition, []error) {
	args := new(ArgumentsDefinition)
	var errs []error
	args.LParen, args.RParen, errs = p.group(lparen, rparen, "input value definition", func() []error {
		def, errs := p.inputValueDefinition()
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

func (p *parser) inputObjectTypeDefinition() (*InputObjectTypeDefinition, []error) {
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
	def.Fields, errs = p.inputFieldsDefinition()
	for i := range errs {
		errs[i] = xerrors.Errorf("input object type definition %s: %w", def.Name.Value, errs[i])
	}
	return def, errs
}

func (p *parser) inputFieldsDefinition() (*InputFieldsDefinition, []error) {
	fields := new(InputFieldsDefinition)
	var errs []error
	fields.LBrace, fields.RBrace, errs = p.group(lbrace, rbrace, "input value definition", func() []error {
		def, errs := p.inputValueDefinition()
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

func (p *parser) inputValueDefinition() (*InputValueDefinition, []error) {
	// Not prepending "input value definition:" to errors, since
	// argumentsDefinition() and inputFieldDefinitions() will prepend
	// "input value definition #X:".

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
	field.Type, errs = p.typeRef()
	if len(errs) > 0 {
		return field, errs
	}
	field.Default, errs = p.optionalDefaultValue()
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
