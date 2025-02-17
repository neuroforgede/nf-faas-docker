TAG?=latest-dev
.PHONY: build
build:
	docker build --build-arg http_proxy="${http_proxy}" --build-arg https_proxy="${https_proxy}" -t neuroforgede/nf-faas-docker:$(TAG) .

.PHONY: test-unit
test-unit:
	go test -v $(go list ./... | grep -v /vendor/) -cover


.PHONY: start-dev
start-dev:
	cd contrib && ./dev.sh

.PHONY: stop-dev
stop-dev:
	docker stack rm func

.PHONY: build-armhf
build-armhf:
	docker build --build-arg http_proxy="${http_proxy}" --build-arg https_proxy="${https_proxy}" -t neuroforgede/nf-faas-docker:$(TAG)-armhf . -f Dockerfile.armhf

.PHONY: push
push:
	docker push neuroforgede/nf-faas-docker:$(TAG)

.PHONY: all
all: build

.PHONY: ci-armhf-build
ci-armhf-build:
	docker build --build-arg http_proxy="${http_proxy}" --build-arg https_proxy="${https_proxy}" -t neuroforgede/nf-faas-docker:$(TAG)-armhf . -f Dockerfile.armhf

.PHONY: ci-armhf-push
ci-armhf-push:
	docker push neuroforgede/nf-faas-docker:$(TAG)-armhf

.PHONY: ci-arm64-build
ci-arm64-build:
	docker build --build-arg http_proxy="${http_proxy}" --build-arg https_proxy="${https_proxy}" -t neuroforgede/nf-faas-docker:$(TAG)-arm64 . -f Dockerfile.arm64

.PHONY: ci-arm64-push
ci-arm64-push:
	docker push neuroforgede/nf-faas-docker:$(TAG)-arm64
