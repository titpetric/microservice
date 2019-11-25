.PHONY: all rpc

all:
	@drone exec

build: export GOOS = linux
build: export GOARCH = amd64
build: export CGO_ENABLED = 0
build: $(shell ls -d cmd/* | sed -e 's/cmd\//build./')
	@echo OK.

build.%: SERVICE=$*
build.%:
	go build -o build/$(SERVICE)-$(GOOS)-$(GOARCH) ./cmd/$(SERVICE)/*.go


rpc: $(shell ls -d rpc/* | sed -e 's/\//./g')
rpc.%: SERVICE=$*
rpc.%:
	@echo '> protoc gen for $(SERVICE)'
	@protoc --proto_path=$(GOPATH)/src:. -Irpc/$(SERVICE) --go_out=paths=source_relative:. rpc/$(SERVICE)/$(SERVICE).proto
	@protoc --proto_path=$(GOPATH)/src:. -Irpc/$(SERVICE) --twirp_out=paths=source_relative:. rpc/$(SERVICE)/$(SERVICE).proto
