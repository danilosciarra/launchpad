package httpmiddleware_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/danilosciarra/launchpad/internal/httpmiddleware"
	"github.com/danilosciarra/launchpad/log"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

type entry struct {
	level   string
	message string
	fields  log.Fields
}

// recordingLogger captures entries, merging the fields accumulated through
// successive WithFields calls.
type recordingLogger struct {
	mu      *sync.Mutex
	entries *[]entry
	fields  log.Fields
}

func newRecordingLogger() *recordingLogger {
	return &recordingLogger{mu: &sync.Mutex{}, entries: &[]entry{}, fields: log.Fields{}}
}

func (l *recordingLogger) record(level, message string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	*l.entries = append(*l.entries, entry{level: level, message: message, fields: l.fields})
}

func (l *recordingLogger) Debugf(format string, _ ...any) { l.record("debug", format) }
func (l *recordingLogger) Infof(format string, _ ...any)  { l.record("info", format) }
func (l *recordingLogger) Warnf(format string, _ ...any)  { l.record("warn", format) }
func (l *recordingLogger) Errorf(format string, _ ...any) { l.record("error", format) }

func (l *recordingLogger) WithFields(fields log.Fields) log.Logger {
	merged := log.Fields{}
	for k, v := range l.fields {
		merged[k] = v
	}
	for k, v := range fields {
		merged[k] = v
	}
	return &recordingLogger{mu: l.mu, entries: l.entries, fields: merged}
}

func (l *recordingLogger) last() entry {
	l.mu.Lock()
	defer l.mu.Unlock()
	return (*l.entries)[len(*l.entries)-1]
}

// serve runs a single request through the RequestLogger middleware.
func serve(t *testing.T, logger log.Logger, handler gin.HandlerFunc) {
	t.Helper()

	engine := gin.New()
	engine.Use(httpmiddleware.RequestLogger(logger))
	engine.GET("/orders", handler)

	req := httptest.NewRequest(http.MethodGet, "/orders?page=2", nil)
	engine.ServeHTTP(httptest.NewRecorder(), req)
}

func TestRequestLogger_SuccessLogsAtInfo(t *testing.T) {
	logger := newRecordingLogger()
	serve(t, logger, func(c *gin.Context) { c.Status(http.StatusOK) })

	got := logger.last()
	assert.Equal(t, "info", got.level)
	assert.Equal(t, "request handled", got.message)
	assert.Equal(t, http.StatusOK, got.fields["status"])
	assert.Equal(t, http.MethodGet, got.fields["method"])
	assert.Equal(t, "/orders", got.fields["path"])
	assert.Equal(t, "page=2", got.fields["query"])
	assert.Contains(t, got.fields, "latency")
	assert.Contains(t, got.fields, "ip")
}

func TestRequestLogger_ClientErrorLogsAtWarn(t *testing.T) {
	logger := newRecordingLogger()
	serve(t, logger, func(c *gin.Context) { c.Status(http.StatusNotFound) })

	got := logger.last()
	assert.Equal(t, "warn", got.level)
	assert.Equal(t, "HTTP client error", got.message)
	assert.Equal(t, http.StatusNotFound, got.fields["status"])
}

func TestRequestLogger_ServerErrorLogsAtError(t *testing.T) {
	logger := newRecordingLogger()
	serve(t, logger, func(c *gin.Context) {
		_ = c.Error(errors.New("database unreachable"))
		c.Status(http.StatusInternalServerError)
	})

	got := logger.last()
	assert.Equal(t, "error", got.level)
	assert.Equal(t, "HTTP server error", got.message)
	require.Contains(t, got.fields, "error")
	assert.Contains(t, got.fields["error"], "database unreachable")
}

func TestRequestLogger_LogsExactlyOnePerRequest(t *testing.T) {
	logger := newRecordingLogger()
	serve(t, logger, func(c *gin.Context) { c.Status(http.StatusOK) })

	logger.mu.Lock()
	defer logger.mu.Unlock()
	assert.Len(t, *logger.entries, 1)
}
