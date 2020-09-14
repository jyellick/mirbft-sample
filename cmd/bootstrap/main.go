/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/guoger/mir-sample/config"
	"github.com/perlin-network/noise"
	"github.com/pkg/errors"
	"gopkg.in/alecthomas/kingpin.v2"
	"gopkg.in/yaml.v2"
)

func dirIsEmpty(name string) (bool, error) {
	f, err := os.Open(name)
	if err != nil {
		return false, err
	}
	defer f.Close()

	_, err = f.Readdirnames(1) // Or f.Readdir(1)
	if err == io.EOF {
		return true, nil
	}
	return false, err // Either not empty or error, suits both cases
}

type args struct {
	outputDir   string
	basePort    uint16
	nodeCount   uint16
	clientCount uint16
}

func parseArgs(argsString []string) (*args, error) {
	app := kingpin.New("bootstrap", "A small bootstrapping tool to bootstrap a mir-sample network.")
	outputDir := app.Flag("outputDir", "The directory in which to create the bootstrap config.").Default("bootstrap.d").ExistingDir()
	basePort := app.Flag("basePort", "The initial port for the first node, incremented per node.").Default("5000").Uint16()
	nodeCount := app.Flag("nodeCount", "The total number of nodes to create for this network.").Default("4").Uint16()
	clientCount := app.Flag("clientCount", "The total number of clients to create for this network.").Default("1").Uint16()

	_, err := app.Parse(argsString)
	if err != nil {
		return nil, err
	}

	isEmpty, err := dirIsEmpty(*outputDir)
	if err != nil {
		return nil, errors.WithMessagef(err, "could not read outputDir '%s'", *outputDir)
	}

	if !isEmpty {
		return nil, errors.Errorf("outputDir '%s' is not empty", *outputDir)
	}

	return &args{
		outputDir:   *outputDir,
		basePort:    *basePort,
		nodeCount:   *nodeCount,
		clientCount: *clientCount,
	}, nil
}

func (a *args) bootstrap() error {
	var nodes []config.Node
	var nodePrivateKeys []string
	var clients []config.Client
	var clientPrivateKeys []string

	for i := uint16(0); i < a.nodeCount; i++ {
		pubkey, privkey, err := noise.GenerateKeys(nil)
		if err != nil {
			return errors.WithMessagef(err, "could not generate key for node %d", i)
		}

		nodePrivateKeys = append(nodePrivateKeys, privkey.String())

		node := config.Node{
			ID:        uint64(i),
			Address:   fmt.Sprintf("127.0.0.1:%d", a.basePort+i),
			PublicKey: pubkey.String(),
		}
		nodes = append(nodes, node)
	}

	for i := uint16(0); i < a.clientCount; i++ {
		pubkey, privkey, err := noise.GenerateKeys(nil)
		if err != nil {
			return errors.WithMessagef(err, "could not generate key for node %d", i)
		}

		clientPrivateKeys = append(clientPrivateKeys, privkey.String())

		client := config.Client{
			ID:        uint64(i),
			PublicKey: pubkey.String(),
		}
		clients = append(clients, client)
	}

	for i := uint16(0); i < a.nodeCount; i++ {
		config := config.NodeConfig{
			ID:            uint64(i),
			ListenAddress: nodes[i].Address,
			PrivateKey:    nodePrivateKeys[i],
			MirRuntime: config.MirRuntime{
				TickInterval:         time.Second,
				HeartbeatTicks:       1,
				SuspectTicks:         4,
				NewEpochTimeoutTicks: 8,
				BatchSize:            20,
				BufferSize:           1000,
			},
			MirBootstrap: config.MirBootstrap{
				NumberOfBuckets:    1,
				ClientWindowSize:   5000,
				CheckpointInterval: 20,
			},
			Nodes:   nodes,
			Clients: clients,
		}

		confDir := filepath.Join(a.outputDir, fmt.Sprintf("node%d", i), "config")

		err := os.MkdirAll(confDir, 0700)
		if err != nil {
			return errors.WithMessage(err, "could not create config dir")
		}

		out, err := yaml.Marshal(config)
		if err != nil {
			return errors.WithMessagef(err, "could not marshal node config %d to yaml", i)
		}

		err = ioutil.WriteFile(filepath.Join(confDir, "node-config.yaml"), out, 0600)
		if err != nil {
			return errors.WithMessagef(err, "could not write node config %d", i)
		}

		runDir := filepath.Join(a.outputDir, fmt.Sprintf("node%d", i), "run")

		err = os.MkdirAll(runDir, 0700)
		if err != nil {
			return errors.WithMessage(err, "could not create run dir")
		}
	}

	for i := uint16(0); i < a.clientCount; i++ {
		config := config.ClientConfig{
			ID:         uint64(i),
			PrivateKey: clientPrivateKeys[i],
			Nodes:      nodes,
		}

		confDir := filepath.Join(a.outputDir, fmt.Sprintf("client%d", i), "config")

		err := os.MkdirAll(confDir, 0700)
		if err != nil {
			return errors.WithMessage(err, "could not create config dir")
		}

		out, err := yaml.Marshal(config)
		if err != nil {
			return errors.WithMessagef(err, "could not marshal node config %d to yaml", i)
		}

		err = ioutil.WriteFile(filepath.Join(confDir, "client-config.yaml"), out, 0600)
		if err != nil {
			return errors.WithMessagef(err, "could not write client config %d", i)
		}
	}
	return nil
}

func main() {
	kingpin.Version("0.0.1")
	args, err := parseArgs(os.Args[1:])
	if err != nil {
		kingpin.Fatalf("Error parsing arguments, %s, try --help", err)
	}

	err = args.bootstrap()
	if err != nil {
		kingpin.Fatalf("Error bootstrapping, %s", err)
	}

}
