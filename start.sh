#!/bin/bash

mkdir -p output

for ii in $(seq 0 3) ; do
	./mir-sample --cryptoConfig cryptogen/config${ii}.yaml --eventLog output/${ii}.eventlog &> output/${ii}.log &
done
