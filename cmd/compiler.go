package main

import (
	"bytes"
	"encoding/json"
	"github.com/santhosh-tekuri/jsonschema/v5"
	"gopkg.in/yaml.v3"
	"io"
	"os"
	"path/filepath"
)

func openYaml(url string) (io.ReadCloser, error) {
	b, err := os.ReadFile(url)
	if err != nil {
		return nil, err
	}

	var m interface{}
	err = yaml.Unmarshal(b, &m)
	if err != nil {
		return nil, err
	}

	b, err = json.Marshal(m)
	if err != nil {
		return nil, err
	}

	return io.NopCloser(bytes.NewReader(b)), nil
}

func openJson(url string) (io.ReadCloser, error) {
	f, err := os.Open(url)
	if err != nil {
		return nil, err
	}

	return f, err
}

func newCompiler(config Config) (*jsonschema.Compiler, error) {
	c := jsonschema.NewCompiler()
	c.ExtractAnnotations = true

	var err error
	var r io.ReadCloser
	for _, resource := range config.Resources {
		ext := filepath.Ext(resource.Source)
		switch ext {
		case ".json":
			r, err = openJson(resource.Source)
			if err != nil {
				return nil, err
			}
		case ".yaml", ".yml":
			r, err = openYaml(resource.Source)
			if err != nil {
				return nil, err
			}
		}

		err = c.AddResource(resource.Url, r)
		_ = r.Close()
		if err != nil {
			return nil, err
		}
	}

	return c, nil
}
