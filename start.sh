#!/bin/bash

set -e

echo "Killing any running nodes"
killall node &> /dev/null || true

echo "Building bootstrap util"
go build ./cmd/bootstrap

echo "Building node"
go build ./cmd/node

echo "Building client"
go build ./cmd/client

echo "Removing old bootstrap and rebootstrapping"
rm -Rf bootstrap.d
mkdir -p bootstrap.d
./bootstrap

echo
echo "Starting nodes"
for ii in $(seq 0 3) ; do
	echo "  starting node $ii"
	CONFIG="bootstrap.d/node${ii}/config/node-config.yaml"
	RUNDIR="bootstrap.d/node${ii}/run/"
	./node --nodeConfig="${CONFIG}" --runDir="${RUNDIR}" --eventLog &> "${RUNDIR}/node.log" &
done


echo
echo To inject some requests into the system run:
echo
echo ./client --clientConfig bootstrap.d/client0/config/client-config.yamlclient-config.yaml 
echo
