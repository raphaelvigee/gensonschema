package gen

import (
	"github.com/santhosh-tekuri/jsonschema/v5"
	"io"
)

type Config struct {
	Out            io.Writer
	Schemas        []*jsonschema.Schema
	ShouldGenerate func(schema *jsonschema.Schema) bool
}

type PathConfig struct {
	Location string
	Bind     string
}
