/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package sample

import (
	"fmt"
	"time"

	pb "github.com/hyperledger-labs/mirbft/pkg/pb/msgs"
	"github.com/jyellick/mirbft-sample/config"
	"github.com/jyellick/mirbft-sample/network"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type Client struct {
	Logger       *zap.SugaredLogger
	ClientConfig *config.ClientConfig
}

func (c *Client) Run(requestCount uint64, requestSize uint16) error {
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
	for i := uint64(0); i < requestCount; i++ {

		data := make([]byte, requestSize)
		for i, c := range fmt.Sprintf("data-%010d.%010d", c.ClientConfig.ID, i) {
			data[i] = byte(c)
		}

		req := &pb.Request{
			ClientId: c.ClientConfig.ID,
			ReqNo:    i,
			Data:     data,
		}

		for _, node := range c.ClientConfig.Nodes {
			err := t.Send(node.ID, req)
			if err != nil {
				fmt.Printf("Error sending client request %d: %s", i, err)
				return errors.WithMessage(err, "failed to submit client req")
			}
		}
	}
	fmt.Printf("\n\nCompleted in %v\n\n", time.Since(start))

	// XXX the close does not seem to wait until sends complete, investigate.
	time.Sleep(2 * time.Second)

	return nil
}
