package config

import (
	"fmt"
	"path/filepath"

	"github.com/knadh/koanf/parsers/json"
	"github.com/knadh/koanf/parsers/toml"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/v2"
)

type ConfigFileType string

func (c ConfigFileType) String() string {
	return string(c)
}

func (c ConfigFileType) Valid() error {
	switch c {
	case FileTypeJSON, FileTypeYAML, FileTypeTOML:
		return nil
	default:
		return fmt.Errorf("invalid config file type: %s", c)
	}
}

func (c ConfigFileType) Parser() koanf.Parser {
	switch c {
	case FileTypeJSON:
		return json.Parser()
	case FileTypeTOML:
		return toml.Parser()
	case FileTypeYAML:
		return yaml.Parser()
	default:
		panic(fmt.Errorf("invalid config file type: %s", c))
	}
}

const (
	FileTypeYAML ConfigFileType = "yaml"
	FileTypeTOML ConfigFileType = "toml"
	FileTypeJSON ConfigFileType = "json"
)

func inferConfigFiletype(path string, defaultFileType ...ConfigFileType) ConfigFileType {
	ext := filepath.Ext(path)
	switch ext {
	case ".toml":
		return FileTypeTOML
	case ".json":
		return FileTypeJSON
	case ".yaml", ".yml":
		return FileTypeYAML
	}

	if len(defaultFileType) > 0 {
		return defaultFileType[0]
	}

	return FileTypeJSON
}
