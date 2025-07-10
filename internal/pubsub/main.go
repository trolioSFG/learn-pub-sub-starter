package pubsub

import (
	"encoding/json"
	"context"
	amqp "github.com/rabbitmq/amqp091-go"
//	"fmt"
	"encoding/gob"
	"bytes"
)

type SimpleQueueType string
const (
	Transient SimpleQueueType = "transient"
	Durable SimpleQueueType = "durable"
)

type AckType int
const (
	Ack AckType = iota
	NackRequeue
	NackDiscard
)

func PublishJSON[T any](ch *amqp.Channel, exchange, key string, val T) error {
	jBytes, err := json.Marshal(val)
	if err != nil {
		return err
	}

	msg := amqp.Publishing {
		ContentType: "application/json",
		Body: jBytes,
	}

	err = ch.PublishWithContext(context.Background(), exchange, key, false, false, msg)
	if err != nil {
		return err
	}

	return nil
}

func DeclareAndBind(conn *amqp.Connection, exchange, queueName, key string, queueType SimpleQueueType) (*amqp.Channel, amqp.Queue, error) {
	ch, err := conn.Channel()
	if err != nil {
		return nil, amqp.Queue{}, err
	}

	q, err := ch.QueueDeclare(queueName, queueType == Durable,
		queueType != Durable,
		queueType != Durable,
		false,
		amqp.Table { "x-dead-letter-exchange": "peril_dlx", },
	)
	if err != nil {
		return nil, amqp.Queue{}, err
	}

	err = ch.QueueBind(queueName, key, exchange, false, nil)
	if err != nil {
		return nil, amqp.Queue{}, err
	}

	return ch, q, nil
}

func SubscribeJSON[T any](conn *amqp.Connection, exchange, queueName, key string,
    queueType SimpleQueueType, // an enum to represent "durable" or "transient"
    handler func(T) AckType) error {

	ch, q, err := DeclareAndBind(conn, exchange, queueName, key, queueType)
	if err != nil {
		return err
	}


	dlvry, err := ch.Consume(q.Name, "", false, false, false, false, nil)
	if err != nil {
		return err
	}

	go func() {
		var msg T
		for rawMsg := range dlvry {
			err := json.Unmarshal(rawMsg.Body, &msg)
			if err != nil {
				continue
			}
			at := handler(msg)
			switch at {
			case Ack:
				rawMsg.Ack(false)
				// fmt.Println("Ack")
			case NackRequeue:
				rawMsg.Nack(false, true)
				// fmt.Println("Nack Requeue")
			case NackDiscard:
				rawMsg.Nack(false, false)
				// fmt.Println("Nack Discard")
			}
		}
	}()

	return nil
}


func PublishGob[T any](ch *amqp.Channel, exchange, key string, val T) error {
	var network bytes.Buffer
	enc := gob.NewEncoder(&network)
	err := enc.Encode(val)

	msg := amqp.Publishing {
		ContentType: "application/gob",
		Body: network.Bytes(),
	}

	err = ch.PublishWithContext(context.Background(), exchange, key, false, false, msg)
	if err != nil {
		return err
	}

	return nil
}


func SubscribeGob[T any](conn *amqp.Connection, exchange, queueName, key string,
    queueType SimpleQueueType, // an enum to represent "durable" or "transient"
    handler func(T) AckType) error {

	ch, q, err := DeclareAndBind(conn, exchange, queueName, key, queueType)
	if err != nil {
		return err
	}

	// Limit prefetch
	ch.Qos(10, 0, false)

	dlvry, err := ch.Consume(q.Name, "", false, false, false, false, nil)
	if err != nil {
		return err
	}

	go func() {
		var stLog T 
		for rawMsg := range dlvry {
			buf := bytes.NewBuffer(rawMsg.Body)
			dec := gob.NewDecoder(buf)
			err := dec.Decode(&stLog) 
			if err != nil {
				continue
			}
			at := handler(stLog)
			switch at {
			case Ack:
				rawMsg.Ack(false)
				// fmt.Println("Ack")
			case NackRequeue:
				rawMsg.Nack(false, true)
				// fmt.Println("Nack Requeue")
			case NackDiscard:
				rawMsg.Nack(false, false)
				// fmt.Println("Nack Discard")
			}
		}
	}()

	return nil
}

