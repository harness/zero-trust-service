ROOT_DIR := $(shell git rev-parse --show-toplevel)

ZTS_IMAGE    ?= zero-trust-service
ZTS_TAG      ?= latest
ZTS_REGISTRY ?=
GOOS         ?=
GOARCH       ?=

ifdef ZTS_REGISTRY
  ZTS_FULL_IMAGE = $(ZTS_REGISTRY)/$(ZTS_IMAGE):$(ZTS_TAG)
else
  ZTS_FULL_IMAGE = $(ZTS_IMAGE):$(ZTS_TAG)
endif

run:
	cd $(ROOT_DIR) && go run examples/zts/main.go -config examples/zts/config.yaml

# docker-build: builds from source inside Docker with Go cross-compilation.
# Use GOOS/GOARCH to pick a platform-specific Dockerfile, else use the default.
docker-build:
ifdef GOOS
	cd $(ROOT_DIR) && docker build -t $(ZTS_FULL_IMAGE) -f examples/zts/docker/Dockerfile.$(GOOS).$(GOARCH) .
else
	cd $(ROOT_DIR) && docker build -t $(ZTS_FULL_IMAGE) -f examples/zts/docker/Dockerfile .
endif

docker-run:
	docker run --rm -p 4210:4210 -p 8898:8898 \
		-v $(ROOT_DIR)/config.yaml:/app/config.yaml \
		$(ZTS_FULL_IMAGE)

docker-push:
	docker push $(ZTS_FULL_IMAGE)

docker-build-push: docker-build docker-push
