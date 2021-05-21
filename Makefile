.PHONY: deps dev build install image release clean

CGO_ENABLED=0
VERSION=$(shell git describe --abbrev=0 --tags 2>/dev/null || echo "$VERSION")
COMMIT=$(shell git rev-parse --short HEAD || echo "$COMMIT")
GOCMD=go

all: build

deps:

dev : DEBUG=1
dev : build
	@./cinit

cli:

build:
	@$(GOCMD) build -tags "netgo static_build" -installsuffix netgo \
		-ldflags "-w \
		-X .Version=$(VERSION) \
		-X .Commit=$(COMMIT)" \
		.

install: build
	@$(GOCMD) install .

ifeq ($(PUBLISH), 1)
image:
	@docker build --build-arg VERSION="$(VERSION)" --build-arg COMMIT="$(COMMIT)" -t prologic/cinit .
	@docker push prologic/cinit
else
image:
	@docker build --build-arg VERSION="$(VERSION)" --build-arg COMMIT="$(COMMIT)" -t prologic/cinit .
endif

release:
	@./tools/release.sh

clean:
	@git clean -f -d -X
