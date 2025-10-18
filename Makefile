build:
	@go build -o bin/site

deploy: build
	@./bin/site

push-to-docker:
	@docker build -t vandit1604/site:latest .
	@docker push vandit1604/site:latest
