# Build
FROM golang:alpine AS build

RUN apk add --no-cache -U build-base git make

RUN mkdir -p /src

WORKDIR /src

# Copy Makefile
COPY Makefile ./

# Copy go.mod and go.sum and install and cache dependencies
COPY go.mod .
COPY go.sum .

# Install deps
RUN make deps
RUN go mod download

# Copy sources
COPY *.go ./

# Version/Commit (there there is no .git in Docker build context)
# NOTE: This is fairly low down in the Dockerfile instructions so
#       we don't break the Docker build cache just be changing
#       unrelated files that actually haven't changed but caused the
#       COMMIT value to change.
ARG VERSION="0.0.0"
ARG COMMIT="HEAD"

# Build binary
RUN make VERSION=$VERSION COMMIT=$COMMIT

# Runtime
FROM alpine:latest

RUN apk --no-cache -U add ca-certificates tzdata

WORKDIR /

# force cgo resolver
ENV GODEBUG=netdns=cgo

COPY --from=build /src/cinit /cinit

ENTRYPOINT ["/cinit"]
CMD ["/bin/sh"]
