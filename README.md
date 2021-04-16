# mirbft-sample

This is a small sample application utilizing the [MirBFT library](https://github.com/hyperledger-labs/mirbft).

This sample is currently reasonably functional, but has not been tested extensively.  It is meant to be instructive, and is based on a development level of the mirbft library, so is not stable.

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

You may want to watch at least one node log via something like:

```
tail -f bootstrap.d/node1/run/node.log
```

4. You may now use the provided sample client to inject requests into the system such as:

```
./client --clientConfig bootstrap.d/client0/config/client-config.yaml
```

By default, the client will attempt to inject an additional 10,000 requests of size 10kb each into the system.  It may be run multiple times.
