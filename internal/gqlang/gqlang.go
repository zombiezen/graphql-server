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
	"strconv"
	"strings"
)

// Document is a parsed GraphQL source.
// https://graphql.github.io/graphql-spec/June2018/#sec-Language.Document
type Document struct {
	Definitions []*Definition
}

// Definition is a top-level GraphQL construct like an operation, a fragment, or
// a type. Only one of its fields will be set.
// https://graphql.github.io/graphql-spec/June2018/#sec-Language.Document
type Definition struct {
	Operation *Operation
	Type      *TypeDefinition
}

// Start returns the position of the definition's first token.
func (defn *Definition) Start() Pos {
	switch {
	case defn.Operation != nil:
		return defn.Operation.Start
	case defn.Type != nil:
		return defn.Type.Start()
	default:
		panic("unknown definition")
	}
}

// Operation is a query, a mutation, or a subscription.
// https://graphql.github.io/graphql-spec/June2018/#sec-Language.Operations
type Operation struct {
	Start               Pos
	Type                OperationType
	Name                *Name
	VariableDefinitions *VariableDefinitions
	SelectionSet        *SelectionSet
}

func (op *Operation) asDefinition() *Definition {
	if op == nil {
		return nil
	}
	return &Definition{Operation: op}
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
// https://graphql.github.io/graphql-spec/June2018/#SelectionSet
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

// Key returns the field's response key. Typically, this is the field's name,
// but if an alias is set then that will be used.
func (f *Field) Key() *Name {
	if f.Alias != nil {
		return f.Alias
	}
	return f.Name
}

// Start returns the byte offset of the beginning of the field.
func (f *Field) Start() Pos {
	if f.Alias != nil {
		return f.Alias.Start
	}
	return f.Name.Start
}

// End returns the byte offset after the end of the field.
func (f *Field) End() Pos {
	if f.SelectionSet != nil {
		return f.SelectionSet.RBrace + 1
	}
	if f.Arguments != nil {
		return f.Arguments.RParen + 1
	}
	return f.Name.End()
}

// Arguments is a set of named arguments on a field.
// https://graphql.github.io/graphql-spec/June2018/#sec-Language.Arguments
type Arguments struct {
	LParen Pos
	Args   []*Argument
	RParen Pos
}

// ByName returns the first argument with the given name or nil if not found.
func (args *Arguments) ByName(name string) *Argument {
	if args == nil {
		return nil
	}
	for _, arg := range args.Args {
		if arg.Name.Value == name {
			return arg
		}
	}
	return nil
}

// IdenticalTo reports whether args is structurally identical to args2,
// ignoring token positions and argument order. The result is undefined if
// either set of arguments contain multiple values for the same argument name.
func (args *Arguments) IdenticalTo(args2 *Arguments) bool {
	if args == nil {
		return args2 == nil
	}
	if args2 == nil {
		return false
	}
	return inputValuesIdentical(args.argMap(), args2.argMap())
}

func (args *Arguments) argMap() map[string]*InputValue {
	if args == nil {
		return nil
	}
	m := make(map[string]*InputValue)
	for _, arg := range args.Args {
		m[arg.Name.Value] = arg.Value
	}
	return m
}

func inputValuesIdentical(m1, m2 map[string]*InputValue) bool {
	if len(m1) != len(m2) {
		return false
	}
	for k, v1 := range m1 {
		if v2 := m2[k]; v2 == nil || !v1.IdenticalTo(v2) {
			return false
		}
	}
	for k := range m2 {
		if m1[k] == nil {
			// Argument present in args2 but not in args.
			return false
		}
	}
	return true
}

// Argument is a single element in Arguments.
// https://graphql.github.io/graphql-spec/June2018/#sec-Language.Arguments
type Argument struct {
	Name  *Name
	Colon Pos
	Value *InputValue
}

// An InputValue is a scalar or a variable reference.
// https://graphql.github.io/graphql-spec/June2018/#sec-Input-Values
type InputValue struct {
	Null        *Name
	Scalar      *ScalarValue
	VariableRef *Variable
	List        *ListValue
	InputObject *InputObjectValue
}

// Start returns the byte offset of the beginning of the expression.
func (ival *InputValue) Start() Pos {
	switch {
	case ival == nil:
		return -1
	case ival.Null != nil:
		return ival.Null.Start
	case ival.Scalar != nil:
		return ival.Scalar.Start
	case ival.VariableRef != nil:
		return ival.VariableRef.Dollar
	case ival.List != nil:
		return ival.List.LBracket
	case ival.InputObject != nil:
		return ival.InputObject.LBrace
	default:
		panic("unknown input value")
	}
}

// String formats the expression as GraphQL syntax.
func (ival *InputValue) String() string {
	switch {
	case ival == nil:
		return ""
	case ival.Null != nil:
		return ival.Null.String()
	case ival.Scalar != nil:
		return ival.Scalar.String()
	case ival.VariableRef != nil:
		return ival.VariableRef.String()
	case ival.List != nil:
		return ival.List.String()
	case ival.InputObject != nil:
		return ival.InputObject.String()
	default:
		panic("unknown input value")
	}
}

// IdenticalTo reports whether ival is structurally identical to ival2,
// ignoring token positions and input object field order. The result is
// undefined if either value contains an input object which contain multiple
// values for the same field name.
func (ival *InputValue) IdenticalTo(ival2 *InputValue) bool {
	if ival == nil {
		return ival2 == nil
	}
	if ival2 == nil {
		return false
	}
	switch {
	case ival.Null != nil && ival2.Null != nil:
		return true
	case ival.Scalar != nil && ival2.Scalar != nil:
		return ival.Scalar.IdenticalTo(ival2.Scalar)
	case ival.VariableRef != nil && ival2.VariableRef != nil:
		return ival.VariableRef.Name.Value == ival2.VariableRef.Name.Value
	case ival.List != nil && ival2.List != nil:
		return ival.List.IdenticalTo(ival2.List)
	case ival.InputObject != nil && ival2.InputObject != nil:
		return ival.InputObject.IdenticalTo(ival2.InputObject)
	default:
		return false
	}
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

// Value converts the raw scalar into a string.
func (sval *ScalarValue) Value() string {
	switch {
	case strings.HasPrefix(sval.Raw, `"""`):
		return parseBlockString(sval.Raw)
	case strings.HasPrefix(sval.Raw, `"`):
		return parseString(sval.Raw)
	default:
		return sval.Raw
	}
}

// IdenticalTo reports whether sval has the same type and value as sval2.
func (sval *ScalarValue) IdenticalTo(sval2 *ScalarValue) bool {
	if sval == nil {
		return sval2 == nil
	}
	if sval2 == nil {
		return false
	}
	return sval.Type == sval2.Type && sval.Value() == sval2.Value()
}

func parseString(raw string) string {
	raw = strings.TrimPrefix(raw, `"`)
	raw = strings.TrimSuffix(raw, `"`)
	sb := new(strings.Builder)
	sb.Grow(len(raw))
	for i := 0; i < len(raw); i++ {
		if raw[i] != '\\' {
			sb.WriteByte(raw[i])
			continue
		}
		i++ // skip past backslash
		switch raw[i] {
		case 'b':
			sb.WriteByte('\b')
		case 'f':
			sb.WriteByte('\f')
		case 'n':
			sb.WriteByte('\n')
		case 'r':
			sb.WriteByte('\r')
		case 't':
			sb.WriteByte('\t')
		case 'u':
			codePoint, err := strconv.ParseUint(raw[i+1:i+5], 16, 16)
			i += 4
			if err != nil {
				sb.WriteRune('\uFFFD') // Unicode replacement character
			}
			sb.WriteRune(rune(codePoint))
		default:
			sb.WriteByte(raw[i])
		}
	}
	return sb.String()
}

func parseBlockString(raw string) string {
	raw = strings.TrimPrefix(raw, `"""`)
	raw = strings.TrimSuffix(raw, `"""`)
	raw = strings.ReplaceAll(raw, `\"""`, `"""`)
	lines := splitLines(raw)
	if len(lines) == 0 {
		return ""
	}

	// Eliminate common indentation.
	commonIndent := -1
	for _, line := range lines[1:] {
		indent := countLeadingWhitespace(line)
		if indent < len(line) && (commonIndent == -1 || indent < commonIndent) {
			commonIndent = indent
		}
	}
	if commonIndent != -1 {
		for i, line := range lines {
			if i == 0 {
				continue
			}
			if commonIndent < len(line) {
				lines[i] = line[commonIndent:]
			} else {
				lines[i] = ""
			}
		}
	}

	// Strip leading and trailing blank lines.
	for len(lines) > 0 && countLeadingWhitespace(lines[0]) == len(lines[0]) {
		lines = lines[1:]
	}
	for len(lines) > 0 && countLeadingWhitespace(lines[len(lines)-1]) == len(lines[len(lines)-1]) {
		lines = lines[:len(lines)-1]
	}

	return strings.Join(lines, "\n")
}

func splitLines(s string) []string {
	lineStart := 0
	var lines []string
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '\r':
			lines = append(lines, s[lineStart:i])
			if i+1 < len(s) && s[i+1] == '\n' {
				// CRLF, advance.
				i++
			}
			lineStart = i + 1
		case '\n':
			lines = append(lines, s[lineStart:i])
			lineStart = i + 1
		}
	}
	if lineStart < len(s) {
		lines = append(lines, s[lineStart:])
	}
	return lines
}

func countLeadingWhitespace(s string) int {
	for i := 0; i < len(s); i++ {
		if s[i] != ' ' && s[i] != '\t' {
			return i
		}
	}
	return len(s)
}

// ScalarType indicates the type of a ScalarValue.
type ScalarType int

// Scalar types.
const (
	StringScalar ScalarType = iota
	BooleanScalar
	EnumScalar
	IntScalar
	FloatScalar
)

// A Variable is an input to a GraphQL operation.
// https://graphql.github.io/graphql-spec/June2018/#Variable
type Variable struct {
	Dollar Pos
	Name   *Name
}

// String returns the variable in the form "$foo".
func (v *Variable) String() string {
	if v == nil {
		return ""
	}
	return "$" + v.Name.String()
}

// A ListValue is an ordered list literal.
type ListValue struct {
	LBracket Pos
	Values   []*InputValue
	RBracket Pos
}

// String formats the list in GraphQL syntax.
func (lval *ListValue) String() string {
	buf := new(strings.Builder)
	buf.WriteByte('[')
	for i, v := range lval.Values {
		if i > 0 {
			buf.WriteString(", ")
		}
		buf.WriteString(v.String())
	}
	buf.WriteByte(']')
	return buf.String()
}

// IdenticalTo reports whether lval is structurally identical to lval2,
// ignoring token positions and input object field order. The result is
// undefined if either value contains an input object which contain multiple
// values for the same field name.
func (lval *ListValue) IdenticalTo(lval2 *ListValue) bool {
	if lval == nil {
		return lval2 == nil
	}
	if lval2 == nil {
		return false
	}
	if len(lval.Values) != len(lval2.Values) {
		return false
	}
	for i := range lval.Values {
		if !lval.Values[i].IdenticalTo(lval2.Values[i]) {
			return false
		}
	}
	return true
}

// An InputObjectValue is an unordered list of keyed input values.
type InputObjectValue struct {
	LBrace Pos
	Fields []*InputObjectField
	RBrace Pos
}

// String formats the value in GraphQL syntax.
func (ioval *InputObjectValue) String() string {
	buf := new(strings.Builder)
	buf.WriteByte('{')
	for i, field := range ioval.Fields {
		if i > 0 {
			buf.WriteString(", ")
		}
		buf.WriteString(field.Name.String())
		buf.WriteString(": ")
		buf.WriteString(field.Value.String())
	}
	buf.WriteByte('}')
	return buf.String()
}

// IdenticalTo reports whether ioval is structurally identical to ioval2,
// ignoring token positions and input object field order. The result is
// undefined if either value is or contains an input object which contain
// multiple values for the same field name.
func (ioval *InputObjectValue) IdenticalTo(ioval2 *InputObjectValue) bool {
	if ioval == nil {
		return ioval2 == nil
	}
	if ioval2 == nil {
		return false
	}
	return inputValuesIdentical(ioval.fieldMap(), ioval2.fieldMap())
}

func (ioval *InputObjectValue) fieldMap() map[string]*InputValue {
	if ioval == nil {
		return nil
	}
	m := make(map[string]*InputValue)
	for _, f := range ioval.Fields {
		m[f.Name.Value] = f.Value
	}
	return m
}

// An InputObjectField is a keyed input value.
type InputObjectField struct {
	Name  *Name
	Colon Pos
	Value *InputValue
}

// DefaultValue specifies the default value of an input.
// https://graphql.github.io/graphql-spec/June2018/#DefaultValue
type DefaultValue struct {
	Eq    Pos
	Value *InputValue
}

// VariableDefinitions is the set of variables an operation defines.
// https://graphql.github.io/graphql-spec/June2018/#Variable
type VariableDefinitions struct {
	LParen Pos
	Defs   []*VariableDefinition
	RParen Pos
}

// VariableDefinition is an element of VariableDefinitions.
// https://graphql.github.io/graphql-spec/June2018/#Variable
type VariableDefinition struct {
	Var     *Variable
	Colon   Pos
	Type    *TypeRef
	Default *DefaultValue
}

// A Name is an identifier.
// https://graphql.github.io/graphql-spec/June2018/#sec-Names
type Name struct {
	Value string
	Start Pos
}

// End returns the position of the byte after the last character of the name.
func (n *Name) End() Pos {
	return n.Start + Pos(len(n.Value))
}

// String returns the name or the empty string if the name is nil.
func (n *Name) String() string {
	if n == nil {
		return ""
	}
	return n.Value
}

// A TypeRef is a named type, a list type, or a non-null type.
// https://graphql.github.io/graphql-spec/June2018/#Type
type TypeRef struct {
	Named   *Name
	List    *ListType
	NonNull *NonNullType
}

// Start returns the byte offset of the start of the type reference.
func (ref *TypeRef) Start() Pos {
	switch {
	case ref.Named != nil:
		return ref.Named.Start
	case ref.List != nil:
		return ref.List.LBracket
	case ref.NonNull != nil:
		return ref.NonNull.Start()
	default:
		panic("unknown type reference")
	}
}

// String formats the type in GraphQL syntax.
func (ref *TypeRef) String() string {
	switch {
	case ref.Named != nil:
		return ref.Named.String()
	case ref.List != nil:
		return ref.List.String()
	case ref.NonNull != nil:
		return ref.NonNull.String()
	default:
		panic("unknown type reference")
	}
}

// ListType declares a homogenous sequence of another type.
// https://graphql.github.io/graphql-spec/June2018/#ListType
type ListType struct {
	LBracket Pos
	Type     *TypeRef
	RBracket Pos
}

// String formats the type in GraphQL syntax.
func (ltype *ListType) String() string {
	return "[" + ltype.Type.String() + "]"
}

// NonNullType declares a named or list type that cannot be null.
// https://graphql.github.io/graphql-spec/June2018/#Type
type NonNullType struct {
	Named *Name
	List  *ListType
	Pos   Pos
}

// Start returns the byte offset of the start of the type.
func (nn *NonNullType) Start() Pos {
	switch {
	case nn.Named != nil:
		return nn.Named.Start
	case nn.List != nil:
		return nn.List.LBracket
	default:
		panic("unknown non-null type")
	}
}

// String formats the type in GraphQL syntax.
func (nn *NonNullType) String() string {
	switch {
	case nn.Named != nil:
		return nn.Named.String() + "!"
	case nn.List != nil:
		return nn.List.String() + "!"
	default:
		panic("unknown non-null type")
	}
}

// A Description is a string that documents a type system definition.
// https://graphql.github.io/graphql-spec/June2018/#Description
type Description struct {
	Start Pos
	Raw   string
}

// Value returns the string value of the description.
func (d *Description) Value() string {
	if d == nil {
		return ""
	}
	if strings.HasPrefix(d.Raw, `"""`) {
		return parseBlockString(d.Raw)
	}
	return parseString(d.Raw)
}

// TypeDefinition holds a type definition.
// https://graphql.github.io/graphql-spec/June2018/#TypeDefinition
type TypeDefinition struct {
	// One of the following must be non-nil:

	Scalar      *ScalarTypeDefinition
	Object      *ObjectTypeDefinition
	Enum        *EnumTypeDefinition
	InputObject *InputObjectTypeDefinition
}

// Start returns the position of the type definition's first token.
func (defn *TypeDefinition) Start() Pos {
	switch {
	case defn.Scalar != nil:
		return defn.Scalar.Keyword
	case defn.Object != nil:
		return defn.Object.Keyword
	case defn.Enum != nil:
		return defn.Enum.Keyword
	case defn.InputObject != nil:
		return defn.InputObject.Keyword
	default:
		panic("unknown type definition")
	}
}

// Description returns the type definition's description or nil if it does not
// have one.
func (defn *TypeDefinition) Description() *Description {
	switch {
	case defn == nil:
		return nil
	case defn.Scalar != nil:
		return defn.Scalar.Description
	case defn.Object != nil:
		return defn.Object.Description
	case defn.Enum != nil:
		return defn.Enum.Description
	case defn.InputObject != nil:
		return defn.InputObject.Description
	default:
		return nil
	}
}

// Name returns the type definition's name.
func (defn *TypeDefinition) Name() *Name {
	switch {
	case defn == nil:
		return nil
	case defn.Scalar != nil:
		return defn.Scalar.Name
	case defn.Object != nil:
		return defn.Object.Name
	case defn.Enum != nil:
		return defn.Enum.Name
	case defn.InputObject != nil:
		return defn.InputObject.Name
	default:
		return nil
	}
}

func (defn *TypeDefinition) asDefinition() *Definition {
	return &Definition{Type: defn}
}

// ScalarTypeDefinition names a scalar type.
// https://graphql.github.io/graphql-spec/June2018/#ScalarTypeDefinition
type ScalarTypeDefinition struct {
	Description *Description
	Keyword     Pos
	Name        *Name
}

func (defn *ScalarTypeDefinition) asTypeDefinition() *TypeDefinition {
	return &TypeDefinition{Scalar: defn}
}

// ObjectTypeDefinition names an output object type.
// https://graphql.github.io/graphql-spec/June2018/#ObjectTypeDefinition
type ObjectTypeDefinition struct {
	Description *Description
	Keyword     Pos
	Name        *Name
	Fields      *FieldsDefinition
}

func (defn *ObjectTypeDefinition) asTypeDefinition() *TypeDefinition {
	return &TypeDefinition{Object: defn}
}

// FieldsDefinition is the list of fields in an ObjectTypeDefinition.
// https://graphql.github.io/graphql-spec/June2018/#FieldsDefinition
type FieldsDefinition struct {
	LBrace Pos
	Defs   []*FieldDefinition
	RBrace Pos
}

// FieldDefinition specifies a single field in an ObjectTypeDefinition.
// https://graphql.github.io/graphql-spec/June2018/#FieldsDefinition
type FieldDefinition struct {
	Description *Description
	Name        *Name
	Args        *ArgumentsDefinition
	Colon       Pos
	Type        *TypeRef
}

// ArgumentsDefinition specifies the arguments for a FieldDefinition.
// https://graphql.github.io/graphql-spec/June2018/#ArgumentsDefinition
type ArgumentsDefinition struct {
	LParen Pos
	Args   []*InputValueDefinition
	RParen Pos
}

// EnumTypeDefinition defines an enumeration type and its possible values.
// https://graphql.github.io/graphql-spec/June2018/#EnumTypeDefinition
type EnumTypeDefinition struct {
	Description *Description
	Keyword     Pos
	Name        *Name
	Values      *EnumValuesDefinition
}

func (defn *EnumTypeDefinition) asTypeDefinition() *TypeDefinition {
	return &TypeDefinition{Enum: defn}
}

// EnumValuesDefinition is a brace-delimited list of enumeration values.
// https://graphql.github.io/graphql-spec/June2018/#EnumValuesDefinition
type EnumValuesDefinition struct {
	LBrace Pos
	Values []*EnumValueDefinition
	RBrace Pos
}

// EnumValueDefinition is a possible value of an enumeration.
// https://graphql.github.io/graphql-spec/June2018/#EnumValueDefinition
type EnumValueDefinition struct {
	Description *Description
	Value       *Name
}

// InputObjectTypeDefinition names an input object type.
// https://graphql.github.io/graphql-spec/June2018/#InputObjectTypeDefinition
type InputObjectTypeDefinition struct {
	Description *Description
	Keyword     Pos
	Name        *Name
	Fields      *InputFieldsDefinition
}

func (defn *InputObjectTypeDefinition) asTypeDefinition() *TypeDefinition {
	return &TypeDefinition{InputObject: defn}
}

// InputFieldsDefinition is the list of fields in an InputObjectTypeDefinition.
// https://graphql.github.io/graphql-spec/June2018/#InputFieldsDefinition
type InputFieldsDefinition struct {
	LBrace Pos
	Defs   []*InputValueDefinition
	RBrace Pos
}

// InputValueDefinition specifies an argument in a FieldDefinition or a field
// in an InputObjectTypeDefinition.
// https://graphql.github.io/graphql-spec/June2018/#InputValueDefinition
type InputValueDefinition struct {
	Description *Description
	Name        *Name
	Colon       Pos
	Type        *TypeRef
	Default     *DefaultValue
}
