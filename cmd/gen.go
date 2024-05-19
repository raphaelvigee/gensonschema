package main

import (
	"bytes"
	"flag"
	"fmt"
	"github.com/raphaelvigee/gensonschema/gen"
	"github.com/santhosh-tekuri/jsonschema/v5"
	"go/format"
	"log"
	"os"
	"path/filepath"
)

func main() {
	err := entrypoint()
	if err != nil {
		log.Print(err)
		os.Exit(1)
		return
	}
}

func entrypoint() error {
	configPath := flag.String("config", "gensonschema.yaml", "path to config file")
	flag.Parse()

	path, err := filepath.Abs(*configPath)
	if err != nil {
		return err
	}

	config, err := ParseConfig(path)
	if err != nil {
		return err
	}

	c, err := newCompiler(config)
	if err != nil {
		return err
	}

	var schemas []*jsonschema.Schema
	for _, s := range config.Generate {
		schema, err := c.Compile(s)
		if err != nil {
			return err
		}

		schemas = append(schemas, schema)
	}

	var buf bytes.Buffer

	err = gen.Gen(gen.Config{
		Out:     &buf,
		Schemas: schemas,
		ShouldGenerate: func(schema *jsonschema.Schema) bool {
			fmt.Println(schema.Location)
			return true
		},
	})
	if err != nil {
		return err
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		fmt.Println(err)
		formatted = buf.Bytes()
	}

	err = os.MkdirAll(filepath.Dir(config.Output.File), os.ModePerm)
	if err != nil {
		return err
	}

	err = os.WriteFile(config.Output.File, formatted, os.ModePerm)
	if err != nil {
		return err
	}

	return nil
}
