# default makefile functionality
VENDOR=-mod=vendor
EXTRABUILD=Y
MIGRATE=
EXECUTABLE=gotest
SPECIALSTART=Y
include ../../Makefile_standard.mk

# extra build step below
extra_build:
	#@echo "$(LIGHTBLUE) > Building again $(BLUE)$(PROJECTNAME) $(YELLOW)$(BRANCHENAME)$(RESET)"
	#@echo "go build -o $(EXECUTABLE) ."
	#@GOPATH=$(GOPATH) GOBIN=$(GOBIN) GOOS=darwin GOARCH=arm64 go build -o $(EXECUTABLE) .

	@echo
	@echo "$(WHITE)> Remove from bin folder$(RESET)"
	@rm -f ~/go/bin/${EXECUTABLE}

	@echo
	@echo "$(WHITE)> Copy to standard bin folder$(RESET)"
	@cp ${EXECUTABLE} ~/go/bin/${EXECUTABLE}
	@goreleaser release --snapshot --rm-dist

# special start step below (demo)
special_start:
	@GOPATH=$(GOPATH) GOBIN=$(GOBIN) go run . $(VENDOR) -count=1 $(TESTTIMEOUT) ./fakeproject/...
