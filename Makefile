BINARY_PATH      := $(CURDIR)/bin
PACKAGE_NAME=opengauss_exporter
BINARY_NAME		= opengauss_exporter
VERSION_PATH 	= pkg/version
V 				= 0
Q 				= $(if $(filter 1,$V),,@)
M 				= $(shell printf "\033[34;1m▶\033[0m")
DOCKER_USERNAME=mogdb

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

BUILD_DATE 		= $(shell date -u '+%Y-%m-%d %I:%M:%S')
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
	@echo "Git Hash:                \"${GIT_COMMIT}\""
	@echo "Git Branch:              \"${GIT_TAG}\""



#################################################
# build
#################################################

.PHONY: build

build: clean fmt bin/${BINARY_NAME};  @ ## Run build

bin/%: cmd/%/main.go; $(info $(M) running build)
	$(GO) build $(GOLDFLAGS) -tags ${BUILD_TAGS} -o $@ $<


.PHONY: goreleaser
goreleaser: ; $(info $(M) cleaning)	@ ## Run goreleaser Build
	goreleaser build --debug --rm-dist


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