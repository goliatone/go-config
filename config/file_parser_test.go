package config

import (
	"os"
	"path/filepath"
	"testing"
)

type testConfig struct {
	Key string `koanf:"key"`
}

func TestInferConfigFiletype(t *testing.T) {
	tests := []struct {
		path     string
		expected ConfigFileType
	}{
		{"testdata/config.json", FileTypeJSON},
		{"testdata/config.yaml", FileTypeYAML},
		{"testdata/config.yml", FileTypeYAML},
		{"testdata/config.toml", FileTypeTOML},
		// unknown extension should default to JSON
		// unless a default is provided
		{"testdata/config.unknown", FileTypeJSON},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := inferConfigFiletype(tt.path)
			if got != tt.expected {
				t.Errorf("inferConfigFiletype(%q) = %v, want %v", tt.path, got, tt.expected)
			}
		})
	}
}

func TestParserForConfigFiles(t *testing.T) {
	baseDir, err := filepath.Abs("testdata")
	if err != nil {
		t.Fatalf("failed to get absolute path for testdata: %v", err)
	}

	tests := []struct {
		filename     string
		expectedType ConfigFileType
		expectedKey  string
	}{
		{"config.json", FileTypeJSON, "jsonValue"},
		{"config.yaml", FileTypeYAML, "yamlValue"},
		{"config.toml", FileTypeTOML, "tomlValue"},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			path := filepath.Join(baseDir, tt.filename)

			fileType := inferConfigFiletype(path)
			if fileType != tt.expectedType {
				t.Errorf("for %q, expected file type %v, got %v", tt.filename, tt.expectedType, fileType)
			}

			parser := fileType.Parser()

			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("failed to read file %q: %v", path, err)
			}

			data, err := parser.Unmarshal(content)
			if err != nil {
				t.Fatalf("failed to parse file %q: %v", path, err)
			}

			if val, ok := data["key"]; !ok {
				t.Errorf("key not found in parsed data for file %q", tt.filename)
			} else if val != tt.expectedKey {
				t.Errorf("for file %q, expected key value %q, got %v", tt.filename, tt.expectedKey, val)
			}
		})
	}
}
