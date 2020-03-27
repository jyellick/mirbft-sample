#!/bin/bash

for ii in $(seq 0 3) ; do
	./mir-sample cryptogen/config${ii}.yaml &> ${ii}.log &
done