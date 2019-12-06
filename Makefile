# define standard colors
SHELL=/bin/bash

BLACK        := $(shell tput -Txterm setaf 0)
RED          := $(shell tput -Txterm setaf 1)
GREEN        := $(shell tput -Txterm setaf 2)
YELLOW       := $(shell tput -Txterm setaf 3)
LIGHTPURPLE  := $(shell tput -Txterm setaf 4)
PURPLE       := $(shell tput -Txterm setaf 5)
BLUE         := $(shell tput -Txterm setaf 6)
WHITE        := $(shell tput -Txterm setaf 7)

RESET := $(shell tput -Txterm sgr0)

.DEFAULT_GOAL := help
.PHONY: start

MAKEFLAGS += --silent
PROJECTNAME=$(shell basename "$(PWD)")
TESTTIMEOUT=-timeout 30s
BRANCHENAME=$(shell git branch --show-current)
VENDOR=-mod=vendor

%:: 
	make
	@echo "$(SOMECOLOR) > type one of the targets above$(NOCOLOR)"
	@echo

colors: ## show all the colors
	@echo "${BLACK}BLACK${RESET}"
	@echo "${RED}RED${RESET}"
	@echo "${GREEN}GREEN${RESET}"
	@echo "${YELLOW}YELLOW${RESET}"
	@echo "${LIGHTPURPLE}LIGHTPURPLE${RESET}"
	@echo "${PURPLE}PURPLE${RESET}"
	@echo "${BLUE}BLUE${RESET}"
	@echo "${WHITE}WHITE${RESET}"

## install: installeer alle dependencies
install:
	@tput reset
	@echo "$(BLUE) > Installing dependencies $(LIGHTPURPLE)$(PROJECTNAME) $(YELLOW)$(BRANCHENAME)$(RESET)"
	@go get -u golang.org/x/tools/cmd/goimports
	@go get -u github.com/rakyll/gotest
	@echo "$(GREEN) > ready: $(LIGHTPURPLE)$(PROJECTNAME)$(RESET) $(YELLOW)$(BRANCHENAME)$(RESET)"

## start: start het programma
start:
	@tput reset
	@echo "$(BLUE) > Starting generator $(LIGHTPURPLE)$(PROJECTNAME) $(YELLOW)$(BRANCHENAME)$(RESET)"
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go run main.go ./... -v loop
	@echo "$(GREEN) > ready: $(LIGHTPURPLE)$(PROJECTNAME)$(RESET) $(YELLOW)$(BRANCHENAME)$(RESET)"

## build: compileert de applicatie
build:
	@tput reset
	@echo "$(BLUE) > Building $(LIGHTPURPLE)$(PROJECTNAME) $(YELLOW)$(BRANCHENAME)$(RESET)"
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go build -o gotest *.go
	@cp gotest ~/go/bin/gotest
	@echo "$(GREEN) > ready: $(LIGHTPURPLE)$(PROJECTNAME)$(RESET) $(YELLOW)$(BRANCHENAME)$(RESET)"

## update: haalt missende dependencies op
update:
	@tput reset
	@echo "$(BLUE) > Updating mod-file $(LIGHTPURPLE)$(PROJECTNAME) $(YELLOW)$(BRANCHENAME)$(RESET)"
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go mod vendor
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go mod tidy
	@echo "$(GREEN) > ready: $(LIGHTPURPLE)$(PROJECTNAME)$(RESET) $(YELLOW)$(BRANCHENAME)$(RESET)"

## check: check de applicatie met golangci-lint
check:
	@tput reset
	@echo "$(BLUE) > Checking with golangci-lint $(LIGHTPURPLE)$(PROJECTNAME) $(YELLOW)$(BRANCHENAME)$(RESET)"
	@golangci-lint run
	@echo "$(GREEN) > ready: $(LIGHTPURPLE)$(PROJECTNAME)$(RESET) $(YELLOW)$(BRANCHENAME)$(RESET)"

## check2: check de applicatie met staticcheck
check2:
	@tput reset
	@echo "$(BLUE) > Checking with staticcheck $(LIGHTPURPLE)$(PROJECTNAME) $(YELLOW)$(BRANCHENAME)$(RESET)"
	@staticcheck -checks all ./...
	@echo "$(GREEN) > ready: $(LIGHTPURPLE)$(PROJECTNAME)$(RESET) $(YELLOW)$(BRANCHENAME)$(RESET)"

## check3: check de applicatie met revive
check3:
	@tput reset
	@echo "$(BLUE) > Checking with revive $(LIGHTPURPLE)$(PROJECTNAME) $(YELLOW)$(BRANCHENAME)$(RESET)"
	@revive -formatter friendly -exclude ./vendor/... ./...
	@echo "$(GREEN) > ready: $(LIGHTPURPLE)$(PROJECTNAME)$(RESET) $(YELLOW)$(BRANCHENAME)$(RESET)"

## test: test de applicatie (short)
test-s:
	@tput reset
	@echo "$(BLUE) > Testing (short) $(LIGHTPURPLE)$(PROJECTNAME) $(YELLOW)$(BRANCHENAME)$(RESET)"
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) gotest $(VENDOR) -count=1 $(TESTTIMEOUT) ./... -short
	@echo "$(GREEN) > ready: $(LIGHTPURPLE)$(PROJECTNAME)$(RESET) $(YELLOW)$(BRANCHENAME)$(RESET)"

## test: test de applicatie (full)
test:
	@tput reset
	@echo "$(BLUE) > Testing $(LIGHTPURPLE)$(PROJECTNAME) $(YELLOW)$(BRANCHENAME)$(RESET)"
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) gotest $(VENDOR) -count=1 $(TESTTIMEOUT) ./...
	@echo "$(GREEN) > ready: $(LIGHTPURPLE)$(PROJECTNAME) $(YELLOW)$(BRANCHENAME)$(RESET)"

## docker: builds and runs docker
docker:
	@tput reset
	@echo "$(BLUE) > Creating Docker.. $(LIGHTPURPLE)$(PROJECTNAME) $(YELLOW)$(BRANCHENAME)$(RESET)"
	@./scripts/run_docker.sh
	@echo "$(GREEN) > ready: $(LIGHTPURPLE)$(PROJECTNAME) $(YELLOW)$(BRANCHENAME)$(RESET)"	

## remotedebug: debug main app code remotely in docker
remotedebug:
	@tput reset
	@echo "$(BLUE) > Starting debug.. $(LIGHTPURPLE)$(PROJECTNAME) $(YELLOW)$(BRANCHENAME)$(RESET)"
	@dlv debug -l 0.0.0.0:2345 --headless=true --api-version=1 --log
	@echo "$(GREEN) > ready: $(LIGHTPURPLE)$(PROJECTNAME) $(YELLOW)$(BRANCHENAME)$(RESET)"	

## test-c: test de applicatie met codecoverage
test-c:
	@make migrate
	@tput reset
	@echo "$(BLUE) > Testing with code codecoverage $(LIGHTPURPLE)$(PROJECTNAME) $(YELLOW)$(BRANCHENAME)$(RESET)"
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) gotest $(VENDOR) $(TESTTIMEOUT) -coverprofile coverage.out ./...
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go tool cover -html coverage.out
	@echo "$(GREEN) > ready: $(LIGHTPURPLE)$(PROJECTNAME)$(RESET) $(YELLOW)$(BRANCHENAME)$(RESET)"

## test-v: test de applicatie verbose
test-v:
	@make migrate
	@tput reset
	@echo "$(BLUE) > Testing verbose $(LIGHTPURPLE)$(PROJECTNAME) $(YELLOW)$(BRANCHENAME)$(RESET)"
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) gotest $(VENDOR) -v -count=1 $(TESTTIMEOUT) ./...
	@echo "$(GREEN) > ready: $(LIGHTPURPLE)$(PROJECTNAME) $(YELLOW)$(BRANCHENAME)$(RESET)"

## format: runs gofmt
format:
	@tput reset
	@echo "$(BLUE) > format $(LIGHTPURPLE)$(PROJECTNAME) $(YELLOW)$(BRANCHENAME)$(RESET)"
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) gofmt -s -w .
	@echo "$(GREEN) > ready: $(LIGHTPURPLE)$(PROJECTNAME)$(RESET) $(YELLOW)$(BRANCHENAME)$(RESET)"

## modd: start modd
modd:
	@tput reset
	@echo "$(BLUE) > Starting MODD $(LIGHTPURPLE)$(PROJECTNAME) $(YELLOW)$(BRANCHENAME)$(RESET)"
	@modd
	@echo "$(GREEN) > ready: $(LIGHTPURPLE)$(PROJECTNAME)$(RESET) $(YELLOW)$(BRANCHENAME)$(RESET)"

makefile: help
help: Makefile

	@tput reset
	@echo "$(RED) > Choose a make command from the following:$(RESET)"
	@echo
	@sed -n 's/^##//p' $< | column -t -s ':' |  sed -e 's/^/ /'
	@echo
	@echo "$(GREEN) > ready: $(LIGHTPURPLE)$(PROJECTNAME)$(RESET) $(YELLOW)$(BRANCHENAME)$(RESET)"
