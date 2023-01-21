VERSION 			?= 0.1.0
BINDIR      	:= $(CURDIR)/bin
BINNAME     	?= gke-auth-plugin

GOPATH        = $(shell go env GOPATH)
GOIMPORTS     = $(GOPATH)/bin/goimports

# go option
PKG        := ./...
TAGS       :=
TESTS      := .
TESTFLAGS  :=
LDFLAGS    := -w -s
GOFLAGS    :=
SRC        := $(shell find . -type f -name '*.go' -print)

GIT_COMMIT = $(shell git rev-parse HEAD)
GIT_SHA    = $(shell git rev-parse --short HEAD)
GIT_TAG    = $(shell git describe --tags --abbrev=0 --exact-match 2>/dev/null)
GIT_DIRTY  = $(shell test -n "`git status --porcelain`" && echo "dirty" || echo "clean")


ifdef VERSION
	BINARY_VERSION = $(VERSION)
endif
BINARY_VERSION ?= ${GIT_TAG}

# Only set Version if building a tag or VERSION is set
ifneq ($(BINARY_VERSION),)
	LDFLAGS += -X github.com/traviswt/gke-auth-plugin/pkg/conf.Version=${BINARY_VERSION}
endif

VERSION_METADATA = unreleased
# Clear the "unreleased" string in BuildMetadata
ifneq ($(GIT_TAG),)
	VERSION_METADATA =
endif

LDFLAGS += -X github.com/traviswt/gke-auth-plugin/pkg/conf.GitCommit=${GIT_COMMIT}

.PHONY: all
all: build

# ------------------------------------------------------------------------------
#  build

build: clean tidy fmt vet test-unit compile

tidy:
	@echo
	@echo "=== tidying ==="
	go mod tidy

.PHONY: compile
compile: $(BINDIR)/$(BINNAME)

$(BINDIR)/$(BINNAME): $(SRC)
	@echo
	@echo "=== running compile ==="
	GO111MODULE=on CGO_ENABLED=0 go build $(GOFLAGS) -tags '$(TAGS)' -ldflags '$(LDFLAGS)' -o $(BINDIR)/$(BINNAME) .

# Run go fmt against code
fmt:
	@echo
	@echo "=== fmt ==="
	go fmt ./...

# Run go vet against code
vet:
	@echo
	@echo "=== vet ==="
	go vet ./...

# Build the docker image
docker-build:
	@echo
	@echo "=== docker build ==="
	docker build -f ./Dockerfile . -t ${IMG} --build-arg VERSION --build-arg SSH_PRIVATE_KEY
#    --progress plain

# Push the docker image
docker-push: docker-build
	@echo
	@echo "=== docker push ==="
	docker push ${IMG}

# ------------------------------------------------------------------------------
#  test

.PHONY: test-unit
test-unit:
	@echo
	@echo "=== running unit tests ==="
	GO111MODULE=on go test $(GOFLAGS) -run $(TESTS) $(PKG) $(TESTFLAGS)


# ------------------------------------------------------------------------------
#  dependencies

$(GOIMPORTS):
	(cd /; GO111MODULE=on go get -u golang.org/x/tools/cmd/goimports)


.PHONY: clean
clean:
	@echo
	@echo "=== cleaning ==="
	rm -rf $(BINDIR)

.PHONY: info
info:
	 @echo "Version:           ${VERSION}"
	 @echo "Git Tag:           ${GIT_TAG}"
	 @echo "Git Commit:        ${GIT_COMMIT}"
	 @echo "Git Tree State:    ${GIT_DIRTY}"


