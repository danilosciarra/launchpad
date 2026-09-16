# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - Unreleased

### Changed

This release is a full rewrite of the internal bootstrap SDK into an
open-source-ready library. The `src/` layout, global registries and
company-specific packages are gone; see `README.md` for the current API and
the migration notes below for what changed and why.

- Repository reorganized to standard Go project layout: `src/` removed,
  public packages (`config`, `httpserver`, `grpcserver`, `dapr`, `log`) now
  live at the repository root, everything else moved under `internal/`.
- Public API redesigned around `launchpad.New(cfg, ...Option)` and the
  functional options pattern (`WithLogger`, `WithSwagger`, `WithMiddleware`,
  `WithUnaryInterceptor`) instead of package-level global registries.
- `configuration.Base` replaced by `config.Config`; validation now uses
  `go-playground/validator` instead of an internal dependency.
- Logging is now dependency-injected via the `log.Logger` interface instead
  of a package-level global (`logging.Log`).
- Dapr integration reduced to sidecar bootstrap only (`dapr.New`, `Client.Raw`
  for full SDK access); pub/sub subscription handling, job scheduling and
  cloud-event/identity propagation were dropped from the core library (see
  Removed).

### Removed

The following domains were out of scope for a general-purpose bootstrap SDK
and were removed rather than open-sourced. They are candidates for separate,
optional companion modules if/when needed:

- JWT authentication and identity propagation (`jwt`, `identity`).
- Redis/MongoDB/Postgres storage helpers (`store`).
- Cron-based job scheduling and the Dapr Jobs API wrapper (`jobs`,
  `orchestrator/scheduler`).
- Dapr pub/sub subscription registration and the Dapr AppCallback gRPC
  service (`subscriptions`, `orchestrator/publisher`, `dapr/grpc`).
- The generic outbound HTTP request builder and `ApiResponse` envelope
  (`http/request`, `http/models`), which existed only to support the
  removed Dapr service-invocation helper.
- Manual gRPC method/stream registration fallback (`RegisterMethod`,
  `RegisterStream`); use `RegisterGRPCService` with protoc-generated
  `ServiceDesc`s instead.

### Added

- `README.md`, `LICENSE` (MIT), `CHANGELOG.md`.
- `examples/minimal-http`, `examples/http-grpc`, `examples/with-dapr`
  runnable example programs.
- `.github/workflows/ci.yml` (build, vet, lint, test) and `.golangci.yml`.
- GoDoc package documentation on every exported package.
