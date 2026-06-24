IMAGE        ?= ghcr.io/jchonig/docker-fetchbox
TAG          ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
GOIMAGE      := golang:1.26-alpine
GOIMAGE_TEST := golang:1.26
SRCDIR       := $(CURDIR)/src

DISTDIR      := $(CURDIR)/dist

.PHONY: all build lint test hooks docker-build image-run-test image-test docker-push release build-darwin-arm64 build-darwin-amd64 clean

all: build

build:
	docker run --rm \
		-v "$(SRCDIR):/src" -w /src \
		$(GOIMAGE) \
		go build -v -o /dev/null ./...

$(DISTDIR):
	mkdir -p $(DISTDIR)

build-darwin-arm64: $(DISTDIR)
	mkdir -p $(DISTDIR)/arm64
	docker run --rm \
		-v "$(SRCDIR):/build" -v "$(DISTDIR)/arm64:/dist" \
		-w /build \
		-e CGO_ENABLED=0 -e GOOS=darwin -e GOARCH=arm64 \
		$(GOIMAGE) \
		go build -ldflags "-X main.version=$(TAG)" -o /dist/fetchbox .
	tar czf $(DISTDIR)/fetchbox-darwin-arm64.tar.gz -C $(DISTDIR)/arm64 fetchbox

build-darwin-amd64: $(DISTDIR)
	mkdir -p $(DISTDIR)/amd64
	docker run --rm \
		-v "$(SRCDIR):/build" -v "$(DISTDIR)/amd64:/dist" \
		-w /build \
		-e CGO_ENABLED=0 -e GOOS=darwin -e GOARCH=amd64 \
		$(GOIMAGE) \
		go build -ldflags "-X main.version=$(TAG)" -o /dist/fetchbox .
	tar czf $(DISTDIR)/fetchbox-darwin-amd64.tar.gz -C $(DISTDIR)/amd64 fetchbox

release: build-darwin-arm64 build-darwin-amd64

lint:
	docker run --rm \
		-v "$(SRCDIR):/src" -w /src \
		$(GOIMAGE) \
		sh -c 'go vet ./... && test -z "$$(gofmt -l .)"'

test:
	docker run --rm \
		-v "$(SRCDIR):/src" -w /src \
		$(GOIMAGE_TEST) \
		go test -v -race -count=1 ./...

hooks:
	git config core.hooksPath .githooks

docker-build:
	docker build -t $(IMAGE):$(TAG) .

# Run the smoke test against a pre-built image (used by CI after docker/build-push-action)
image-run-test:
	docker run --rm \
		--entrypoint /usr/local/bin/fetchbox \
		-v "$(CURDIR)/testdata/fetchbox.yml:/config/fetchbox.yml:ro" \
		$(IMAGE):$(TAG) \
		--config /config/fetchbox.yml

# Build the image locally then smoke-test it
image-test: docker-build image-run-test

docker-push: image-test
	docker push $(IMAGE):$(TAG)
	docker tag $(IMAGE):$(TAG) $(IMAGE):latest
	docker push $(IMAGE):latest

clean:
	rm -f src/fetchbox src/docker-fetchbox
	rm -rf $(DISTDIR)
