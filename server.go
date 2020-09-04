package main

import (
	"context"
	"crypto/md5"
	"fmt"
	"time"

	"github.com/IBM/mirbft"
	pb "github.com/IBM/mirbft/mirbftpb"
	"github.com/IBM/mirbft/recorder"
	"github.com/IBM/mirbft/sample"
	"github.com/golang/protobuf/proto"
	"github.com/guoger/mir-sample/network"
	"go.uber.org/zap"
)

type server struct {
	node      *mirbft.Node
	transport *network.Transport
	config    *serverConfig
}

func (s *server) run() error {
	err := s.transport.Start()
	if err != nil {
		return err
	}
	defer s.transport.Close()

	// TODO, maybe detect if this is first start and actually
	// detect other node liveness via the transport?

	// let the links establish first to reduce logspam...
	time.Sleep(2 * time.Second)

	// a simple non-optimized processor
	processor := &sample.SerialProcessor{
		Link:   s.transport,
		Hasher: md5.New,
		Log:    s.config.log,
		Node:   s.node,
	}

	tickTime := 1000 * time.Millisecond
	ticker := time.NewTicker(tickTime)
	defer ticker.Stop()

	node := s.node

	// Main control loop
	for {
		select {
		case <-s.config.log.applicationDone:
			return nil
		case <-ticker.C:
			err := node.Tick()
			if err != nil {
				return err
			}
		case actions := <-node.Ready():
			results := processor.Process(&actions)
			err := node.AddResults(*results)
			if err != nil {
				return err
			}
		case <-node.Err():
			_, err := node.Status(context.Background())
			return err
		}
	}
}

type serverConfig struct {
	batchSize       uint32
	storage         mirbft.Storage
	recording       *recorder.Interceptor
	log             *applicationLog
	transportConfig *network.Config
	doneC           <-chan struct{}
}

func (sc *serverConfig) initialize() (*server, error) {
	logger := zap.NewExample()

	nodeConfig := &mirbft.Config{
		ID:                   sc.transportConfig.ID,
		Logger:               logger,
		BatchSize:            sc.batchSize,
		HeartbeatTicks:       2,
		SuspectTicks:         4,
		NewEpochTimeoutTicks: 8,
		BufferSize:           500,
	}

	// Note, if the interface value is not actually nil
	// we will end up with a nil dereference in the node
	if sc.recording != nil {
		nodeConfig.EventInterceptor = sc.recording
	}

	node, err := mirbft.StartNode(nodeConfig, sc.doneC, sc.storage)
	if err != nil {
		return nil, err
	}

	// Create transport
	t, err := network.NewTransport(logger, sc.transportConfig)
	if err != nil {
		return nil, err
	}

	t.Handle(func(id uint64, data []byte) {
		msg := &pb.Msg{}
		err := proto.Unmarshal(data, msg)
		if err != nil {
			panic(fmt.Sprintf("unexpected marshaling error: %s", err))
		}

		err = node.Step(context.Background(), id, msg)
		if err != nil {
			logger.Warn(fmt.Sprintf("Failed to step message to mir node: %s", err))
		}
	})

	return &server{
		node:      node,
		transport: t,
		config:    sc,
	}, nil
}

type applicationLog struct {
	count             uint
	targetCount       uint
	timeAtFirstCommit time.Time
	applicationDone   chan struct{}
}

func (al *applicationLog) Apply(entry *pb.QEntry) {
	for _, request := range entry.Requests {
		fmt.Printf("Applying clientID=%d reqNo=%d with data %s to log\n", request.Request.ClientId, request.Request.ReqNo, request.Request.Data)
		if al.count == 0 {
			al.timeAtFirstCommit = time.Now()
		}
		al.count++

		if al.count == al.targetCount {
			fmt.Printf("Successfully applied all expected requests in %v!", time.Since(al.timeAtFirstCommit))
			close(al.applicationDone)
		}
	}
}

func (al *applicationLog) Snap() []byte {
	return []byte("unimplemented")
}
