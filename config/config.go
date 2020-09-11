/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package config

import (
	"io"
	"io/ioutil"
	"time"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
)

type NodeConfig struct {
	ID            uint64   `yaml:"id"`
	ListenAddress string   `yaml:"listen_address"`
	PrivateKey    string   `yaml:"private_key"`
	Mir           Mir      `yaml:"mir"`
	Nodes         []Node   `yaml:"nodes"`
	Clients       []Client `yaml:"clients"`
}

type ClientConfig struct {
	ID         uint64 `yaml:"id"`
	PrivateKey string `yaml:"private_key"`
	Nodes      []Node `yaml:"nodes"`
}

type Node struct {
	ID        uint64 `yaml:"id"`
	Address   string `yaml:"address"`
	PublicKey string `yaml:"public_key"`
}

type Client struct {
	ID        uint64 `yaml:"id"`
	PublicKey string `yaml:"public_key"`
}

// MirRuntime contains per Node instance fields which should be consistent
// across honest nodes, but which may be tweaked after bootstrap.
type MirRuntime struct {
	TickInterval            time.Duration `yaml:"tick_interval"`
	HeartbeatTicks          uint32        `yaml:"heartbeat_ticks"`
	SuspectTicks            uint32        `yaml:"suspect_ticks"`
	EpochChangeTimeoutTicks uint32        `yaml:"epoch_change_timeout_ticks"`
	BatchSize               uint32        `yaml:"batch_size"`
}

// MirBootstrap contains network-wide initialization parameters which cannot
// be modified directly, but must be modified via the Mir state machine.
type MirBootstrap struct {
	NumberOfBuckets    uint32 `yaml:"number_of_buckets"`
	ClientWindowSize   uint32 `yaml:"client_window_size"`
	CheckpointInterval uint32 `yaml"checkpoint_interval"`
}

func LoadNodeConfig(f io.Reader) (*NodeConfig, error) {
	data, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, errors.WithMessage(err, "error reading config")
	}

	config := &NodeConfig{}
	if err = yaml.Unmarshal(data, config); err != nil {
		return nil, err
	}

	return config, nil
}

func LoadClientConfig(f io.Reader) (*NodeConfig, error) {
	data, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, errors.WithMessage(err, "error reading config")
	}

	config := &NodeConfig{}
	if err = yaml.Unmarshal(data, config); err != nil {
		return nil, err
	}

	return config, nil
}
