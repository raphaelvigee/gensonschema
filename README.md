# gensonschema

> **G**enerate J**son** **Schema**

gensonschema generate type-safe code from JSON Schema, with full support for allOf, oneOf, anyOf

## Usage

Create a `gensonschema.yaml`:
```yaml
output:
  package: gen
  file: generated.go

resources:
  - url: simple.yaml
    source: ./testdata/simple.yaml

generate:
  - simple.yaml
```

At the top of a go file, add:
```go
//go:generate go run github.com/raphaelvigee/gensonschema/cmd
```
