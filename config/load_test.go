package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/danilosciarra/launchpad/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeConfig(t *testing.T, contents string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	require.NoError(t, os.WriteFile(path, []byte(contents), 0o600))
	return path
}

func TestLoadFile_FillsAndValidates(t *testing.T) {
	t.Setenv("TEST_HTTP_PORT", "9090")

	path := writeConfig(t, `{
		"name": "orders",
		"version": "1.0.0",
		"http": {"host": "0.0.0.0", "port": {{.TEST_HTTP_PORT}}},
		"shutdownTimeout": "45s"
	}`)

	var cfg config.Config
	err := config.LoadFile(path, &cfg)
	require.NoError(t, err)

	assert.Equal(t, "orders", cfg.Name)
	assert.Equal(t, "0.0.0.0", cfg.HTTP.Host)
	assert.Equal(t, 9090, cfg.HTTP.Port)
	assert.Equal(t, 45*time.Second, cfg.ShutdownTimeout)
}

func TestLoadFile_MissingRequiredField(t *testing.T) {
	path := writeConfig(t, `{"http": {"host": "0.0.0.0", "port": 8080}}`)

	var cfg config.Config
	err := config.LoadFile(path, &cfg)
	assert.Error(t, err)
}

func TestLoadFile_EmbeddedConfig(t *testing.T) {
	type AppConfig struct {
		config.Config
		FeatureFlag bool `json:"featureFlag"`
	}

	path := writeConfig(t, `{
		"name": "orders",
		"version": "1.0.0",
		"http": {"host": "0.0.0.0", "port": 8080},
		"featureFlag": true
	}`)

	var cfg AppConfig
	require.NoError(t, config.LoadFile(path, &cfg))
	assert.True(t, cfg.FeatureFlag)
	assert.Equal(t, 8080, cfg.HTTP.Port)
}

func TestLoadFile_FileNotFound(t *testing.T) {
	var cfg config.Config
	err := config.LoadFile(filepath.Join(t.TempDir(), "missing.json"), &cfg)
	assert.Error(t, err)
}

func TestLoadFile_InvalidJSON(t *testing.T) {
	path := writeConfig(t, `{"name": "orders",`)

	var cfg config.Config
	err := config.LoadFile(path, &cfg)
	assert.Error(t, err)
}

func TestLoadFile_InvalidTemplate(t *testing.T) {
	path := writeConfig(t, `{"name": "{{.UNCLOSED"}`)

	var cfg config.Config
	err := config.LoadFile(path, &cfg)
	assert.Error(t, err)
}

func TestLoadFile_MissingEnvVarIsAnError(t *testing.T) {
	path := writeConfig(t, `{
		"name": "{{.LAUNCHPAD_TEST_UNSET_NAME}}",
		"version": "1.0.0",
		"http": {"host": "0.0.0.0", "port": 8080}
	}`)

	// A referenced but unset variable must fail loudly rather than expand to
	// a placeholder and start a misconfigured application.
	var cfg config.Config
	assert.Error(t, config.LoadFile(path, &cfg))
}

func TestLoadFile_OptionalSectionsStayNil(t *testing.T) {
	path := writeConfig(t, `{
		"name": "orders",
		"version": "1.0.0",
		"http": {"host": "0.0.0.0", "port": 8080}
	}`)

	var cfg config.Config
	require.NoError(t, config.LoadFile(path, &cfg))

	assert.Nil(t, cfg.GRPC)
	assert.Nil(t, cfg.Dapr)
	assert.Zero(t, cfg.ShutdownTimeout)
	assert.Equal(t, config.LogConfig{}, cfg.Log)
}

func TestLoadFile_PopulatesOptionalSections(t *testing.T) {
	path := writeConfig(t, `{
		"name": "orders",
		"version": "1.0.0",
		"http": {"host": "0.0.0.0", "port": 8080},
		"grpc": {"host": "127.0.0.1", "port": 9090},
		"dapr": {"host": "127.0.0.1", "port": 50001},
		"log": {"level": "debug", "format": "text", "output": "stdout"}
	}`)

	var cfg config.Config
	require.NoError(t, config.LoadFile(path, &cfg))

	require.NotNil(t, cfg.GRPC)
	assert.Equal(t, config.Address{Host: "127.0.0.1", Port: 9090}, *cfg.GRPC)
	require.NotNil(t, cfg.Dapr)
	assert.Equal(t, 50001, cfg.Dapr.Port)
	assert.Equal(t, config.LogConfig{Level: "debug", Format: "text", Output: "stdout"}, cfg.Log)
}

func TestLoadFile_CoercesStringValues(t *testing.T) {
	t.Setenv("LAUNCHPAD_TEST_PORT", "8081")
	t.Setenv("LAUNCHPAD_TEST_TIMEOUT", "90s")
	t.Setenv("LAUNCHPAD_TEST_FLAG", "true")
	t.Setenv("LAUNCHPAD_TEST_RATIO", "0.25")
	t.Setenv("LAUNCHPAD_TEST_RETRIES", "7")

	type AppConfig struct {
		config.Config
		Flag    bool    `json:"flag"`
		Ratio   float64 `json:"ratio"`
		Retries uint16  `json:"retries"`
	}

	path := writeConfig(t, `{
		"name": "orders",
		"version": "1.0.0",
		"http": {"host": "0.0.0.0", "port": "{{.LAUNCHPAD_TEST_PORT}}"},
		"shutdownTimeout": "{{.LAUNCHPAD_TEST_TIMEOUT}}",
		"flag": "{{.LAUNCHPAD_TEST_FLAG}}",
		"ratio": "{{.LAUNCHPAD_TEST_RATIO}}",
		"retries": "{{.LAUNCHPAD_TEST_RETRIES}}"
	}`)

	var cfg AppConfig
	require.NoError(t, config.LoadFile(path, &cfg))

	assert.Equal(t, 8081, cfg.HTTP.Port)
	assert.Equal(t, 90*time.Second, cfg.ShutdownTimeout)
	assert.True(t, cfg.Flag)
	assert.InDelta(t, 0.25, cfg.Ratio, 1e-9)
	assert.Equal(t, uint16(7), cfg.Retries)
}

func TestLoadFile_DurationAsNanoseconds(t *testing.T) {
	path := writeConfig(t, `{
		"name": "orders",
		"version": "1.0.0",
		"http": {"host": "0.0.0.0", "port": 8080},
		"shutdownTimeout": 5000000000
	}`)

	var cfg config.Config
	require.NoError(t, config.LoadFile(path, &cfg))
	assert.Equal(t, 5*time.Second, cfg.ShutdownTimeout)
}

func TestLoadFile_KeysAreCaseInsensitive(t *testing.T) {
	path := writeConfig(t, `{
		"Name": "orders",
		"VERSION": "1.0.0",
		"Http": {"Host": "0.0.0.0", "PORT": 8080}
	}`)

	var cfg config.Config
	require.NoError(t, config.LoadFile(path, &cfg))

	assert.Equal(t, "orders", cfg.Name)
	assert.Equal(t, "1.0.0", cfg.Version)
	assert.Equal(t, 8080, cfg.HTTP.Port)
}

func TestLoadFile_UnknownKeysIgnored(t *testing.T) {
	path := writeConfig(t, `{
		"name": "orders",
		"version": "1.0.0",
		"http": {"host": "0.0.0.0", "port": 8080},
		"somethingElse": {"deeply": {"nested": true}}
	}`)

	var cfg config.Config
	assert.NoError(t, config.LoadFile(path, &cfg))
}

func TestLoadFile_SkipsUnexportedAndIgnoredFields(t *testing.T) {
	type AppConfig struct {
		config.Config
		Secret  string `json:"-"`
		Renamed string `json:"aliasedName"`
	}

	path := writeConfig(t, `{
		"name": "orders",
		"version": "1.0.0",
		"http": {"host": "0.0.0.0", "port": 8080},
		"secret": "should-not-be-set",
		"aliasedName": "from-alias"
	}`)

	var cfg AppConfig
	require.NoError(t, config.LoadFile(path, &cfg))

	assert.Empty(t, cfg.Secret)
	assert.Equal(t, "from-alias", cfg.Renamed)
}

func TestLoad_ReadsConfigNextToExecutable(t *testing.T) {
	exe, err := os.Executable()
	require.NoError(t, err)

	path := filepath.Join(filepath.Dir(exe), "config.json")
	if _, err := os.Stat(path); err == nil {
		t.Skip("a config.json already exists next to the test binary")
	}

	require.NoError(t, os.WriteFile(path, []byte(`{
		"name": "orders",
		"version": "1.0.0",
		"http": {"host": "0.0.0.0", "port": 8080}
	}`), 0o600))
	t.Cleanup(func() { _ = os.Remove(path) })

	var cfg config.Config
	require.NoError(t, config.Load(&cfg))
	assert.Equal(t, "orders", cfg.Name)
}
