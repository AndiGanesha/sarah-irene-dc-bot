SHELL := /bin/bash

# Golang Stuff
GOCMD=go
GORUN=$(GOCMD) run
ARGS=$(filter-out $@,$(MAKECMDGOALS))

# Database stuff
POSTGREQSQL_URL="$(DB_DRIVER)://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=disable&search_path$(DB_SCHEMA)"

build:
	go build -o bin/main main.go

run:
	export GOSUMDB=off
	set -o allexport; source configuration/local.env; set +o allexport && ${GORUN} main.go ${ARGS}

mock: #run with -B
	chmod +x ./mock/script.sh
	./mock/script.sh
	$(GOCMD) generate ./...

lint: 
	golangci-lint run --deadline=1m

test:
	export GOSUMDB=off
	$(GOCMD) test ./...