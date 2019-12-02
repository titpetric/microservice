#!/bin/bash
cd $(dirname $(dirname $(readlink -f $0)))

## list all services
schemas=$(ls -d rpc/* | xargs -n1 basename)

function render_wire {
	echo "package client"
	echo
	echo "import ("
	echo -e "\t\"github.com/google/wire\""
	echo
	for schema in $schemas; do
		echo -e "\t\"${MODULE}/client/${schema}\""
	done
	echo ")"
	echo
	echo "var Inject = wire.NewSet("
	for schema in $schemas; do
		echo -e "\t${schema}.New,"
	done
	echo ")"
}

render_wire > client/wire.go
echo "~ client/wire.go"
