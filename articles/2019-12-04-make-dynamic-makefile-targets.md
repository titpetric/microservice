# Make: Dynamic Makefile targets

A common pattern which we want to support when building our services is to build
everything under `rpc/{service}/{service}.proto` as well as `cmd/*` and other
locations which may be manually written or generated with various tools.

Currently, the state of our project is such that we would need to add individual
`go build` and other lines for each `cmd/` folder, or copy our `protoc` steps
in `.drone.yml` in order to support additional services. Since that is just busy
work, let's look at how to generate dynamic make targets, that will help us out.

## Dynamic Makefile target for building RPC/Protobuf files

The first target that we want to support is running all code generation required
for our RPC services. We define a `rpc` makefile target, and use dynamic execution
to build a list of dynamic targets:

~~~Makefile
rpc: $(shell ls -d rpc/* | sed -e 's/\//./')
~~~

Here we use the make built-in feature to run a shell, and so define our dynamic targets.
For each folder found under `rpc/`, a target like `rpc.{folder}` is created. For example,
if we wanted to build the code for a target, we could do:

~~~Makefile
rpc.stats:
	protoc --proto_path=$GOPATH/src:. -Irpc/stats --go_out=paths=source_relative:. rpc/stats/stats.proto
	protoc --proto_path=$GOPATH/src:. -Irpc/stats --twirp_out=paths=source_relative:. rpc/stats/stats.proto
~~~

If we used `rpc/stats` as the target name, `make` wouldn't do anything as that file/folder
already exists. This is why we are rewriting the target name to include a `.` (rpc.stats).
The obvious second part of our requirement is to make this target dynamic. We want to build
any number of services.

~~~diff
--- a/Makefile
+++ b/Makefile
@@ -1,4 +1,10 @@
-.PHONY: all
+.PHONY: all rpc

 all:
        drone exec
+
+rpc: $(shell ls -d rpc/* | sed -e 's/\//./g')
+rpc.%: SERVICE=$*
+rpc.%:
+	@echo '> protoc gen for $(SERVICE)'
+       @protoc --proto_path=$(GOPATH)/src:. -Irpc/$(SERVICE) --go_out=paths=source_relative:. rpc/$(SERVICE)/$(SERVICE).proto
+       @protoc --proto_path=$(GOPATH)/src:. -Irpc/$(SERVICE) --twirp_out=paths=source_relative:. rpc/$(SERVICE)/$(SERVICE).proto
~~~

The `%` target takes any option (which doesn't include / in the pattern). We can also declare
variables like `SERVICE` for each target. The variable can also be exported into the environment
by prefixing it with `export ` (example: `rpc.%: export SERVICE=$*`). Currently we don't need this,
but we will need it later on to pass build flags for our cmd/ programs.

The `$*` placeholder is the matched target parameter. As the target is `rpc.stats`, the variable here
will only contain `stats` but not `rpc.` since it's part of the target definition.

With the `@` prefix on individual commands in the target, we suppress the output of the command.
All we need to do is update the step in `.drone.yml` into `make rpc`, and we have support for
a dynamic number of services. Running make verifies this:

~~~
# make
[test:0] + make rpc
[test:1] > protoc gen for stats
[test:2] + go mod tidy > /dev/null 2>&1
[test:3] + go mod download > /dev/null 2>&1
[test:4] + go fmt ./... > /dev/null 2>&1
~~~

## Building our services from cmd/

For each service, we will create a `cmd/{service}/*.go` structure, containing at least `main.go`.
Let's start with adding a simple `cmd/stats/main.go` with a hello world to greet us. We will come
back and scaffold the actual service later.

~~~go
package main

void main() {
	println("Hello world")
}
~~~

> The function `println` is a Go built-in function that works without importing any package.
> It shouldn't really be used, but as far as providing some test output, it's the shortest
> way to do that. We will throw this program away, so don't pay it much attention.

Now, we want our app to build all the applications under `cmd/`, by running `make build`.

~~~Makefile
build: export GOOS = linux
build: export GOARCH = amd64
build: export CGO_ENABLED = 0
build: $(shell ls -d cmd/* | sed -e 's/cmd\//build./')
	echo OK.

build.%: SERVICE=$*
build.%:
	go build -o build/$(SERVICE)-$(GOOS)-$(GOARCH) ./cmd/$(SERVICE)/*.go
~~~

For the main `build` target, we define our build environment variables - we want to build our
services for linux, for amd64 architecture, and we want to disable CGO so we have static binaries.
We list all cmd locations as dynamic targets and remap them to `build.%`, similarly to what we
do with the rpc target. All that is left to do is to add `make build` at the end of `.drone.yml`,
and add our `/build` folder into the `.gitignore` file for our project.
