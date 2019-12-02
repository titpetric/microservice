#!/bin/bash
cd $(dirname $(dirname $(readlink -f $0)))

if [ -z "${SERVICE}" ]; then
	echo "Usage: SERVICE=[name] SERVICE_CAMEL=[Name] $0"
	exit 255
fi

OUTPUT="server/${SERVICE}/server.go"

# only generate server.go if it doesn't exist
if [ ! -f "$OUTPUT" ]; then
	envsubst < templates/server_server.go.tpl > $OUTPUT
	impl -dir rpc/${SERVICE} 'svc *Server' ${SERVICE}.${SERVICE_CAMEL}Service >> $OUTPUT
	echo "~ $OUTPUT"
fi
