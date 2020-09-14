#!/bin/bash

killall node &> /dev/null
rm -Rf bootstrap.d
mkdir -p bootstrap.d

./bootstrap

for ii in $(seq 0 3) ; do
	CONFIG="bootstrap.d/node${ii}/config/node-config.yaml"
	RUNDIR="bootstrap.d/node${ii}/run/"
	./node --nodeConfig="${CONFIG}" --runDir="${RUNDIR}" --eventLog &> "${RUNDIR}/node.log" &
done
