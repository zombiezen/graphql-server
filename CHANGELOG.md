# zombiezen GraphQL Go server changelog

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## Unreleased

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

-  Extensions
-  Directives
-  Fragments
-  Interface types
-  Union types
-  Subscriptions
-  Concurrent field resolution
-  Validation for custom scalar types
-  Unmarshaling of arguments into Go types
-  Metrics and trace spans
-  Explicit `schema` blocks
