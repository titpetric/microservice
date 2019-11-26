.PHONY: all rpc build templates

# run the CI job for everything

all:
	@drone exec

# build all cmd/ programs

build: export GOOS = linux
build: export GOARCH = amd64
build: export CGO_ENABLED = 0
build: $(shell ls -d cmd/* | sed -e 's/cmd\//build./')
	@echo OK.

build.%: SERVICE=$*
build.%:
	go build -o build/$(SERVICE)-$(GOOS)-$(GOARCH) ./cmd/$(SERVICE)/*.go


# code generator for client/server/cmd

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

# rpc generators

rpc: $(shell ls -d rpc/* | sed -e 's/\//./g')
	@echo OK.

rpc.%: SERVICE=$*
rpc.%:
	@echo '> protoc gen for $(SERVICE)'
	@protoc --proto_path=$(GOPATH)/src:. -Irpc/$(SERVICE) --go_out=paths=source_relative:. rpc/$(SERVICE)/$(SERVICE).proto
	@protoc --proto_path=$(GOPATH)/src:. -Irpc/$(SERVICE) --twirp_out=paths=source_relative:. rpc/$(SERVICE)/$(SERVICE).proto

# database migrations

migrate: $(shell ls -d db/schema/*/migrations.sql | xargs -n1 dirname | sed -e 's/db.schema./migrate./')
	@echo OK.

migrate.%: export SERVICE = $*
migrate.%: export MYSQL_ROOT_PASSWORD = default
migrate.%:
	mysql -h mysql-test -u root -p$(MYSQL_ROOT_PASSWORD) -e "CREATE DATABASE $(SERVICE);"
	./build/db-migrate-linux-amd64 -service $(SERVICE) -db-dsn "root:$(MYSQL_ROOT_PASSWORD)@tcp(mysql-test:3306)/$(SERVICE)" -real=true
	./build/db-migrate-linux-amd64 -service $(SERVICE) -db-dsn "root:$(MYSQL_ROOT_PASSWORD)@tcp(mysql-test:3306)/$(SERVICE)" -real=true
