/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package main

import (
	"fmt"
	"os"

	sample "github.com/jyellick/mirbft-sample"
	"github.com/jyellick/mirbft-sample/config"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"gopkg.in/alecthomas/kingpin.v2"
)

type args struct {
	clientConfig *os.File
	requestCount uint64
	requestSize  uint16
}

func parseArgs(argsString []string) (*args, error) {
	app := kingpin.New("client", "A small sample client for the mirbft-sample application.")
	clientConfig := app.Flag("clientConfig", "The YAML file containing this client's config (as generated via bootstrap).").Required().File()
	requestCount := app.Flag("requestCount", "The total number of requests to send").Default("10000").Uint64()
	requestSize := app.Flag("requestSize", "The size in bytes for each request (must be at least 26 bytes)").Default("10240").Uint16()

	_, err := app.Parse(argsString)
	if err != nil {
		return nil, err
	}

	return &args{
		clientConfig: *clientConfig,
		requestCount: *requestCount,
		requestSize:  *requestSize,
	}, nil

}

func (a *args) initializeClient() (*sample.Client, error) {
	clientConfig, err := config.LoadClientConfig(a.clientConfig)
	if err != nil {
		return nil, errors.WithMessage(err, "could not parse client config")
	}

	return &sample.Client{
		Logger:       zap.NewExample().Sugar(),
		ClientConfig: clientConfig,
	}, nil
}

func main() {
	kingpin.Version("0.0.1")
	args, err := parseArgs(os.Args[1:])
	if err != nil {
		kingpin.Fatalf("Error parsing arguments, %s, try --help", err)
	}

	client, err := args.initializeClient()
	if err != nil {
		kingpin.Fatalf("Error initializing client, %s", err)
	}

	err = client.Run(args.requestCount, args.requestSize)
	if err != nil {
		kingpin.Fatalf("Client exited abnormally, %s", err)
	}

	fmt.Printf("Success! All worker go routines exited, terminating!\n")
}
