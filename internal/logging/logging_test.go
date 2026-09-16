package logging_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/danilosciarra/launchpad/config"
	"github.com/danilosciarra/launchpad/internal/logging"
	"github.com/danilosciarra/launchpad/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureStdout runs fn with os.Stdout redirected to a pipe and returns
// everything written to it.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	r, w, err := os.Pipe()
	require.NoError(t, err)

	orig := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	fn()
	require.NoError(t, w.Close())

	var sb strings.Builder
	buf := make([]byte, 4096)
	for {
		n, err := r.Read(buf)
		sb.Write(buf[:n])
		if err != nil {
			break
		}
	}
	require.NoError(t, r.Close())
	return sb.String()
}

func TestNew_DefaultsToJSONInfoStdout(t *testing.T) {
	out := captureStdout(t, func() {
		logger := logging.New(config.LogConfig{})
		logger.Debugf("invisible at info level")
		logger.Infof("hello %s", "world")
	})

	lines := strings.Split(strings.TrimSpace(out), "\n")
	require.Len(t, lines, 1, "debug output must be filtered out at info level")

	var entry map[string]any
	require.NoError(t, json.Unmarshal([]byte(lines[0]), &entry))
	assert.Equal(t, "hello world", entry["msg"])
	assert.Equal(t, "info", entry["level"])
}

func TestNew_TextFormat(t *testing.T) {
	out := captureStdout(t, func() {
		logging.New(config.LogConfig{Format: "text"}).Infof("plain text entry")
	})

	assert.Contains(t, out, "plain text entry")
	assert.False(t, json.Valid([]byte(out)), "text format must not emit JSON")
}

func TestNew_LevelIsHonored(t *testing.T) {
	out := captureStdout(t, func() {
		logging.New(config.LogConfig{Level: "debug"}).Debugf("debug entry")
	})
	assert.Contains(t, out, "debug entry")

	out = captureStdout(t, func() {
		logging.New(config.LogConfig{Level: "error"}).Warnf("warn entry")
	})
	assert.Empty(t, strings.TrimSpace(out))
}

func TestNew_InvalidLevelFallsBackToInfo(t *testing.T) {
	out := captureStdout(t, func() {
		logger := logging.New(config.LogConfig{Level: "not-a-level"})
		logger.Debugf("debug entry")
		logger.Infof("info entry")
	})

	assert.NotContains(t, out, "debug entry")
	assert.Contains(t, out, "info entry")
}

func TestWithFields_AnnotatesEntriesAndIsCumulative(t *testing.T) {
	out := captureStdout(t, func() {
		logger := logging.New(config.LogConfig{})
		logger.
			WithFields(log.Fields{"service": "orders"}).
			WithFields(log.Fields{"requestID": "abc-123"}).
			Warnf("careful")
	})

	var entry map[string]any
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(out)), &entry))
	assert.Equal(t, "orders", entry["service"])
	assert.Equal(t, "abc-123", entry["requestID"])
	assert.Equal(t, "warning", entry["level"])
	assert.Equal(t, "careful", entry["msg"])
}

func TestWithFields_DoesNotMutateParentLogger(t *testing.T) {
	out := captureStdout(t, func() {
		logger := logging.New(config.LogConfig{})
		_ = logger.WithFields(log.Fields{"scoped": "yes"})
		logger.Errorf("bare entry")
	})

	var entry map[string]any
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(out)), &entry))
	assert.NotContains(t, entry, "scoped")
	assert.Equal(t, "error", entry["level"])
}

func TestNew_FileOutput(t *testing.T) {
	// The file writer is hardcoded to ./app.log, so run from a temp dir.
	// t.TempDir is not used here because the log file stays open for the
	// lifetime of the process, which breaks its automatic cleanup on Windows.
	dir, err := os.MkdirTemp("", "launchpad-logging")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	cwd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(cwd) })

	logging.New(config.LogConfig{Output: "file"}).Infof("to file")

	contents, err := os.ReadFile("app.log")
	require.NoError(t, err)
	assert.Contains(t, string(contents), "to file")
}
