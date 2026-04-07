package proxy

import (
	"context"
	"github.com/datastax/go-cassandra-native-protocol/frame"
	"github.com/datastax/go-cassandra-native-protocol/message"
	"github.com/datastax/go-cassandra-native-protocol/primitive"
	"go.uber.org/zap"
)

func (c *client) handleRegister(ctx context.Context, raw *frame.RawFrame, msg *message.Register) (message.Message, error) {
	c.proxy.logger.Info("Client registered for events", zap.Any("event_types", msg.EventTypes))
	for _, t := range msg.EventTypes {
		if t == primitive.EventTypeSchemaChange {
			c.proxy.registerForEvents(c)
		}
	}
	return &message.Ready{}, nil
}
