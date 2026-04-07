package proxy

import (
	"context"
	"github.com/datastax/go-cassandra-native-protocol/frame"
	"github.com/datastax/go-cassandra-native-protocol/message"
)

func (c *client) handleStartup(ctx context.Context, raw *frame.RawFrame, msg *message.Startup) (message.Message, error) {
	// CC -  register for Event types and respond READY
	return &message.Ready{}, nil
}
