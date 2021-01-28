/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package sample

import (
	"bytes"
	"fmt"
	"time"

	pb "github.com/IBM/mirbft/mirbftpb"
	"github.com/jyellick/mirbft-sample/config"
	"github.com/jyellick/mirbft-sample/network"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type Client struct {
	Logger       *zap.SugaredLogger
	ClientConfig *config.ClientConfig
}

func (c *Client) Run() error {
	// Create transport
	t, err := network.NewClientTransport(c.Logger, c.ClientConfig)
	if err != nil {
		return errors.WithMessage(err, "could not create networking")
	}

	err = t.Start()
	if err != nil {
		return errors.WithMessage(err, "could not start networking")
	}
	defer t.Close()

	start := time.Now()
	for i := 0; i < 5000; i++ {

		data := make([]byte, 100*1024)
		fmt.Fprintf(bytes.NewBuffer(data), "my-request-%d.%d.data", c.ClientConfig.ID, i)

		req := &pb.Request{
			ClientId: c.ClientConfig.ID,
			ReqNo:    uint64(i),
			Data:     data,
		}

		for _, node := range c.ClientConfig.Nodes {
			t.Send(node.ID, req)
		}
	}
	fmt.Printf("\n\nCompleted in %v\n\n", time.Since(start))

	// XXX the close does not seem to wait until sends complete, investigate.
	time.Sleep(2 * time.Second)

	return nil
}
