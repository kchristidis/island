.PHONY: all dev clean build env-up env-down run

all: clean build env-up run

dev: build run

##### BUILD
build:
	@echo "Build ..."
	@go build
	@echo "Build done"

##### ENV
env-up:
	@echo "Starting environment..."
	@cd fixtures && docker-compose up --force-recreate -d
	@echo "Environment up"

env-down:
	@echo "Stopping environment..."
	@cd fixtures && docker-compose down
	@echo "Environment down"

##### RUN
run:
	@echo "Starting run..."
	@./island -exp=2

##### CLEAN
clean: env-down
	@echo "Cleaning up..."
	@rm -rf /tmp/island* island
	@docker rm -f -v `docker ps -a --no-trunc | grep "example.com" | cut -d ' ' -f 1` 2>/dev/null || true
	@docker rm -f -v `docker ps -a --no-trunc | grep "clark" | cut -d ' ' -f 1` 2>/dev/null || true
	@docker rmi `docker images --no-trunc | grep "example.com" | cut -d ' ' -f 1` 2>/dev/null || true
	@docker rmi `docker images --no-trunc | grep "clark" | cut -d ' ' -f 1` 2>/dev/null || true
	@echo "Clean up done"
