# Changelog

All notable changes to this project will be documented in this file.

The format is based on Keep a Changelog and this project aims to follow Semantic Versioning.

## [Unreleased]

### Added
- Core HTTP app, routing, middleware pipeline, request context helpers
- Request parsing/binding (JSON, forms, multipart)
- Response helpers (JSON, HTML, redirect, file/download)
- Centralized error handling (`OnError`, `OnStatus`, typed HTTP errors)
- Templates and static file serving
- Config loading (defaults, JSON, env)
- Blueprints/modules
- CLI commands (`new`, `run`, `routes`, `doctor`)
- Testing toolkit package
- Observability + security middleware
- ORM integration with GORM and custom dialector registry
- Official examples (`hello`, `rest-api`, `web-app`)
- Documentation hub and API reference
- CI workflow (tests, vet, formatting, tidy check)

## [0.1.0] - 2026-02-22

### Added
- Initial public pre-1.0 framework foundation and documentation set
