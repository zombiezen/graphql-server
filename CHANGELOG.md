# @zombiezen's GraphQL Go server changelog

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased][]

[Unreleased]: https://github.com/zombiezen/graphql-server/compare/v0.1.0...HEAD

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
