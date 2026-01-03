.PHONY: dev wails-build wails-rebuild

WAILS ?= ~/go/bin/wails

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

wails-build:
	@cd wails/RateDifferences && \
	bash ../RateDifferences/scripts/build_extractor.sh && \
	$(WAILS) build -platform darwin/amd64 && \
	if [ -f build/bin/RateDifferences.app/Contents/Resources/extract ]; then true; \
	else cp resources/extract build/bin/RateDifferences.app/Contents/Resources/extract; fi

wails-rebuild:
	@rm -rf wails/RateDifferences/build wails/RateDifferences/frontend/dist && \
	cd wails/RateDifferences && \
	bash ../RateDifferences/scripts/build_extractor.sh && \
	$(WAILS) build -platform darwin/amd64 && \
	if [ -f build/bin/RateDifferences.app/Contents/Resources/extract ]; then true; \
	else cp resources/extract build/bin/RateDifferences.app/Contents/Resources/extract; fi
