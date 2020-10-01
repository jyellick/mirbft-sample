/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package sample

import (
	"context"
	"crypto/md5"
	"fmt"
	"os"
	"time"

	"github.com/IBM/mirbft"
	pb "github.com/IBM/mirbft/mirbftpb"
	"github.com/IBM/mirbft/recorder"
	"github.com/IBM/mirbft/reqstore"
	"github.com/IBM/mirbft/simplewal"
	"github.com/golang/protobuf/proto"
	"github.com/jyellick/mirbft-sample/config"
	"github.com/jyellick/mirbft-sample/network"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type Server struct {
	Logger           *zap.Logger
	NodeConfig       *config.NodeConfig
	WALPath          string
	RequestStorePath string
	EventLogPath     string
	Serial           bool

	doneC chan struct{}
	exitC chan struct{}
}

func (s *Server) Run() error {
	s.doneC = make(chan struct{})
	s.exitC = make(chan struct{})
	defer close(s.exitC)

	mirConfig := mirConfig(s.NodeConfig)
	mirConfig.Logger = s.Logger

	if s.EventLogPath != "" {
		file, err := os.Create(s.EventLogPath)
		if err != nil {
			return errors.WithMessage(err, "could not create event log file")
		}
		startTime := time.Now()
		recorder := recorder.NewInterceptor(
			s.NodeConfig.ID,
			func() int64 {
				return time.Since(startTime).Milliseconds()
			},
			5000,
		)
		go recorder.Drain(file)
		defer recorder.Stop()

		mirConfig.EventInterceptor = recorder
	}

	wal, err := simplewal.Open(s.WALPath)
	if err != nil {
		return errors.WithMessage(err, "could not open WAL")
	}
	defer wal.Close()

	firstStart, err := wal.IsEmpty()
	if err != nil {
		return errors.WithMessage(err, "could not query WAL")
	}

	reqStore, err := reqstore.Open(s.RequestStorePath)
	if err != nil {
		return errors.WithMessage(err, "could not open request store")
	}
	defer reqStore.Close()

	var node *mirbft.Node
	if firstStart {
		node, err = mirbft.StartNewNode(mirConfig, initialNetworkState(s.NodeConfig), []byte("initial-checkpoint-value"))
		if err != nil {
			return errors.WithMessage(err, "could not bootstrap node")
		}
	} else {
		node, err = mirbft.RestartNode(mirConfig, wal, reqStore)
		if err != nil {
			return errors.WithMessage(err, "could not restart node")
		}
	}
	defer node.Stop()

	// Create transport
	t, err := network.NewServerTransport(s.Logger, s.NodeConfig)
	if err != nil {
		return errors.WithMessage(err, "could not create networking")
	}

	clientProposers := map[uint64]*mirbft.ClientProposer{}
	for _, client := range s.NodeConfig.Clients {
		clientProposer, err := node.ClientProposer(context.Background(), client.ID)
		if err != nil {
			return errors.WithMessagef(err, "could not create proposer for client %d\n", client.ID)
		}
		clientProposers[client.ID] = clientProposer
	}

	t.Handle(
		func(nodeID uint64, data []byte) error {
			msg := &pb.Msg{}
			err := proto.Unmarshal(data, msg)
			if err != nil {
				return errors.WithMessage(err, "unexpected unmarshaling error")
			}

			err = node.Step(context.Background(), nodeID, msg)
			if err != nil {
				return errors.WithMessage(err, "failed to step message to mir node")
			}

			return nil
		},
		func(clientID uint64, data []byte) error {
			msg := &pb.Request{}
			err := proto.Unmarshal(data, msg)
			if err != nil {
				return errors.WithMessage(err, "unexpected unmarshaling error")
			}

			if msg.ClientId != clientID {
				return errors.Errorf("client ID mismatch, claims to be %d but is %d\n", msg.ClientId, clientID)
			}

			proposer, ok := clientProposers[clientID]
			if !ok {
				return errors.Errorf("unknown client id\n", clientID)
			}

			fmt.Printf("About to propose %d.%d\n", msg.ClientId, msg.ReqNo)

			err = proposer.Propose(context.Background(), msg)
			if err != nil {
				return errors.WithMessagef(err, "failed to propose message to client %d", clientID)
			}

			fmt.Printf(" ... done proposing %d.%d\n", msg.ClientId, msg.ReqNo)
			return nil
		},
	)

	err = t.Start()
	if err != nil {
		return errors.WithMessage(err, "could not start networking")
	}
	defer t.Close()

	// TODO, maybe detect if this is first start and actually
	// detect other node liveness via the transport?

	// let the links establish first to reduce logspam...
	time.Sleep(2 * time.Second)

	processor := &mirbft.Processor{
		Link:   t,
		Hasher: md5.New,
		Log: &applicationLog{
			reqStore: reqStore,
		}, // TODO, make more useful fixme
		Node:         node,
		RequestStore: reqStore,
		WAL:          wal,
	}
	var process func(*mirbft.Actions) *mirbft.ActionResults

	if s.Serial {
		process = processor.Process
	} else {
		parallelProcessor := mirbft.NewProcessorWorkPool(processor, mirbft.ProcessorWorkPoolOpts{})
		defer parallelProcessor.Stop()
		process = parallelProcessor.Process
	}

	ticker := time.NewTicker(s.NodeConfig.MirRuntime.TickInterval)
	defer ticker.Stop()

	// Main control loop
	for {
		select {
		case <-ticker.C:
			err := node.Tick()
			if err != nil {
				return err
			}
		case actions := <-node.Ready():
			results := process(&actions)
			err := node.AddResults(*results)
			if err != nil {
				return err
			}
		case <-node.Err():
			_, err := node.Status(context.Background())
			return err
		case <-s.doneC:
			return nil
		}
	}
}

func (s *Server) Stop() {
	close(s.doneC)
	<-s.exitC
}

type applicationLog struct {
	count    uint
	reqStore *reqstore.Store
}

func (al *applicationLog) Apply(entry *pb.QEntry) {
	fmt.Printf("Committing an entry for seq_no=%d\n", entry.SeqNo)
	for _, request := range entry.Requests {
		reqData, err := al.reqStore.Get(request)
		if err != nil {
			panic(err)
		}
		fmt.Printf("Applying clientID=%d reqNo=%d with data %s to log\n", request.ClientId, request.ReqNo, reqData)
		al.count++
	}
}

func (al *applicationLog) Snap() []byte {
	return []byte("unimplemented")
}

func mirConfig(nodeConfig *config.NodeConfig) *mirbft.Config {
	return &mirbft.Config{
		ID:                   nodeConfig.ID,
		BatchSize:            nodeConfig.MirRuntime.BatchSize,
		HeartbeatTicks:       nodeConfig.MirRuntime.HeartbeatTicks,
		SuspectTicks:         nodeConfig.MirRuntime.SuspectTicks,
		NewEpochTimeoutTicks: nodeConfig.MirRuntime.NewEpochTimeoutTicks,
		BufferSize:           nodeConfig.MirRuntime.BufferSize,
	}

}

func initialNetworkState(nodeConfig *config.NodeConfig) *pb.NetworkState {
	clientIDs := []uint64{}
	for _, client := range nodeConfig.Clients {
		clientIDs = append(clientIDs, client.ID)
	}

	networkState := mirbft.StandardInitialNetworkState(len(nodeConfig.Nodes), clientIDs...)
	networkState.Config.NumberOfBuckets = int32(nodeConfig.MirBootstrap.NumberOfBuckets)
	networkState.Config.CheckpointInterval = int32(nodeConfig.MirBootstrap.CheckpointInterval)
	for _, client := range networkState.Clients {
		client.Width = nodeConfig.MirBootstrap.ClientWindowSize
	}

	return networkState
}
