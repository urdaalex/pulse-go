package pulse

import (
	"code.google.com/p/go-uuid/uuid"
	"fmt"
	"github.com/streadway/amqp"
	"log"
)

func failOnError(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %s", msg, err)
		panic(fmt.Sprintf("%s: %s", msg, err))
	}
}

type pulseQueue struct {
}

// connection is not exported, so that a factory function must be used
// to create an instance, to control variable initialisation
type connection struct {
	User        string
	Password    string
	Url         string
	AMQPConn    *amqp.Connection
	connected   bool
	closedAlert chan amqp.Error
}

func (c *connection) SetURL(url string) {
	c.Url = url
}

// NewConnection returns a connection to the production instance (pulse.mozilla.org).
// In production, users and passwords can be self-managed by Pulse Guardian under
// https://pulse.mozilla.org/profile
func NewConnection(user string, password string) connection {
	return connection{
		User:     user,
		Password: password,
		Url:      "amqps://" + user + ":" + password + "@pulse.mozilla.org:5671"}
}

func (c *connection) connect() {
	var err error
	c.AMQPConn, err = amqp.Dial(c.Url)
	failOnError(err, "Failed to connect to RabbitMQ")
	c.connected = true
	// reconnect if drops
	// TODO: need to think through this logic
	// c.closedAlert = make(chan amqp.Error)
	// c.AMQPConn.NotifyClose(closedAlert)
	// go func(ch chan amqp.Error) {
	// 	for {
	// 		<-ch
	// 		connect()
	// 	}
	// }(c.closedAlert)
}

// Having a custom type
type Binding interface {
	RoutingKey() string
	ExchangeName() string
}

type simpleBinding struct {
	rk string
	en string
}

func Bind(routingKey, exchangeName string) *simpleBinding {
	return &simpleBinding{rk: routingKey, en: exchangeName}
}

func (s simpleBinding) RoutingKey() string {
	return s.rk
}

func (s simpleBinding) ExchangeName() string {
	return s.en
}

func (c *connection) Consume(
	queueName string,
	callback func(amqp.Delivery),
	prefetch int,
	maxLength int,
	bindings ...Binding) pulseQueue {

	if !c.connected {
		c.connect()
	}

	ch, err := c.AMQPConn.Channel()
	failOnError(err, "Failed to open a channel")
	defer ch.Close()

	for i := range bindings {
		err = ch.ExchangeDeclarePassive(
			bindings[i].ExchangeName(), // name
			"topic",                    // type
			false,                      // durable
			false,                      // auto-deleted
			false,                      // internal
			false,                      // no-wait
			nil,                        // arguments
		)
		failOnError(err, "Failed to passively declare exchange "+bindings[i].ExchangeName())
	}

	var q amqp.Queue
	if queueName == "" {
		q, err = ch.QueueDeclare(
			"queue/"+c.User+"/"+uuid.New(), // name
			false, // durable
			// unnamed queues get deleted when disconnected
			true, // delete when usused
			// unnamed queues are exclusive
			true,  // exclusive
			false, // no-wait
			nil,   // arguments
		)
	} else {
		q, err = ch.QueueDeclare(
			"queue/"+c.User+"/"+queueName, // name
			false, // durable
			false, // delete when usused
			false, // exclusive
			false, // no-wait
			nil,   // arguments
		)
	}
	failOnError(err, "Failed to declare queue")

	for i := range bindings {
		log.Printf("Binding queue %s to exchange %s with routing key %s", q.Name, bindings[i].ExchangeName(), bindings[i].RoutingKey())
		err = ch.QueueBind(
			q.Name, // queue name
			bindings[i].RoutingKey(),   // routing key
			bindings[i].ExchangeName(), // exchange
			false,
			nil)
		failOnError(err, "Failed to bind a queue")
	}

	eventsChan, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		true,   // auto ack
		false,  // exclusive
		false,  // no local
		false,  // no wait
		nil,    // args
	)
	failOnError(err, "Failed to register a consumer")

	go func() {
		for i := range eventsChan {
			// fmt.Println(string(i.Body))
			callback(i)
		}
	}()
	return pulseQueue{}
}

func (pq pulseQueue) Pause() {
}

func (pq pulseQueue) Delete() {
}

func (pq pulseQueue) Resume() {
}

func (pq pulseQueue) Close() {
}
