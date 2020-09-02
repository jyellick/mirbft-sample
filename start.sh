#!/bin/bash

mkdir -p output

for ii in $(seq 0 3) ; do
	./mir-sample cryptogen/config${ii}.yaml &> output/${ii}.log &
done
