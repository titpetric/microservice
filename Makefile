.PHONY: all build build-cli templates rpc migrate tidy docker push lint

# run the CI job for everything

all:
	@drone exec

# build all cmd/ programs

build: export GOOS = linux
build: export GOARCH = amd64
build: export CGO_ENABLED = 0
build: $(shell ls -d cmd/* | grep -v "\-cli" | sed -e 's/cmd\//build./')
	@echo OK.

build.%: SERVICE=$*
build.%:
	go build -o build/$(SERVICE)-$(GOOS)-$(GOARCH) ./cmd/$(SERVICE)/*.go

# build cli tooling from cmd/

build-cli: export GOOS = linux
build-cli: export GOARCH = amd64
build-cli: export CGO_ENABLED = 0
build-cli: $(shell ls -d cmd/*-cli | sed -e 's/cmd\//build-cli./')
	@echo OK.

build-cli.%: SERVICE=$*
build-cli.%:
	go build -o build/$(SERVICE)-$(GOOS)-$(GOARCH) ./cmd/$(SERVICE)/*.go


# code generator for client/server/cmd

templates: export MODULE=$(shell grep ^module go.mod | sed -e 's/module //g')
templates: $(shell ls -d rpc/* | sed -e 's/rpc\//templates./g')
	@rm db/schema_*.go db/schema.go
	@./templates/db_schema.go.sh
	@./templates/client_wire.go.sh
	go fmt ./...
	@echo OK.

templates.%: export SERVICE=$*
templates.%: export SERVICE_CAMEL=$(shell echo $(SERVICE) | sed -r 's/(^|_)([a-z])/\U\2/g')
templates.%: export MODULE=$(shell grep ^module go.mod | sed -e 's/module //g')
templates.%:
	@mkdir -p cmd/$(SERVICE) client/$(SERVICE) server/$(SERVICE)
	@envsubst < templates/cmd_main.go.tpl > cmd/$(SERVICE)/main.go
	@echo "~ cmd/$(SERVICE)/main.go"
	@envsubst < templates/client_client.go.tpl > client/$(SERVICE)/client.go
	@echo "~ client/$(SERVICE)/client.go"
	@./templates/server_server.go.sh
	@envsubst < templates/server_wire.go.tpl > server/$(SERVICE)/wire.go
	@echo "~ server/$(SERVICE)/wire.go"

# rpc generators

rpc: $(shell ls -d rpc/* | sed -e 's/\//./g')
	@echo OK.

rpc.%: SERVICE=$*
rpc.%:
	@echo '> protoc gen for $(SERVICE)'
	@protoc --proto_path=$(GOPATH)/src:. -Irpc/$(SERVICE) --go_out=plugins=grpc,paths=source_relative:. rpc/$(SERVICE)/$(SERVICE).proto
	@protoc --proto_path=$(GOPATH)/src:. -Irpc/$(SERVICE) --twirp_out=paths=source_relative:. rpc/$(SERVICE)/$(SERVICE).proto

# database migrations

migrate: $(shell ls -d db/schema/*/migrations.sql | xargs -n1 dirname | sed -e 's/db.schema./migrate./')
	@echo OK.

migrate.%: export SERVICE = $*
migrate.%: export MYSQL_ROOT_PASSWORD = default
migrate.%:
	mkdir -p db/types
	mysql -h mysql-test -u root -p$(MYSQL_ROOT_PASSWORD) -e "CREATE DATABASE $(SERVICE);"
	./build/db-migrate-cli-linux-amd64 -service $(SERVICE) -db-dsn "root:$(MYSQL_ROOT_PASSWORD)@tcp(mysql-test:3306)/$(SERVICE)" -real=true
	./build/db-migrate-cli-linux-amd64 -service $(SERVICE) -db-dsn "root:$(MYSQL_ROOT_PASSWORD)@tcp(mysql-test:3306)/$(SERVICE)" -real=true
	./build/db-schema-cli-linux-amd64 -schema $(SERVICE) -db-dsn "root:$(MYSQL_ROOT_PASSWORD)@tcp(mysql-test:3306)/$(SERVICE)" -format go -output server/$(SERVICE)
	./build/db-schema-cli-linux-amd64 -schema $(SERVICE) -db-dsn "root:$(MYSQL_ROOT_PASSWORD)@tcp(mysql-test:3306)/$(SERVICE)" -format markdown -output docs/schema/$(SERVICE)

tidy:
	go mod tidy > /dev/null 2>&1
	go mod download > /dev/null 2>&1
	go fmt ./... > /dev/null 2>&1

# docker image build

IMAGE_PREFIX := titpetric/service-

docker: $(shell ls -d cmd/* | sed -e 's/cmd\//docker./')
	@echo OK.

docker.%: export SERVICE = $(shell basename $*)
docker.%:
	@figlet $(SERVICE)
	docker build --rm --no-cache -t $(IMAGE_PREFIX)$(SERVICE) --build-arg service_name=$(SERVICE) -f docker/serve/Dockerfile .

# docker image push

push: $(shell ls -d cmd/* | sed -e 's/cmd\//push./')
	@echo OK.

push.%: export SERVICE = $(shell basename $*)
push.%:
	@figlet $(SERVICE)
	docker push $(IMAGE_PREFIX)$(SERVICE)

# lint code

lint:
	golint -set_exit_status ./...