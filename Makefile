build:
	@go build -o bin/site

deploy: build
	@./bin/site

