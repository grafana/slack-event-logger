.PHONY: build-dev build test push

IMAGE_NAME ?= slack-event-exporter
IMAGE_PREFIX ?= us.gcr.io/kubernetes-dev
IMAGE_TAG ?= 0.0.14

build-dev: 
	go build .

build:
	docker build -t $(IMAGE_PREFIX)/${IMAGE_NAME} .
	docker tag $(IMAGE_PREFIX)/${IMAGE_NAME} $(IMAGE_PREFIX)/${IMAGE_NAME}:$(IMAGE_TAG)

test: build
	go test ./...

push: build test push-image

push-image:
	docker push $(IMAGE_PREFIX)/${IMAGE_NAME}:$(IMAGE_TAG)
	docker push $(IMAGE_PREFIX)/${IMAGE_NAME}:latest
