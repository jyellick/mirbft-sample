#!/bin/bash

set -ex

# Set output directory
if [ -n "$1" ]; then
  OUTDIR="$1"
else
  OUTDIR=bootstrap.d
fi

# Create binaries.
go build ./cmd/bootstrap
go build ./cmd/node
go build ./cmd/client

# Clean up.
killall node client &> /dev/null || true
rm -Rf "${OUTDIR}"
mkdir -p "${OUTDIR}"

# Generate new configuration.
./bootstrap --outputDir="${OUTDIR}"

# Start nodes.
for ii in $(seq 0 3) ; do
	CONFIG="${OUTDIR}/node${ii}/config/node-config.yaml"
	RUNDIR="${OUTDIR}/node${ii}/run/"
	./node --nodeConfig="${CONFIG}" --runDir="${RUNDIR}" --eventLog &> "${RUNDIR}/node.log" &
done

# Run client.
CL_CONFIG="${OUTDIR}/client0/config/client-config.yaml"
CL_RUNDIR="${OUTDIR}/client0/run/"
mkdir -p "${CL_RUNDIR}"
sleep 10
./client --clientConfig "${CL_CONFIG}" &> "${CL_RUNDIR}/client.log"
sleep 10

# Stop nodes.
killall node
wait
