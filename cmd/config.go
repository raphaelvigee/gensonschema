package main

import (
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
)

type Config struct {
	Output struct {
		Package string `yaml:"package"`
		File    string `yaml:"file"`
	}
	Resources []struct {
		Url    string `yaml:"url"`
		Source string `yaml:"source"`
	} `yaml:"resources"`
	Generate []string `yaml:"generate"`
}

func ParseConfig(path string) (Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}

	var config Config
	err = yaml.Unmarshal(b, &config)
	if err != nil {
		return Config{}, err
	}

	for i, resource := range config.Resources {
		absUrl := filepath.Join(filepath.Dir(path), resource.Source)

		config.Resources[i].Source = absUrl
	}

	config.Output.File = filepath.Join(filepath.Dir(path), config.Output.File)

	return config, nil
}
