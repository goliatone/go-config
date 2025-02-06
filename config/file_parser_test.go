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
		{"testdata/fileparser/config.json", FileTypeJSON},
		{"testdata/fileparser/config.yaml", FileTypeYAML},
		{"testdata/fileparser/config.yml", FileTypeYAML},
		{"testdata/fileparser/config.toml", FileTypeTOML},
		// unknown extension should default to JSON
		// unless a default is provided
		{"testdata/fileparser/config.unknown", FileTypeJSON},
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
		{"fileparser/config.json", FileTypeJSON, "jsonValue"},
		{"fileparser/config.yaml", FileTypeYAML, "yamlValue"},
		{"fileparser/config.toml", FileTypeTOML, "tomlValue"},
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
