package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	sample "github.com/jyellick/mirbft-sample"
	"github.com/jyellick/mirbft-sample/config"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"gopkg.in/alecthomas/kingpin.v2"
)

type args struct {
	nodeConfig *os.File
	runDir     string
	eventLog   bool
	parallel   bool
}

func parseArgs(argsString []string) (*args, error) {
	app := kingpin.New("mirbft-sample", "A small sample application implemented using the mirbft library.")
	nodeConfig := app.Flag("nodeConfig", "The YAML file containing this node's config (as generated via bootstrap).").Required().File()
	runDir := app.Flag("runDir", "A path to a location to write the WAL, RequestStore, and EventLog.").ExistingDir()
	eventLog := app.Flag("eventLog", "Whether the node should record a state machine event log").Default("false").Bool()
	parallel := app.Flag("parallel ", "Whether the node should process actions in parallel or in series.").Default("true").Bool()

	_, err := app.Parse(argsString)
	if err != nil {
		return nil, err
	}

	return &args{
		nodeConfig: *nodeConfig,
		runDir:     *runDir,
		eventLog:   *eventLog,
		parallel:   *parallel,
	}, nil

}

func handleSignals(stop func()) {
	sigC := make(chan os.Signal, 1)

	signal.Notify(sigC, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigC
	fmt.Printf("Caught signal, exiting: %v\n", sig)
	stop()
}

func (a *args) initializeServer() (*sample.Server, error) {
	nodeConfig, err := config.LoadNodeConfig(a.nodeConfig)
	if err != nil {
		return nil, errors.WithMessage(err, "could not parse node config")
	}

	walDir := filepath.Join(a.runDir, "WAL")
	reqStoreDir := filepath.Join(a.runDir, "reqStore")
	var eventLogPath string
	if a.eventLog {
		eventLogPath = filepath.Join(a.runDir, "eventlog.gz")
	}

	return &sample.Server{
		Logger:           zap.NewExample(),
		NodeConfig:       nodeConfig,
		Parallel:         a.parallel,
		EventLogPath:     eventLogPath,
		WALPath:          walDir,
		RequestStorePath: reqStoreDir,
	}, nil
}

func main() {
	kingpin.Version("0.0.1")
	args, err := parseArgs(os.Args[1:])
	if err != nil {
		kingpin.Fatalf("Error parsing arguments, %s, try --help", err)
	}

	server, err := args.initializeServer()
	if err != nil {
		kingpin.Fatalf("Error initializing server, %s", err)
	}

	err = server.Run()
	if err != nil {
		kingpin.Fatalf("Application exited abnormally, %s", err)
	}

	handleSignals(server.Stop)

	fmt.Printf("Success! All worker go routines exited, terminating!\n")
}
