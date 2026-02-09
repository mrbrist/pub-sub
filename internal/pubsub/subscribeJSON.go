package pubsub

import (
	"encoding/json"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

type AckType int

const (
	Ack AckType = iota
	NackRequeue
	NackDiscard
)

func SubscribeJSON[T any](
	conn *amqp.Connection,
	exchange,
	queueName,
	key string,
	queueType SimpleQueueType, // an enum to represent "durable" or "transient"
	handler func(T) AckType,
) error {
	ch, _, err := DeclareAndBind(conn, exchange, queueName, key, queueType)
	if err != nil {
		return err
	}
	delCh, err := ch.Consume(queueName, "", false, false, false, false, nil)
	if err != nil {
		return err
	}

	go func() {
		for d := range delCh {
			var msg T
			if err := json.Unmarshal(d.Body, &msg); err != nil {
				_ = d.Nack(false, false)
				continue
			}

			switch handler(msg) {
			case Ack:
				fmt.Println("Ack")
				d.Ack(false)
			case NackRequeue:
				fmt.Println("NackRequeue")
				d.Nack(false, true)
			case NackDiscard:
				fmt.Println("NackDiscard")
				d.Nack(true, false)
			default:
				fmt.Println("Unknoqn AckType")
			}

			_ = d.Ack(false)
		}
	}()

	return nil
}
