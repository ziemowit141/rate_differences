.PHONY: dev

SHELL := /bin/bash

dev:
	@bash -c 'set -e; \
	if lsof -ti :8080 >/dev/null 2>&1; then \
		lsof -ti :8080 | xargs kill; \
	fi; \
	go run ./cmd/mt940api --addr :8080 & \
	BACK_PID=$$!; \
	trap "[[ -n \"$$BACK_PID\" ]] && kill $$BACK_PID 2>/dev/null" INT TERM EXIT; \
	cd frontend; \
	npm install; \
	npm run dev'
