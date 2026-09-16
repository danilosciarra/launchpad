package launchpad

import (
	"github.com/danilosciarra/launchpad/log"

	"github.com/gin-gonic/gin"
	"github.com/swaggo/swag"
	"google.golang.org/grpc"
)

// Option configures an App. Pass options to New.
type Option func(*options)

type options struct {
	logger       log.Logger
	swagger      *swag.Spec
	middleware   map[string][]gin.HandlerFunc
	interceptors []grpc.UnaryServerInterceptor
}

func newOptions() *options {
	return &options{middleware: make(map[string][]gin.HandlerFunc)}
}

// WithLogger overrides the default logger (JSON to stdout) used for
// request logging and server lifecycle messages. Implement log.Logger to
// plug in logrus, zap, slog or any other logging library.
func WithLogger(logger log.Logger) Option {
	return func(o *options) { o.logger = logger }
}

// WithSwagger enables the Swagger UI at GET /swagger/*any, serving spec.
// Typically spec is the generated *swag.Spec from a swaggo/swag docs
// package (`docs.SwaggerInfo`). Its Title, Version, Description and
// BasePath are populated from the App's config.Config automatically.
func WithSwagger(spec *swag.Spec) Option {
	return func(o *options) { o.swagger = spec }
}

// WithMiddleware registers Gin middleware that runs for every route
// registered under the given group (an empty group is the root group "/").
func WithMiddleware(group string, middleware ...gin.HandlerFunc) Option {
	return func(o *options) {
		o.middleware[group] = append(o.middleware[group], middleware...)
	}
}

// WithUnaryInterceptor appends a gRPC unary interceptor, after the built-in
// recovery and logging interceptors. Interceptors run in the order they are
// added. It has no effect if config.Config.GRPC is nil.
func WithUnaryInterceptor(interceptor grpc.UnaryServerInterceptor) Option {
	return func(o *options) { o.interceptors = append(o.interceptors, interceptor) }
}
