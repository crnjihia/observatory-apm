# Contributing to Observatory APM

Thank you for your interest in contributing to **Observatory APM**! We welcome code contributions, issue reports, documentation improvements, and performance benchmarks.

## Development Workflow

1. **Branching Strategy**: Branch off `main` using descriptive branch names (`feat/feature-name`, `fix/issue-description`, `perf/optimization`).
2. **Commit Convention**: We follow [Conventional Commits](https://www.conventionalcommits.org/):
   - `feat(scope): add new capability`
   - `fix(scope): resolve defect`
   - `perf(scope): latency or memory optimization`
   - `docs(scope): documentation update`
   - `test(scope): add or improve test coverage`
3. **Strict Quality Standards**:
   - Go code must pass `go vet ./...` and `golangci-lint run`.
   - TypeScript code must pass `npm run type-check` with strict mode enabled. Zero use of `any` type without prior ADR justification.
   - All database queries must be tested against TimescaleDB hypertables.
   - Third-party integrations (Safaricom Daraja, Africa's Talking) must have deterministic mock adapters activated under `USE_MOCK_ADAPTERS=true`.
   - Never log secrets, auth tokens, or customer PII.
4. **Running Tests**:
   - Run API tests: `cd api && go test -v -race ./...`
   - Run Web tests: `cd web && npm test`
   - Run End-to-End tests: `npx playwright test`
