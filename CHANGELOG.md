# zombiezen GraphQL Go server changelog

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## Unreleased

Known unimplemented features:

-  Extensions
-  Directives
-  Fragments
-  Interface types
-  Union types
-  Subscriptions
-  Concurrent field resolution
-  Validation for custom scalar types
-  Unmarshaling of arguments into Go types

### Added

-  Scalar types
-  Field arguments
-  Field methods can inspect their selection set
-  Schema validation
-  Validation
-  Marshaling of Go types into GraphQL output types
-  Context propagation
-  Precise error reporting
