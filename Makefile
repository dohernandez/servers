#GOLANGCI_LINT_VERSION := "v1.43.0" # Optional configuration to pinpoint golangci-lint version.

# The head of Makefile determines location of dev-go to include standard targets.
GO ?= go
export GO111MODULE = on

ifneq "$(GOFLAGS)" ""
  $(info GOFLAGS: ${GOFLAGS})
endif

ifneq "$(wildcard ./vendor )" ""
  $(info Using vendor)
  modVendor =  -mod=vendor
  ifeq (,$(findstring -mod,$(GOFLAGS)))
      export GOFLAGS := ${GOFLAGS} ${modVendor}
  endif
  ifneq "$(wildcard ./vendor/github.com/bool64/dev)" ""
  	DEVGO_PATH := ./vendor/github.com/bool64/dev
  endif
  # adding github.com/dohernandez/dev-grpc
  ifneq "$(wildcard ./vendor/github.com/dohernandez/dev-grpc)" ""
  	DEVGRPCGO_PATH := ./vendor/github.com/dohernandez/dev-grpc
  endif
endif

ifeq ($(DEVGO_PATH),)
	DEVGO_PATH := $(shell GO111MODULE=on $(GO) list ${modVendor} -f '{{.Dir}}' -m github.com/bool64/dev)
	ifeq ($(DEVGO_PATH),)
    	$(info Module github.com/bool64/dev not found, downloading.)
    	DEVGO_PATH := $(shell export GO111MODULE=on && $(GO) get github.com/bool64/dev && $(GO) list -f '{{.Dir}}' -m github.com/bool64/dev)
	endif
endif

# defining DEVGRPCGO_PATH
ifeq ($(DEVGRPCGO_PATH),)
	DEVGRPCGO_PATH := $(shell GO111MODULE=on $(GO) list ${modVendor} -f '{{.Dir}}' -m github.com/bool64/dev)
	ifeq ($(DEVGRPCGO_PATH),)
    	$(info Module github.com/dohernandez/dev-grpc not found, downloading.)
    	DEVGRPCGO_PATH := $(shell export GO111MODULE=on && $(GO) get github.com/dohernandez/dev-grpc && $(GO) list -f '{{.Dir}}' -m github.com/dohernandez/dev-grpc)
	endif
endif

SRC_PROTO_PATH = ./testdata/proto
GO_PROTO_PATH = ./testdata
SWAGGER_PATH = ./testdata

-include $(DEVGRPCGO_PATH)/makefiles/protoc.mk

-include $(DEVGO_PATH)/makefiles/main.mk
-include $(DEVGO_PATH)/makefiles/lint.mk
-include $(DEVGO_PATH)/makefiles/test-unit.mk

# Add your custom targets here.

## Run tests
test: test-unit

## Check the commit compile and test the change.
check: lint test

## Generate code from proto file(s)
proto-gen: proto-gen-code-swagger

.PHONY: test check proto-gen