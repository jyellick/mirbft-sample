/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package sample

import (
	"encoding/binary"
	"fmt"
	"math"
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

	nextReqNos := make([]uint64, len(c.ClientConfig.Nodes))

	lowestReqNo := uint64(math.MaxUint64)
	highestReqNo := uint64(0)

	// Get the nextReqNo according to everyone
	for i, node := range c.ClientConfig.Nodes {
		res, err := t.Request(node.ID, []byte{})
		if err != nil {
			fmt.Printf("Error fetching next request number: %s", err)
			return errors.WithMessage(err, "failed to submit client req")
		}

		reqNo := binary.BigEndian.Uint64(res)
		nextReqNos[i] = reqNo
		if reqNo < lowestReqNo {
			lowestReqNo = reqNo
		}
		if reqNo > highestReqNo {
			highestReqNo = reqNo
		}

	}

	targetReqNo := highestReqNo + requestCount - 1
	fmt.Printf("\nFound lowestReqNo=%d and highestReqNo=%d, will submit requests through targetReqNo=%d\n", lowestReqNo, highestReqNo, targetReqNo)

	for i := lowestReqNo; i <= targetReqNo; i++ {

		data := make([]byte, requestSize)
		for i, c := range fmt.Sprintf("data-%010d.%010d", c.ClientConfig.ID, i) {
			data[i] = byte(c)
		}

		req := &pb.Request{
			ClientId: c.ClientConfig.ID,
			ReqNo:    i,
			Data:     data,
		}

		for j, node := range c.ClientConfig.Nodes {
			if nextReqNos[j] > i {
				continue
			}
			if nextReqNos[j] == i {
				fmt.Printf("  starting to send reqs to node %d at reqNo=%d\n", j, i)
			}
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
