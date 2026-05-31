# Operational backend (Node.js / TypeScript).
#
# Thin wrappers over the npm scripts so the repo's `make -C ... <target>`
# muscle memory keeps working across the Go → Node rewrite. CI calls the npm
# scripts directly; these targets are for local convenience.

.PHONY: install build run dev typecheck lint test test-coverage clean

install:
	npm ci

build:
	npm run build

run: build
	npm start

dev:
	npm run dev

typecheck:
	npm run typecheck

lint:
	npm run lint

test:
	npm test

# Runs the suite with coverage. The 80% business-coverage gate is enforced by
# the thresholds in vitest.config.ts (which exclude the entry point and the
# thin runtime-only wrappers), mirroring the previous Go business-coverage rule.
test-coverage:
	npm run test:ci

clean:
	rm -rf dist coverage node_modules
