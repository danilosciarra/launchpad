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
