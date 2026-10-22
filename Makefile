VERSION ?= dev

.PHONY: build test vet docker-test docker demo release
build:
	go build -trimpath -ldflags "-X main.version=$(VERSION)" -o dist/reqdiff ./cmd/reqdiff

test:
	go test -race ./...

vet:
	go vet ./...

docker-test:
	docker build --target test -t reqdiff:test .

docker:
	docker build --build-arg VERSION=$(VERSION) -t reqdiff:local .

demo:
	python3 scripts/demo.py

release:
	./scripts/build-release.sh $(VERSION)
