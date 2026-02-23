package pubsub

import (
	"bytes"
	"encoding/gob"
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

func subscribe[T any](
	conn *amqp.Connection,
	exchange,
	queueName,
	key string,
	simpleQueueType SimpleQueueType,
	handler func(T) AckType,
	unmarshaller func([]byte) (T, error),
) error {
	ch, _, err := DeclareAndBind(conn, exchange, queueName, key, simpleQueueType)
	if err != nil {
		return err
	}
	delCh, err := ch.Consume(queueName, "", false, false, false, false, nil)
	if err != nil {
		return err
	}

	go func() {
		for d := range delCh {
			msg, err := unmarshaller(d.Body)
			if err != nil {
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
				fmt.Println("Unknown AckType")
			}
		}
	}()

	return nil
}

func SubscribeJSON[T any](
	conn *amqp.Connection,
	exchange,
	queueName,
	key string,
	queueType SimpleQueueType, // an enum to represent "durable" or "transient"
	handler func(T) AckType,
) error {
	unmarshaller := func(data []byte) (T, error) {
		var v T
		err := json.Unmarshal(data, &v)
		return v, err
	}

	err := subscribe(conn, exchange, queueName, key, queueType, handler, unmarshaller)
	if err != nil {
		return err
	}
	return nil
}

func SubscribeGob[T any](
	conn *amqp.Connection,
	exchange,
	queueName,
	key string,
	queueType SimpleQueueType, // an enum to represent "durable" or "transient"
	handler func(T) AckType,
) error {
	unmarshaller := func(data []byte) (T, error) {
		var v T

		buffer := bytes.NewBuffer(data)
		decoder := gob.NewDecoder(buffer)

		err := decoder.Decode(&v)
		return v, err
	}

	err := subscribe(conn, exchange, queueName, key, queueType, handler, unmarshaller)
	if err != nil {
		return err
	}
	return nil
}
