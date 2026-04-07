package proxy

import (
	"context"
	"github.com/datastax/go-cassandra-native-protocol/frame"
	"github.com/datastax/go-cassandra-native-protocol/message"
)

func (c *client) handleOptions(ctx context.Context, raw *frame.RawFrame, msg *message.Options) (message.Message, error) {
	// CC - responding with status READY
	return &message.Supported{Options: map[string][]string{
		"CQL_VERSION": {c.proxy.config.Options.CQLVersion},
		"COMPRESSION": {},
	}}, nil
}
