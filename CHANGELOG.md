# @zombiezen's GraphQL Go Server Changelog

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased][]

This release focused on implementing the functionality necessary to make GraphQL
servers operate correctly with [GraphQL Playground][]. Beyond introspection
improvements, the largest new user-facing feature is [fragments][].

[Unreleased]: https://github.com/zombiezen/graphql-server/compare/v0.1.0...HEAD
[GraphQL Playground]: https://github.com/prisma-labs/graphql-playground
[fragments]: https://graphql.org/learn/queries/#fragments

### Added

-  Fragments are now fully supported. There's no new API: fields from fragments
   are added to the `SelectionSet` like normal fields. ([#9][])
-  Schemas can now use the `@deprecated` annotation on fields and annotations.
   ([#11][])
-  A [new option][IgnoreDescriptions] for `graphql.ParseSchema` permits
   stripping descriptions to avoid leaking information from introspection.
   ([#4][])
-  A new function, [`graphql.ParseSchemaFile`][], provides a shortcut for
   reading a schema from the local filesystem. ([#25][])
-  [Code of Conduct][]

[#4]: https://github.com/zombiezen/graphql-server/issues/4
[#9]: https://github.com/zombiezen/graphql-server/issues/9
[#11]: https://github.com/zombiezen/graphql-server/issues/11
[#25]: https://github.com/zombiezen/graphql-server/issues/25
[Code of Conduct]: https://github.com/zombiezen/graphql-server/blob/master/CODE_OF_CONDUCT.md
[IgnoreDescriptions]: https://godoc.org/zombiezen.com/go/graphql-server/graphql#SchemaOptions.IgnoreDescriptions
[`graphql.ParseSchemaFile`]: https://godoc.org/zombiezen.com/go/graphql-server/graphql#ParseSchemaFile

### Changed

-  `graphqlhttp.Handler` handles `OPTIONS` requests by returning 204 No Content.
   ([#21][])
-  GraphQL documents now have depth and size limits. ([#1][])

[#1]: https://github.com/zombiezen/graphql-server/issues/1
[#21]: https://github.com/zombiezen/graphql-server/issues/21

### Fixed

-  Permit field merging. ([#2][] and [#24][])
-  Built-in types are now surfaced in introspection. ([#5][])
-  Requesting `__schema.directives` no longer causes an error.
-  `__type.interfaces` will be an empty list for objects.
-  When an error is encountered when resolving an element of a list with
   nullable elements, the list element is `null` instead of the whole list.
   ([#3][])

[#2]: https://github.com/zombiezen/graphql-server/issues/2
[#3]: https://github.com/zombiezen/graphql-server/issues/3
[#5]: https://github.com/zombiezen/graphql-server/issues/5
[#24]: https://github.com/zombiezen/graphql-server/issues/24

## [0.1.0][]

This is the first tagged release of the GraphQL Go server library. The main
goal for this release was to demonstrate the feasibility of this API's approach
against some small demo applications and develop a backlog for future
development work.

[0.1.0]: https://github.com/zombiezen/graphql-server/releases/tag/v0.1.0

### Added

-  Scalar types
-  Enum types
-  List types
-  Input object types
-  Field arguments
-  Variables
-  Field methods can inspect their selection set
-  Schema validation
-  Validation
-  Introspection (although GraphQL playgrounds tend to use fragments, which
   aren't implemented yet)
-  Marshaling of Go types into GraphQL output types
-  Context propagation
-  Precise error reporting
-  Verification of Go types as result types

### Not Implemented

-  Fragments ([#9][])
-  Extensions ([#10][])
-  The `@deprecated` directive ([#11][])
-  The `@skip` and `@include` directives ([#12][])
-  User-defined directives ([#13][])
-  Interface types ([#14][])
-  Union types ([#15][])
-  Subscriptions ([#16][])
-  Concurrent field resolution ([#8][])
-  Validation for custom scalar types ([#17][])
-  Unmarshaling of arguments into Go types ([#18][])
-  Metrics and trace spans ([#7][])
-  Explicit `schema` blocks ([#6][])

[#6]: https://github.com/zombiezen/graphql-server/issues/6
[#7]: https://github.com/zombiezen/graphql-server/issues/7
[#8]: https://github.com/zombiezen/graphql-server/issues/8
[#9]: https://github.com/zombiezen/graphql-server/issues/9
[#10]: https://github.com/zombiezen/graphql-server/issues/10
[#11]: https://github.com/zombiezen/graphql-server/issues/11
[#12]: https://github.com/zombiezen/graphql-server/issues/12
[#13]: https://github.com/zombiezen/graphql-server/issues/13
[#14]: https://github.com/zombiezen/graphql-server/issues/14
[#15]: https://github.com/zombiezen/graphql-server/issues/15
[#16]: https://github.com/zombiezen/graphql-server/issues/16
[#17]: https://github.com/zombiezen/graphql-server/issues/17
[#18]: https://github.com/zombiezen/graphql-server/issues/18
