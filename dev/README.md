# Development & Testing Resources (`dev/`)

This directory groups local development-only, debugging, testing, and scratchpad resources.
**All contents of this directory (except this README) are strictly ignored by version control (`.gitignore`).**

Future end users and production consumers of Antigravity Swiss Knife do not require these resources to run, build, or package the application.

## Directory Structure

- `dev/scratch/`: Temporary scratchpad scripts, exploratory Go/Node scripts, ad-hoc API callers, and visual capture comparisons.
- `dev/testing/`: Legacy test harnesses, adversarial tokens/geometry test scripts, stress test results, fixtures, and local test runners.
  - Note: Official automated test suites for CI/CD reside idiomatic to the source tree in `pkg/...` (`go test ./pkg/...`) and `frontend/src/...` (`npm test --prefix frontend`).
- `dev/milestones/`: Milestone gate records, adversarial audit logs, test infrastructure checklists (`GATE_STATUS.md`, `TEST_INFRA.md`, `TEST_READY.md`), and local developer notes.
- `dev/tools/`: Local development tools, profiling dumps, and developer convenience helpers.
