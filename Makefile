.PHONY: all rpc build templates

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


templates: $(shell ls -d rpc/* | sed -e 's/rpc\//templates./g')
	rm db/schema_*.go db/schema.go
	./templates/db_schema.go.sh
	@echo OK.

templates.%: export SERVICE=$*
templates.%: export SERVICE_CAMEL=$(shell echo $(SERVICE) | sed -r 's/(^|_)([a-z])/\U\2/g')
templates.%: export MODULE=$(shell grep ^module go.mod | sed -e 's/module //g')
templates.%:
	mkdir -p cmd/$(SERVICE) client/$(SERVICE) server/$(SERVICE)
	envsubst < templates/cmd_main.go.tpl > cmd/$(SERVICE)/main.go
	envsubst < templates/client_client.go.tpl > client/$(SERVICE)/client.go
	envsubst < templates/server_server.go.tpl > server/$(SERVICE)/server.go
	impl -dir rpc/$(SERVICE) 'svc *Server' $(SERVICE).$(SERVICE_CAMEL)Service >> server/$(SERVICE)/server.go

rpc: $(shell ls -d rpc/* | sed -e 's/\//./g')
rpc.%: SERVICE=$*
rpc.%:
	@echo '> protoc gen for $(SERVICE)'
	@protoc --proto_path=$(GOPATH)/src:. -Irpc/$(SERVICE) --go_out=paths=source_relative:. rpc/$(SERVICE)/$(SERVICE).proto
	@protoc --proto_path=$(GOPATH)/src:. -Irpc/$(SERVICE) --twirp_out=paths=source_relative:. rpc/$(SERVICE)/$(SERVICE).proto
