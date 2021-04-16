/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package sample

import (
	"compress/gzip"
	"context"
	"crypto"
	"encoding/binary"
	"fmt"
	"os"
	"time"

	"github.com/hyperledger-labs/mirbft"
	"github.com/hyperledger-labs/mirbft/pkg/eventlog"
	pb "github.com/hyperledger-labs/mirbft/pkg/pb/msgs"
	"github.com/hyperledger-labs/mirbft/pkg/reqstore"
	"github.com/hyperledger-labs/mirbft/pkg/simplewal"
	"github.com/jyellick/mirbft-sample/config"
	"github.com/jyellick/mirbft-sample/network"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

type Server struct {
	Logger           *zap.SugaredLogger
	NodeConfig       *config.NodeConfig
	WALPath          string
	RequestStorePath string
	EventLogPath     string
	Serial           bool

	doneC chan struct{}
	exitC chan struct{}
}

type MirLogAdapter zap.SugaredLogger

func (m *MirLogAdapter) Log(level mirbft.LogLevel, msg string, pairs ...interface{}) {
	z := (*zap.SugaredLogger)(m)
	switch level {
	case mirbft.LevelDebug:
		z.Debugw(msg, pairs...)
	case mirbft.LevelInfo:
		z.Infow(msg, pairs...)
	case mirbft.LevelWarn:
		z.Warnw(msg, pairs...)
	case mirbft.LevelError:
		z.Errorw(msg, pairs...)
	}
}

func (s *Server) Run() error {
	s.doneC = make(chan struct{})
	s.exitC = make(chan struct{})
	defer close(s.exitC)

	mirConfig := mirConfig(s.NodeConfig)
	mirConfig.Logger = (*MirLogAdapter)(s.Logger)

	var recorder *eventlog.Recorder
	if s.EventLogPath != "" {
		file, err := os.Create(s.EventLogPath)
		if err != nil {
			return errors.WithMessage(err, "could not create event log file")
		}
		recorder = eventlog.NewRecorder(
			s.NodeConfig.ID,
			file,
			eventlog.CompressionLevelOpt(gzip.NoCompression),
		)
		defer recorder.Stop()
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

	// Create transport
	t, err := network.NewServerTransport(s.Logger, s.NodeConfig)
	if err != nil {
		return errors.WithMessage(err, "could not create networking")
	}

	node, err := mirbft.NewNode(
		s.NodeConfig.ID,
		mirConfig,
		&mirbft.ProcessorConfig{
			Link:   t,
			Hasher: crypto.SHA256,
			App: &application{
				reqStore: reqStore,
			}, // TODO, make more useful fixme
			RequestStore: reqStore,
			WAL:          wal,
			Interceptor:  recorder,
		},
	)
	if err != nil {
		return errors.WithMessage(err, "could not create mirbft node")
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

			proposer := node.Client(clientID)
			if err != nil {
				return errors.Errorf("unknown client id\n", clientID)
			}

			err = proposer.Propose(context.Background(), msg.ReqNo, msg.Data)
			if err != nil {
				return errors.WithMessagef(err, "failed to propose message to client %d", clientID)
			}

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

	ticker := time.NewTicker(s.NodeConfig.MirRuntime.TickInterval)
	defer ticker.Stop()

	// Main control loop
	if firstStart {
		return node.ProcessAsNewNode(s.doneC, ticker.C, initialNetworkState(s.NodeConfig), []byte("initial-checkpoint-value"))
	}

	return node.RestartProcessing(s.doneC, ticker.C)
}

func (s *Server) Stop() {
	close(s.doneC)
	<-s.exitC
}

type application struct {
	count    uint64
	reqStore *reqstore.Store
}

func (app *application) Apply(entry *pb.QEntry) error {
	fmt.Printf("Committing an entry for seq_no=%d (current count=%d)\n", entry.SeqNo, app.count)
	for _, request := range entry.Requests {
		reqData, err := app.reqStore.GetRequest(request)
		if err != nil {
			return errors.WithMessage(err, "could get entry from request store")
		}
		fmt.Printf("  Applying clientID=%d reqNo=%d with data of length %d start %q to log\n", request.ClientId, request.ReqNo, len(reqData), string(reqData[:26]))
		app.count++
	}

	return nil
}

func (app *application) Snap(networkConfig *pb.NetworkState_Config, clients []*pb.NetworkState_Client) ([]byte, []*pb.Reconfiguration, error) {
	// XXX, we put the entire configuration into the snapshot value, we should
	// really hash this, and have some protocol level state transfer, but this is easy for now
	// and relatively small.  Also note, proto isn't deterministic, but for right now, good enough.
	data, err := proto.Marshal(&pb.NetworkState{
		Config:  networkConfig,
		Clients: clients,
	})
	if err != nil {
		return nil, nil, errors.WithMessage(err, "could not marsshal network state")
	}

	countValue := make([]byte, 8)
	binary.BigEndian.PutUint64(countValue, uint64(app.count))

	return append(countValue, data...), nil, nil
}

func (app *application) TransferTo(seq uint64, value []byte) (*pb.NetworkState, error) {
	countValue := value[:8]
	app.count = binary.BigEndian.Uint64(countValue)

	stateValue := value[8:]
	ns := &pb.NetworkState{}
	err := proto.Unmarshal(stateValue, ns)
	if err != nil {
		return nil, errors.WithMessage(err, "could not unmarshal checkpoint value to network state")
	}
	fmt.Printf("Completed state transfer to sequence %d with a total count of %d requests applied\n", seq, app.count)

	return ns, nil
}

func mirConfig(nodeConfig *config.NodeConfig) *mirbft.Config {
	return &mirbft.Config{
		BatchSize:            nodeConfig.MirRuntime.BatchSize,
		HeartbeatTicks:       nodeConfig.MirRuntime.HeartbeatTicks,
		SuspectTicks:         nodeConfig.MirRuntime.SuspectTicks,
		NewEpochTimeoutTicks: nodeConfig.MirRuntime.NewEpochTimeoutTicks,
		BufferSize:           nodeConfig.MirRuntime.BufferSize,
	}

}

func initialNetworkState(nodeConfig *config.NodeConfig) *pb.NetworkState {

	// The sample application relies on the configuration assigning client IDs contiguously starting from 0.
	networkState := mirbft.StandardInitialNetworkState(len(nodeConfig.Nodes), len(nodeConfig.Clients))
	networkState.Config.NumberOfBuckets = int32(nodeConfig.MirBootstrap.NumberOfBuckets)
	networkState.Config.CheckpointInterval = int32(nodeConfig.MirBootstrap.CheckpointInterval)
	for _, client := range networkState.Clients {
		client.Width = nodeConfig.MirBootstrap.ClientWindowSize
	}

	return networkState
}
