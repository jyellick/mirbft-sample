/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package network

import (
	"context"
	"encoding/hex"
	"fmt"
	"net"
	"strconv"

	pb "github.com/hyperledger-labs/mirbft/pkg/pb/msgs"
	"github.com/jyellick/mirbft-sample/config"
	"github.com/perlin-network/noise"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

type ClientTransport struct {
	logger *zap.SugaredLogger

	id      uint64
	id2addr map[uint64]string

	node *noise.Node
}

func NewClientTransport(logger *zap.SugaredLogger, config *config.ClientConfig) (*ClientTransport, error) {
	id2addr := make(map[uint64]string)
	pubkey2nodeid := make(map[noise.PublicKey]uint64)
	for _, p := range config.Nodes {
		pubkeyBytes, err := hex.DecodeString(p.PublicKey)
		if err != nil {
			return nil, err
		}

		var pubkey noise.PublicKey
		copy(pubkey[:], pubkeyBytes)

		id2addr[p.ID] = p.Address
		pubkey2nodeid[pubkey] = p.ID
		fmt.Printf("Adding mapping from %x to node %d\n", pubkeyBytes, p.ID)
	}

	key, err := hex.DecodeString(config.PrivateKey)
	if err != nil {
		return nil, err
	}
	var privkey noise.PrivateKey
	copy(privkey[:], key)

	node, err := noise.NewNode(
		noise.WithNodePrivateKey(privkey),
		noise.WithNodeLogger(logger.Named("noise").Desugar()),
	)
	if err != nil {
		return nil, err
	}

	return &ClientTransport{
		logger:  logger,
		id:      config.ID,
		id2addr: id2addr,
		node:    node,
	}, nil
}

func (t *ClientTransport) Start() error {
	t.logger.Infof("Start listening on %s...", t.node.Addr())
	return t.node.Listen()
}

func (t *ClientTransport) Close() {
	t.logger.Infof("Closing transport")
	t.node.Close()
}

func (t *ClientTransport) Request(dest uint64, data []byte) ([]byte, error) {
	addr, ok := t.id2addr[dest]
	if !ok {
		panic("Unknown remote")
	}

	return t.node.Request(context.TODO(), addr, data)
}

func (t *ClientTransport) Send(dest uint64, msg *pb.Request) error {
	data, err := proto.Marshal(msg)
	if err != nil {
		panic("Failed to marshal outbound message")
	}

	addr, ok := t.id2addr[dest]
	if !ok {
		panic("Unknown remote")
	}

	return t.node.Send(context.TODO(), addr, data)
}

type ServerTransport struct {
	logger *zap.SugaredLogger

	id              uint64
	id2addr         map[uint64]string
	pubkey2nodeid   map[noise.PublicKey]uint64
	pubkey2clientid map[noise.PublicKey]uint64

	nodeHandler Handler
	node        *noise.Node
}

type Handler func(id uint64, data []byte) ([]byte, error)

func NewServerTransport(logger *zap.SugaredLogger, config *config.NodeConfig) (*ServerTransport, error) {
	id2addr := make(map[uint64]string)
	pubkey2nodeid := make(map[noise.PublicKey]uint64)
	for _, p := range config.Nodes {
		pubkeyBytes, err := hex.DecodeString(p.PublicKey)
		if err != nil {
			return nil, err
		}

		var pubkey noise.PublicKey
		copy(pubkey[:], pubkeyBytes)

		id2addr[p.ID] = p.Address
		pubkey2nodeid[pubkey] = p.ID
		fmt.Printf("Adding mapping from %x to node %d\n", pubkeyBytes, p.ID)
	}

	pubkey2clientid := make(map[noise.PublicKey]uint64)
	for _, c := range config.Clients {
		pubkeyBytes, err := hex.DecodeString(c.PublicKey)
		if err != nil {
			return nil, err
		}

		var pubkey noise.PublicKey
		copy(pubkey[:], pubkeyBytes)

		pubkey2clientid[pubkey] = c.ID
		fmt.Printf("Adding mapping from %x to client %d\n", pubkeyBytes, c.ID)
	}

	key, err := hex.DecodeString(config.PrivateKey)
	if err != nil {
		return nil, err
	}
	var privkey noise.PrivateKey
	copy(privkey[:], key)

	addr, port, err := net.SplitHostPort(config.ListenAddress)
	if err != nil {
		return nil, err
	}

	ip := net.ParseIP(addr)

	p, err := strconv.ParseInt(port, 0, 16)
	if err != nil {
		return nil, err
	}

	id := noise.ID{
		ID:   privkey.Public(),
		Host: ip,
		Port: uint16(p),
	}
	node, err := noise.NewNode(
		noise.WithNodePrivateKey(privkey),
		noise.WithNodeID(id),
		noise.WithNodeLogger(logger.Named("noise").Desugar()),
		noise.WithNodeBindPort(uint16(p)),
	)
	if err != nil {
		return nil, err
	}

	return &ServerTransport{
		logger:          logger,
		id:              config.ID,
		id2addr:         id2addr,
		pubkey2nodeid:   pubkey2nodeid,
		pubkey2clientid: pubkey2clientid,
		node:            node,
	}, nil
}

func (t *ServerTransport) Handle(nodeHandler, clientHandler Handler) {
	t.node.Handle(func(ctx noise.HandlerContext) error {
		nodeID, ok := t.pubkey2nodeid[ctx.ID().ID]
		if ok {
			result, err := nodeHandler(nodeID, ctx.Data())
			if err == nil && ctx.IsRequest() {
				return ctx.Send(result)
			}
			return err
		}

		clientID, ok := t.pubkey2clientid[ctx.ID().ID]
		if ok {
			result, err := clientHandler(clientID, ctx.Data())
			if err == nil && ctx.IsRequest() {
				return ctx.Send(result)
			}
			return err
		}

		t.logger.Warnf("Unknown remote: %+v", ctx.ID())
		return errors.Errorf("unknown node or client")
	})
	t.nodeHandler = nodeHandler
}

func (t *ServerTransport) Start() error {
	t.logger.Infof("Start listening on %s...", t.node.Addr())
	return t.node.Listen()
}

func (t *ServerTransport) Close() {
	t.logger.Infof("Closing transport")
	t.node.Close()
}

func (t *ServerTransport) Send(dest uint64, msg *pb.Msg) {
	data, err := proto.Marshal(msg)
	if err != nil {
		panic("Failed to marshal outbound message")
	}

	// local message, we should never hit this case, but handling anyway
	if dest == t.id {
		t.nodeHandler(dest, data)
		return
	}

	addr, ok := t.id2addr[dest]
	if !ok {
		panic("Unknown remote")
	}

	err = t.node.Send(context.TODO(), addr, data)
	if err != nil {
		t.logger.Warnf("Failed to send to %s: %s", addr, err)
	}
}
