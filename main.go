package main

import (
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/IBM/mirbft"
	pb "github.com/IBM/mirbft/mirbftpb"
	"github.com/IBM/mirbft/mock"
	"github.com/IBM/mirbft/recorder"
	"github.com/IBM/mirbft/sample"
	"github.com/golang/protobuf/proto"
	"github.com/guoger/mir-sample/network"
	"go.uber.org/zap"
)

type ScreenLog struct {
	count int
}

func (sl *ScreenLog) Apply(entry *pb.QEntry) {
	for _, request := range entry.Requests {
		fmt.Printf("Applying reqNo=%d with data %s to log\n", request.Request.ReqNo, request.Request.Data)
		sl.count++
	}
}

func (sl *ScreenLog) Snap() []byte {
	return []byte("unimplemented")
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func startRecording(id uint64, doneC <-chan struct{}) *recorder.Interceptor {
	file, err := os.Create(fmt.Sprintf("output/%d.eventlog", id))
	check(err)

	startTime := time.Now()
	interceptor := recorder.NewInterceptor(
		id,
		func() int64 {
			return time.Since(startTime).Milliseconds()
		},
		10000,
		doneC,
	)

	go func() {
		interceptor.Drain(file)
		defer file.Close()
	}()

	return interceptor
}

func mockStorage(networkState *pb.NetworkState) mirbft.Storage {
	storage := &mock.Storage{}
	storage.LoadReturnsOnCall(0, &pb.Persistent{
		Type: &pb.Persistent_CEntry{
			CEntry: &pb.CEntry{
				SeqNo:           0,
				CheckpointValue: []byte("fake-initial-value"),
				NetworkState:    networkState,
				EpochConfig: &pb.EpochConfig{
					Number:            0,
					Leaders:           networkState.Config.Nodes,
					PlannedExpiration: 0,
				},
			},
		},
	}, nil)
	storage.LoadReturnsOnCall(1, &pb.Persistent{
		Type: &pb.Persistent_EpochChange{
			EpochChange: &pb.EpochChange{
				NewEpoch: 1,
				Checkpoints: []*pb.Checkpoint{
					{
						SeqNo: 0,
						Value: []byte("fake-initial-value"),
					},
				},
			},
		},
	}, nil)
	storage.LoadReturnsOnCall(2, nil, io.EOF)

	return storage
}

func periodicallyPollStatus(node *mirbft.Node, frequency time.Duration, doneC <-chan struct{}) {
	go func() {
		for {
			select {
			case <-doneC:
				return
			case <-time.After(frequency):
				status, _ := node.Status(context.Background())
				fmt.Printf("Current status is:\n")
				fmt.Println(status.Pretty())
			}
		}
	}()
}

func main() {
	logger, err := zap.NewProduction()
	check(err)

	f := os.Args[1]
	config, err := network.LoadConfig(f)
	if err != nil {
		fmt.Printf("Could not load config at '%s': %s\n", os.Args[1], err)
	}
	check(err)

	// bootstrap network
	networkState := mirbft.StandardInitialNetworkState(len(config.Peers), 0)
	networkState.Config.NumberOfBuckets = 1
	networkState.Config.CheckpointInterval = 10

	doneC := make(chan struct{})

	mirConfig := &mirbft.Config{
		ID:                   config.ID,
		Logger:               logger,
		EventInterceptor:     startRecording(config.ID, doneC),
		BatchSize:            1,
		HeartbeatTicks:       2,
		SuspectTicks:         4,
		NewEpochTimeoutTicks: 8,
		BufferSize:           500,
	}

	node, err := mirbft.StartNode(mirConfig, doneC, mockStorage(networkState))
	check(err)

	// Create transport
	t, err := network.NewTransport(logger, config)
	check(err)

	t.Handle(func(id uint64, data []byte) {
		msg := &pb.Msg{}
		err := proto.Unmarshal(data, msg)
		check(err)

		err = node.Step(context.Background(), id, msg)
		if err != nil {
			logger.Warn(fmt.Sprintf("Failed to step message to mir node: %s", err))
		}
	})

	err = t.Start()
	check(err)
	defer t.Close()

	// let the links establish first...
	time.Sleep(2 * time.Second)

	screenLog := &ScreenLog{}

	processor := &sample.SerialProcessor{
		Link:   t,
		Hasher: md5.New,
		Log:    screenLog,
		Node:   node,
	}

	msgCount := 1000
	msgSeparation := 50 * time.Millisecond

	go func() {
		defer func() {
			fmt.Printf("Proposer go routine exiting\n")
		}()

		for i := 0; i < msgCount; i++ {
			// fmt.Printf("Proposing %d\n", i)
			req := &pb.Request{
				ClientId: 0,
				ReqNo:    uint64(i),
				Data:     []byte(fmt.Sprintf("data-%d", i)),
			}
			proposer, err := node.ClientProposer(context.Background(), 0)
			check(err)

			err = proposer.Propose(context.Background(), req)
			check(err)
			// fmt.Printf("Proposed %d\n", i)
			time.Sleep(msgSeparation)
		}
	}()

	periodicallyPollStatus(node, 10*time.Second, doneC)

	tickTime := 1000 * time.Millisecond
	ticker := time.NewTicker(tickTime)

	for {
		select {
		case <-ticker.C:
			err := node.Tick()
			check(err)
		case actions := <-node.Ready():
			results := processor.Process(&actions)
			if screenLog.count >= msgCount {
				fmt.Printf("\nDone committing %d requests\n", screenLog.count)
				ticker.Stop()
				close(doneC)
				<-node.Err()
				status, err := node.Status(context.Background())
				if status != nil {
					fmt.Println(status.Pretty())
				}

				if err == mirbft.ErrStopped {
					fmt.Printf("\n\nStopped normally!\n")
				} else {
					fmt.Printf("\n\nStopped abnormally with error: %s", err)
				}
				return
			}
			err := node.AddResults(*results)
			check(err)
		case <-node.Err():
			status, err := node.Status(context.Background())
			fmt.Printf("exited with error: %+v\n", err)
			fmt.Println(status.Pretty())
			return
		}
	}

}
