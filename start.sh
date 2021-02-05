#!/bin/bash

set -ex

# Create binaries.
go build ./cmd/bootstrap
go build ./cmd/node
go build ./cmd/client

# Clean up.
killall node client &> /dev/null || true
rm -Rf bootstrap.d
mkdir -p bootstrap.d

# Generate new configuration.
./bootstrap

# Start nodes.
for ii in $(seq 0 3) ; do
	CONFIG="bootstrap.d/node${ii}/config/node-config.yaml"
	RUNDIR="bootstrap.d/node${ii}/run/"
	./node --nodeConfig="${CONFIG}" --runDir="${RUNDIR}" --eventLog &> "${RUNDIR}/node.log" &
done

# Run client.
CL_CONFIG="bootstrap.d/client0/config/client-config.yaml"
CL_RUNDIR="bootstrap.d/client0/run/"
mkdir -p "${CL_RUNDIR}"
sleep 10
./client --clientConfig "${CL_CONFIG}" &> "${CL_RUNDIR}/client.log"
sleep 10

# Stop nodes.
killall node
wait
