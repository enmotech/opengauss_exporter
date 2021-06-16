BINARY_PATH      := $(CURDIR)/bin
PACKAGE_NAME=opengauss_exporter
BINARY_NAME		= opengauss_exporter
VERSION_PATH 	= pkg/version
V 				= 0
Q 				= $(if $(filter 1,$V),,@)
M 				= $(shell printf "\033[34;1m▶\033[0m")
DOCKER_USERNAME=mogdb
PKGS     		= $(or $(PKG),$(shell $(GO) list ./...|grep -v obs|grep -v oraxml))
TESTPKGS 		= $(shell $(GO) list -f '{{ if or .TestGoFiles .XTestGoFiles }}{{ .ImportPath }}{{ end }}' $(PKGS))

### ################################################
### tools
### ################################################
GO = go
GOLINT = golangci-lint
GOCOVMERGE = gocovmerge
GOCOV = gocov
GOCOVXML = gocov-xml
GO2XUNIT = go2xunit
CHGLOG = git-chglog

## global env
export GOPROXY=https://goproxy.cn

BUILD_DATE 		= $(shell date '+%Y-%m-%d %H:%M:%S')
GIT_COMMIT 		= $(shell git rev-parse HEAD)
GIT_SHA    		= $(shell git rev-parse --short HEAD)
GIT_BRANCH    	= $(shell git describe --tags --always 2>/dev/null)
GIT_DIRTY  		= $(shell test -n "`git status --porcelain`" && echo "dirty" || echo "clean")
GIT_TAG    		= $(shell git describe --tags --abbrev=0 --exact-match 2>/dev/null)
BASE_VERSION 	= $(shell grep 'var version' ${VERSION_PATH}/version.go | sed -E 's/.*"(.+)"$$/v\1/')


# 判断是否指定版本
ifdef VERSION
	BINARY_VERSION = ${VERSION}
endif
# 没有则用git tag
BINARY_VERSION ?= ${GIT_TAG}

# 如果没有指定版本和git tag 没有信息.则用代码内嵌版本
ifeq ($(BINARY_VERSION),)
	BINARY_VERSION = ${BASE_VERSION}
endif
# docker 版本标签
DOCKER_IMAGE_VERSION=${BINARY_VERSION}
# 发布文件版本
RELEASE_FILE_VERSION=${BINARY_VERSION}
# 测试版本或者发布版本
VERSION_METADATA = beta

# Clear the "beta" string in BuildMetadata
ifneq ($(GIT_TAG),)
	VERSION_METADATA =
endif

ifneq ($(VERSION_METADATA),)
	DOCKER_IMAGE_VERSION:=${DOCKER_IMAGE_VERSION}-${VERSION_METADATA}
	RELEASE_FILE_VERSION:=${RELEASE_FILE_VERSION}-${VERSION_METADATA}
endif


ifdef TAGS
	BUILD_TAGS = ${TAGS}
endif
BUILD_TAGS ?= codes

LDFLAGS += -X "${PACKAGE_NAME}/$(VERSION_PATH).version=${BINARY_VERSION}"
LDFLAGS += -X "${PACKAGE_NAME}/$(VERSION_PATH).metadata=${VERSION_METADATA}"
LDFLAGS += -X "${PACKAGE_NAME}/$(VERSION_PATH).buildTimestamp=${BUILD_DATE}"
LDFLAGS += -X "${PACKAGE_NAME}/$(VERSION_PATH).gitCommit=${GIT_COMMIT}"
LDFLAGS += -X "${PACKAGE_NAME}/$(VERSION_PATH).gitTagInfo=${GIT_BRANCH}"
LDFLAGS += $(EXT_LDFLAGS)
GOLDFLAGS = -ldflags '$(LDFLAGS)'

ifeq ("$(WITH_RACE)", "1")
	GOLDFLAGS += -race
endif

.PHONY: fmt
fmt: ; $(info $(M) running gofmt) @ ## Run go fmt on all source files
	$Q go fmt ./...

.PHONY: lint
lint: fmt ; $(info $(M) running golangci-lint) @ ## Run golangci-lint
	$Q golangci-lint run


.PHONY: info
info:
	@echo "Version:                 \"${VERSION}\""
	@echo "Binary Version:          \"${BINARY_VERSION}\""
	@echo "Release File Version:    \"${RELEASE_FILE_VERSION}\""
	@echo "Docker Image Version:    \"${DOCKER_IMAGE_VERSION}\""
	@echo "Release Or Beta:         \"${VERSION_METADATA}\""
	@echo "Build Timestamp:         \"${BUILD_DATE}\""
	@echo "Git Commit Hash:         \"${GIT_COMMIT}\""
	@echo "Git Tag:                 \"${GIT_TAG}\""



#################################################
# build
#################################################

.PHONY: build

build: clean fmt bin/${BINARY_NAME};  @ ## Run build

bin/%: cmd/%/main.go; $(info $(M) running build)
	$(GO) build $(GOLDFLAGS) -tags ${BUILD_TAGS} -o $@ $<


.PHONY: goreleaser_build
goreleaser_build: ; $(info $(M) cleaning)	@ ## Run goreleaser Build
	goreleaser build --debug --snapshot --rm-dist --parallelism 2

.PHONY: goreleaser_beta
goreleaser_beta: ; $(info $(M) cleaning)	@ ## Run goreleaser Build
	goreleaser --debug --snapshot --rm-dist --parallelism 2

.PHONY: goreleaser_releaser
goreleaser_releaser: ; $(info $(M) cleaning)	@ ## Run goreleaser Build
	goreleaser --debug --rm-dist --parallelism 2
#################################################
# Release
#################################################

files = linux/amd64 linux/arm64 windows/amd64 darwin/amd64
xgo:; $(info) @ ## Run Xgo Build Release
	xgo -x --targets="${files}" ${GOLDFLAGS} -dest bin -out ${BINARY_NAME}-${RELEASE_FILE_VERSION} -pkg /cmd/opengauss_exporter .

#################################################
# Cleaning
#################################################

.PHONY: clean
clean: ; $(info $(M) cleaning)	@ ## Run cleanup everything
	@rm -rf bin
	@rm -rf data*
	@rm -rf test/tests.* test/coverage.*
	@rm -rf build


docker: clean ; $(info) @ ## Run Docker Build Release
	docker build -t ${DOCKER_USERNAME}/${BINARY_NAME}:${DOCKER_IMAGE_VERSION} .
ifeq ($(VERSION_METADATA),)
	docker tag ${DOCKER_USERNAME}/${BINARY_NAME}:${DOCKER_IMAGE_VERSION} ${DOCKER_USERNAME}/${BINARY_NAME}:latest
endif

docker-push: docker ; $(info) @ ## Run Docker Build Release And Push Docker Hub
	docker push ${DOCKER_USERNAME}/${BINARY_NAME}:${DOCKER_IMAGE_VERSION}
ifeq ($(VERSION_METADATA),)
	docker push ${DOCKER_USERNAME}/${BINARY_NAME}:latest
endif

# ################################################
# Tests
# ################################################

TEST_TARGETS := test-default test-bench test-short test-verbose test-race
.PHONY: $(TEST_TARGETS) test-xml check test tests
test-bench:   ARGS=-run=__absolutelynothing__ -bench=. ## Run benchmarks
test-short:   ARGS=-short        ## Run only short tests
test-verbose: ARGS=-v            ## Run tests in verbose mode with coverage reporting
test-race:    ARGS=-race         ## Run tests with race detector
$(TEST_TARGETS): NAME=$(MAKECMDGOALS:test-%=%)
$(TEST_TARGETS): test
check test tests: fmt ; $(info $(M) running $(NAME:%=% )tests…) @ ## Run tests
	$Q $(GO) test $(ARGS) $(TESTPKGS)

test-xml: fmt | $(GO2XUNIT) ; $(info $(M) running $(NAME:%=% )tests…) @ ## Run tests with xUnit output
	$Q mkdir -p test
	$Q 2>&1 $(GO) test -v $(TESTPKGS) | tee test/tests.output
	$(GO2XUNIT) -fail -input test/tests.output -output test/tests.xml

test-json: fmt ; $(info $(M) running $(NAME:%=% )tests…) @ ## Run tests with xUnit output
	$Q mkdir -p test
	$Q 2>&1 $(GO) test -json -timeout 20s -v $(TESTPKGS) > test/test-report.json

COVERAGE_MODE = atomic
COVERAGE_PROFILE = test/profile.out
COVERAGE_XML = $(COVERAGE_DIR)/coverage.xml
COVERAGE_HTML = $(COVERAGE_DIR)/index.html
.PHONY: test-coverage
test-coverage: COVERAGE_DIR := $(CURDIR)/test/coverage.$(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

test-coverage: fmt ; $(info $(M) running coverage tests…) @ ## Run coverage tests
	$Q mkdir -p $(COVERAGE_DIR)/coverage
	$Q for pkg in $(TESTPKGS); do \
		$(GO) test \
			-coverpkg=$$($(GO) list -f '{{ join .Deps "\n" }}' $$pkg | \
					grep '^$(PACKAGE)/' | \
					tr '\n' ',')$$pkg \
			-covermode=$(COVERAGE_MODE) \
			-coverprofile="$(COVERAGE_DIR)/coverage/`echo $$pkg | tr "/" "-"`.cover" $$pkg ;\
	 done
	$Q $(GOCOVMERGE) $(COVERAGE_DIR)/coverage/*.cover > $(COVERAGE_PROFILE)
	$Q $(GO) tool cover -html=$(COVERAGE_PROFILE) -o $(COVERAGE_HTML)
	$Q $(GOCOV) convert $(COVERAGE_PROFILE) | $(GOCOVXML) > $(COVERAGE_XML)