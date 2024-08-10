build:
	@go build -o site

deploy: build
	@./site

