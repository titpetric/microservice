#!/bin/bash
cd $(dirname $(dirname $(readlink -f $0)))

if [ -z "${SERVICE}" ]; then
	echo "Usage: SERVICE=[name] MODULE=... $0"
	exit 255
fi

OUTPUT="server/${SERVICE}/wire.go"

if [ ! -f "$OUTPUT" ]; then
	envsubst < templates/server_wire.go.tpl > $OUTPUT
	echo "~ $OUTPUT"
fi
