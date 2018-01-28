# VARIABLES
PACKAGE="github.com/slushie/kubist-agent"
BINARY_NAME="kubist-agent"
DOCKER_REPO="slushie/kubist-agent"

default: usage

clean: ## Trash binary files
	@echo "--> cleaning..."
	@go clean || (echo "Unable to clean project" && exit 1)
	@rm -rf $(GOPATH)/bin/$(BINARY_NAME) 2> /dev/null
	@rm -rf build/
	@echo "Clean OK"

test: ## Run all tests
	@echo "--> testing..."
	@go test -v $(PACKAGE)/...

glide: ## Fetch dependencies via glide up
	@echo "--> fetching dependencies..."
	@glide install

install: clean ## Compile sources and build binary
	@echo "--> installing..."
	@go install $(PACKAGE) || (echo "Compilation error" && exit 1)
	@echo "Install OK"

run: install ## Run your application
	@echo "--> running application..."
	@$(GOPATH)/bin/$(BINARY_NAME)

linux: build/linux/$(BINARY_NAME) ## Build cross-compiled binaries for Linux

build/%/$(BINARY_NAME):
	@echo "--> building for $*..."
	@GOOS=$* go build -o $@
	@echo "Build OK"

docker: glide linux _docker ## Build a Docker image
_docker:
	@echo "--> installing for docker"
	@docker build -t $(BINARY_NAME) .

docker-publish: ## Publish Docker image
	@echo "--> publishing to $(DOCKER_REPO)..."
	@docker tag $(BINARY_NAME) $(DOCKER_REPO)
	@docker push $(DOCKER_REPO)

usage: ## List available targets
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'
