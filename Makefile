build:
	@go build -o bin/site

deploy: build
	@./bin/site

push-to-docker: docker-buildx-push
	@echo "Pushed multi-arch Docker image to registry."

# Build a Linux binary (useful when building images for Linux hosts)
build-linux:
	@GOOS=linux GOARCH=amd64 go build -o bin/site-linux

# Build and push a single-platform Linux image (amd64)
docker-build-linux: build-linux
	@docker build --platform linux/amd64 -t vandit1604/site:linux-amd64 .
	@docker push vandit1604/site:linux-amd64

# Build and push a multi-arch image using buildx (requires docker buildx set up)
docker-buildx-push: build-linux
	@docker buildx build --platform linux/amd64,linux/arm64 -t vandit1604/site:latest --push .
