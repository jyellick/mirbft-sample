# mirbft-sample

This is a small sample application utilizing the [MirBFT library](https://github.com/IBM/mirbft).

**WARNING**  This sample is currently _barely_ functional, and is meant to be instructive, not useful.  **WARNING**

That being said, if you'd like to use it, you may do the following:

1. Build the binaries:
 
```
go build ./cmd/bootstrap
go build ./cmd/node
```

2. Bootstrap the network configuration (you may want to play with the flags via `--help`).

```
mkdir -p bootstrap.d
./bootstrap
```

3. Start each node pointing to their configuration and a run directory.

```
./node --nodeConfig=bootstrap.d/node0/config/node-config.yaml --runDir=bootstrap.d/node0/run/ &
./node --nodeConfig=bootstrap.d/node1/config/node-config.yaml --runDir=bootstrap.d/node1/run/ &
./node --nodeConfig=bootstrap.d/node2/config/node-config.yaml --runDir=bootstrap.d/node2/run/ &
./node --nodeConfig=bootstrap.d/node3/config/node-config.yaml --runDir=bootstrap.d/node3/run/ &
```

You can alternatively execute `./start.sh` which will perform steps (1), (2), and (3) for you.

Note.  As warned, there are currently unsquashed bugs, and unimplemented features, and it's been observed that this network will often break and stop working after some period of time.  That's expected, please don't report it as a bug.  Feel free to submit PRs for documentation improvements, usability improvements, etc. but work to find and address these bugs is underway in the main repository with automated testing frameworks.
