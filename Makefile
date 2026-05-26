BUILDDIR ?= $(CURDIR)/build
DESTDIR ?= $(CURDIR)/out

NDK_GO_ARCH_MAP_x86 := 386
NDK_GO_ARCH_MAP_x86_64 := amd64
NDK_GO_ARCH_MAP_arm := arm
NDK_GO_ARCH_MAP_arm64 := arm64
NDK_GO_ARCH_MAP_mips := mipsx
NDK_GO_ARCH_MAP_mips64 := mips64x

comma := ,
CLANG_FLAGS := --target=$(TARGET) --sysroot=$(SYSROOT)
export CGO_CFLAGS := $(CLANG_FLAGS) $(subst -mthumb,-marm,$(CFLAGS))
export CGO_LDFLAGS := $(CLANG_FLAGS) $(patsubst -Wl$(comma)--build-id=%,-Wl$(comma)--build-id=none,$(LDFLAGS)) -Wl,-soname=liblocaldns-go.so
export GOARCH := $(NDK_GO_ARCH_MAP_$(ANDROID_ARCH_NAME))
export GOOS := android
export CGO_ENABLED := 1

SYSTEM_GO := $(shell command -v go 2>/dev/null)
GO_VERSION := 1.25.5
GO_PLATFORM := $(shell uname -s | tr '[:upper:]' '[:lower:]')-$(NDK_GO_ARCH_MAP_$(shell uname -m))
GO_TARBALL := go$(GO_VERSION).$(GO_PLATFORM).tar.gz

BUILD_PKG := ./main
BUILD_DEPS := go.mod main/main.go main/jni.c infra/android/bridge.go

default: $(DESTDIR)/liblocaldns-go.so

ifeq ($(SYSTEM_GO),)
$(GRADLE_USER_HOME)/caches/golang/$(GO_TARBALL):
	mkdir -p "$(dir $@)"
	curl -fL --retry 3 -o "$@.tmp" "https://go.dev/dl/$(GO_TARBALL)" && mv "$@.tmp" "$@"

$(BUILDDIR)/go-$(GO_VERSION)/.prepared: $(GRADLE_USER_HOME)/caches/golang/$(GO_TARBALL)
	mkdir -p "$(dir $@)"
	tar -C "$(dir $@)" --strip-components=1 -xzf "$^" && touch "$@"

GO := $(BUILDDIR)/go-$(GO_VERSION)/bin/go
$(DESTDIR)/liblocaldns-go.so: export PATH := $(BUILDDIR)/go-$(GO_VERSION)/bin/:$(PATH)
$(DESTDIR)/liblocaldns-go.so: $(BUILDDIR)/go-$(GO_VERSION)/.prepared $(BUILD_DEPS)
else
GO := $(SYSTEM_GO)
$(DESTDIR)/liblocaldns-go.so: $(BUILD_DEPS)
endif
	$(GO) mod tidy
	$(GO) build -v -trimpath -buildvcs=false -o "$@" -buildmode c-shared $(BUILD_PKG)

.DELETE_ON_ERROR:
