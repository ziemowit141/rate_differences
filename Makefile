.PHONY: dev

dev:
	@lsof -ti :8080 >/dev/null 2>&1 || go run ./cmd/mt940api --addr :8080 & \
	cd frontend && \
	npm install && \
	npm run dev
