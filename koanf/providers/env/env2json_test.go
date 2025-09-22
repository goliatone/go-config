package env

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func setEnv(t *testing.T, key, value string) {
	t.Helper()
	t.Setenv(key, value)
}

func unsetEnv(t *testing.T, key string) {
	t.Helper()
	t.Setenv(key, "")
}

func TestProvider(t *testing.T) {
	tests := []struct {
		name     string
		prefix   string
		delim    string
		envVars  map[string]string
		expected string
	}{
		{
			name:   "Single key",
			prefix: "TEST_",
			delim:  "__",
			envVars: map[string]string{
				"TEST_DATABASE__PASSWORD": "password",
			},
			expected: `{"TEST_DATABASE":{"PASSWORD":"password"}}`,
		},
		{
			name:   "Array handling",
			prefix: "TEST_",
			delim:  "__",
			envVars: map[string]string{
				"TEST_DATABASE__0__PASSWORD": "password_1",
				"TEST_DATABASE__1__PASSWORD": "password_2",
				"TEST_DATABASE__2__PASSWORD": "password_3",
			},
			expected: `{"TEST_DATABASE":[{"PASSWORD":"password_1"},{"PASSWORD":"password_2"},{"PASSWORD":"password_3"}]}`,
		},
		{
			name:   "Nested keys",
			prefix: "TEST_",
			delim:  "__",
			envVars: map[string]string{
				"TEST_PARENT__CHILD__KEY": "value",
			},
			expected: `{"TEST_PARENT":{"CHILD":{"KEY":"value"}}}`,
		},
		{
			name:   "Prefix filtering",
			prefix: "TEST_",
			delim:  "__",
			envVars: map[string]string{
				"TEST_KEY":         "app_value",
				"OTHER_KEY":        "other_value",
				"TEST_OTHER__KEY":  "app_other_value",
				"OTHER_OTHER__KEY": "other_other_value",
			},
			expected: `{"TEST_KEY":"app_value","TEST_OTHER":{"KEY":"app_other_value"}}`,
		},
		{
			name:   "No prefix",
			prefix: "",
			delim:  "__",
			envVars: map[string]string{
				"DATABASE__PASSWORD": "password",
			},
			expected: `{"DATABASE":{"PASSWORD":"password"}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Clearenv()
			for key, value := range tt.envVars {
				setEnv(t, key, value)
				defer unsetEnv(t, key)
			}

			provider := Provider(tt.prefix, tt.delim, nil)
			data, err := provider.ReadBytes()
			assert.NoError(t, err)
			assert.JSONEq(t, tt.expected, string(data))
		})
	}
}

func TestProviderWithCallback(t *testing.T) {
	setEnv(t, "TEST_DATABASE__PASSWORD", "password")
	defer unsetEnv(t, "TEST_DATABASE__PASSWORD")

	provider := Provider("TEST_", "__", func(s string) string {
		return strings.ToLower(strings.Replace(s, "TEST_", "", 1))
	})
	data, err := provider.ReadBytes()
	assert.NoError(t, err)
	assert.JSONEq(t, `{"database":{"password":"password"}}`, string(data))
}

func TestProviderWithCallback_format(t *testing.T) {
	tests := []struct {
		name     string
		prefix   string
		delim    string
		envVars  map[string]string
		expected string
	}{
		{
			name:   "Single key",
			prefix: "TEST_",
			delim:  "__",
			envVars: map[string]string{
				"TEST_DATABASE__PASSWORD": "password",
			},
			expected: `{"database":{"password":"password"}}`,
		},
		{
			name:   "Array handling",
			prefix: "TEST_",
			delim:  "__",
			envVars: map[string]string{
				"TEST_DATABASE__0__PASSWORD": "password_1",
				"TEST_DATABASE__1__PASSWORD": "password_2",
				"TEST_DATABASE__2__PASSWORD": "password_3",
			},
			expected: `{"database":[{"password":"password_1"},{"password":"password_2"},{"password":"password_3"}]}`,
		},
		{
			name:   "Nested keys",
			prefix: "TEST_",
			delim:  "__",
			envVars: map[string]string{
				"TEST_PARENT__CHILD__KEY": "value",
			},
			expected: `{"parent":{"child":{"key":"value"}}}`,
		},
		{
			name:   "Prefix filtering",
			prefix: "TEST_",
			delim:  "__",
			envVars: map[string]string{
				"TEST_KEY":         "app_value",
				"OTHER_KEY":        "other_value",
				"TEST_OTHER__KEY":  "app_other_value",
				"OTHER_OTHER__KEY": "other_other_value",
			},
			expected: `{"key":"app_value","other":{"key":"app_other_value"}}`,
		},
		{
			name:   "Multiple objects",
			prefix: "TEST_",
			delim:  "__",
			envVars: map[string]string{
				"TEST_DATABASE__0__HOST": "primary.db",
				"TEST_DATABASE__0__PORT": "5432",
				"TEST_DATABASE__1__HOST": "replica.db",
				"TEST_DATABASE__1__PORT": "5432",
			},
			expected: `{"database":[{"host":"primary.db","port":"5432"},{"host":"replica.db","port":"5432"}]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Clearenv()
			for key, value := range tt.envVars {
				setEnv(t, key, value)
				defer unsetEnv(t, key)
			}
			provider := Provider("TEST_", "__", func(s string) string {
				return strings.ToLower(strings.Replace(s, "TEST_", "", 1))
			})
			data, err := provider.ReadBytes()
			assert.NoError(t, err)
			assert.JSONEq(t, tt.expected, string(data))
		})
	}
}

func TestProviderWithValue(t *testing.T) {
	setEnv(t, "TEST_DATABASE__PASSWORD", "password")
	defer unsetEnv(t, "TEST_DATABASE__PASSWORD")

	provider := ProviderWithValue("TEST_", "__", func(key string, value string) (string, any) {
		return strings.ToLower(strings.Replace(key, "TEST_", "", 1)), []string{value}
	})
	data, err := provider.ReadBytes()
	assert.NoError(t, err)
	assert.JSONEq(t, `{"database":{"password":["password"]}}`, string(data))
}

func TestReadNotSupported(t *testing.T) {
	provider := Provider("", "__", nil)
	_, err := provider.Read()
	assert.Error(t, err)
	assert.Equal(t, "envextended provider does not support this method", err.Error())
}
