package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/IBM/mirbft"
	pb "github.com/IBM/mirbft/mirbftpb"
	"github.com/IBM/mirbft/recorder"
	"github.com/IBM/mirbft/reqstore"
	"github.com/IBM/mirbft/simplewal"
	"github.com/guoger/mir-sample/network"
	"github.com/pkg/errors"
	"gopkg.in/alecthomas/kingpin.v2"
)

type arguments struct {
	cryptoConfig       *os.File
	runDir             string
	eventLog           string
	bucketCount        uint
	batchSize          uint
	checkpointInterval uint
	clientCount        uint
	msgsPerClient      uint
}

func parseArgs(argsString []string) (*arguments, error) {
	app := kingpin.New("mirbft-sample", "A small sample application implemented using the mirbft library.")
	cryptoConfigFile := app.Flag("cryptoConfig", "The YAML file containing this node's crypto config (as generated via cryptogen).").Required().File()
	eventLogFile := app.Flag("eventLog", "A path to a location to write the state event log for this invocation.").String()
	bucketCount := app.Flag("bucketCount", "The number of buckets for the network configuration (max possible parallel leaders).").Default("1").Uint()
	batchSize := app.Flag("batchSize ", "The number of requests to batch per sequence number.").Default("20").Uint()
	checkpointInterval := app.Flag("checkpointInterval", "The checkpoint interval for network configuration (how often the application is asked to snapshot).").Default("20").Uint()
	clientCount := app.Flag("clientCount", "The number of clients initially configured.").Default("1").Uint()
	msgsPerClient := app.Flag("msgsPerClient", "The number of messages each client should send.").Default("1000").Uint()

	_, err := app.Parse(argsString)
	if err != nil {
		return nil, err
	}

	return &arguments{
		cryptoConfig:       *cryptoConfigFile,
		eventLog:           *eventLogFile,
		batchSize:          *batchSize,
		bucketCount:        *bucketCount,
		checkpointInterval: *checkpointInterval,
		clientCount:        *clientCount,
		msgsPerClient:      *msgsPerClient,
	}, nil

}

type application struct {
	mutex   sync.Mutex
	wg      sync.WaitGroup
	doneC   chan struct{}
	args    *arguments
	server  *server
	clients []*client
}

type client struct {
	id       uint64
	msgCount uint
}

func (a *arguments) initializeApp() (*application, error) {
	config, err := network.LoadConfig(a.cryptoConfig)
	if err != nil {
		return nil, errors.WithMessage(err, "failed loading crypto config")
	}

	clientIDs := make([]uint64, int(a.clientCount))
	for i := range clientIDs {
		clientIDs[i] = uint64(i)
	}

	// bootstrap network state
	networkState := mirbft.StandardInitialNetworkState(len(config.Peers), clientIDs...)
	networkState.Config.NumberOfBuckets = int32(a.bucketCount)
	networkState.Config.CheckpointInterval = int32(a.checkpointInterval)
	for _, client := range networkState.Clients {
		client.Width = 5000
	}

	doneC := make(chan struct{})

	// If an eventLog is configured, configure a recording interceptor for the server
	// to log the state machine events to this log file, note, this is generally
	// not required or desirable for production apps, but, is nice for debugging
	// and visualization.
	var recording *recorder.Interceptor
	if a.eventLog != "" {
		startTime := time.Now()
		recording = recorder.NewInterceptor(
			config.ID,
			func() int64 {
				return time.Since(startTime).Milliseconds()
			},
			10000,
			doneC,
		)
	}

	serverConfig := &serverConfig{
		batchSize: uint32(a.batchSize),
		storage:   mockStorage(networkState),
		recording: recording,
		log: &applicationLog{
			targetCount:     a.clientCount * a.msgsPerClient,
			applicationDone: make(chan struct{}),
		},
		transportConfig: config,
		doneC:           doneC,
	}

	fmt.Printf("Setting target count to: %d\n", serverConfig.log.targetCount)

	server, err := serverConfig.initialize()
	if err != nil {
		return nil, errors.WithMessage(err, "could not initialize server")
	}

	clients := make([]*client, len(networkState.Clients))
	for i, c := range networkState.Clients {
		clients[i] = &client{
			id:       c.Id,
			msgCount: a.msgsPerClient,
		}
	}

	return &application{
		args:    a,
		server:  server,
		clients: clients,
		doneC:   doneC,
	}, nil
}

func (a *application) runClients(node *mirbft.Node) {
	a.wg.Add(len(a.clients))
	for _, c := range a.clients {
		go func(client *client) {
			// Note, the blocking calls into node.ClientProposer and
			// into proposer.Propose will unblock when doneC closes,
			// as the server will exit and cause these to return.
			defer func() {
				fmt.Printf("Client %d go routine exiting\n", client.id)
				a.wg.Done()
			}()
			proposer, err := node.ClientProposer(context.Background(), 0)
			if err != nil {
				fmt.Printf("ERROR: Client %d failed to get proposer: %s\n", client.id, err)
				return
			}

			for i := uint(0); i < client.msgCount; i++ {
				// fmt.Printf("Proposing %d\n", i)
				req := &pb.Request{
					ClientId: client.id,
					ReqNo:    uint64(i),
					Data:     []byte(fmt.Sprintf("application-data-%d-%d", client.id, i)),
				}

				err = proposer.Propose(context.Background(), req)
				if err != nil {
					fmt.Printf("ERROR: Client %d failed to propose request %d: %s\n", client.id, i, err)
					return
				}
			}
		}(c)
	}
}

func (a *application) drainRecorder() error {
	if a.server.config.recording == nil {
		return nil
	}

	file, err := os.Create(a.args.eventLog)
	if err != nil {
		return err
	}
	a.wg.Add(1)

	go func() {
		// Note, the recording took a.doneC as
		// a parameter at creation time and will
		// exit when it closes.

		defer file.Close()
		defer a.wg.Done()
		a.server.config.recording.Drain(file)
		fmt.Printf("Recorder go routine exiting\n")
	}()

	return nil
}

func (a *application) run() error {
	// Each of these calls spawns zero or more go routines
	// and adds them to the wait group a.wg.  Each of these
	// go routines will eventually exit once a.doneC closes.
	a.runClients(a.server.node)
	err := a.drainRecorder()
	if err != nil {
		return errors.WithMessage(err, "could not begin draining the recorder")
	}

	// This call triggers the main server loop which will execute
	// until the application log finishes, or an error occurs
	err = a.server.run()
	if err != nil {
		return errors.WithMessage(err, "server exited abnormally")
	}

	return nil
}

// stop may be called by the main thread, or by the signal handler
// so we ues a mutex to prevent double closure of the doneC
func (a *application) stop() {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	select {
	case <-a.doneC:
	default:
		close(a.doneC)
	}
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

func handleSignals(doneC <-chan struct{}, stop func()) {
	sigC := make(chan os.Signal, 1)

	signal.Notify(sigC, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		select {
		case sig := <-sigC:
			fmt.Printf("Caught signal, exiting: %v\n", sig)
			stop()
		case <-doneC:
		}
	}()
}

func main() {
	kingpin.Version("0.0.1")
	args, err := parseArgs(os.Args[1:])
	if err != nil {
		kingpin.Fatalf("Error parsing arguments, %s, try --help", err)
	}

	application, err := args.initializeApp()
	if err != nil {
		kingpin.Fatalf("Error initializing app, %s", err)
	}

	handleSignals(application.doneC, application.stop)

	err = application.run()
	application.stop()
	application.wg.Wait()
	if err != nil {
		kingpin.Fatalf("Application exited abnormally, %s", err)
	}

	fmt.Printf("Success! All worker go routines exited, terminating!\n")
}
