// Package pulse provides operations for interacting with
// https://pulse.mozilla.org/.  Currently it supports consuming messages, not
// publishing messages. Please note publishing messages is entirely possible
// using package "github.com/streadway/amqp" directly, and indeed this library
// is built on top of the amqp package. If a user so wishes, they can also
// consume pulse messages programming directly with the amqp package too.
// However, for a user that is simply interesting in processing pulse messages
// without wishing to acquire a detailed understanding of how pulse.mozilla.org
// has been designed, or how AMQP 0.9.1 works, this client provides basic
// utility methods to get you started off quickly.
//
// Please note that parent package "github.com/petemoore/pulse-go" provides a
// very simple command line interface into this library too, which can be
// called directly from a shell, for example, so that the user requires no go
// programming expertise.
//
// To get started, let's create an example go program which uses this library.
// The first thing we need to do is establish a connection to pulse which we do
// like this:
//
//  conn := pulse.NewConnection("myuser", "mypassword", "")
//
// This does not create an actual connection to the pulse server, only prepares
// the data that will be needed when we finally make the connection.  Users and
// passwords can be created by going to https://pulse.mozilla.org and
// registering an account.  Please note the third string passed to
// NewConnection is the amqp url to use to connect to the pulse server. If you
// pass an empty string, it will default to using production. However if you
// have a different instance to connect to, such as a test, dev or staging
// instance of pulse, you may provide the url for that instance instead.
//
// So now we have created a connection, we should create a new queue, or
// connect to an existing one, and bind it to one or more exchanges with a set
// of exchange key / routing key pairs.  This sounds rather complex, but in
// fact is quite trivial.
//
// In pulse, all messages are delivered to "topic exchanges" and the way to
// receive these messages is to get the ones you are interested in copied onto
// a queue you can read from, and then to read them from the queue. This is
// called binding. To bind messages from an exchange to a queue, you specify
// the name of the exchange you want to receive messages from, and a matching
// criteria to select just those you want. This matching criteria consists of a
// '.' delimited string such as *.*.*.my-little-pony.*.*.* which allows you to
// match all messages where the 4th field in the message "routing key" is
// "my-little-pony". # is the same as * but matches dots too, so
// *.*.*.my-little-pony.# would also match in this case.
//
//  package main
//
//  import (
//  	"github.com/petemoore/pulse-go/pulse"
//  )
//
//  func main() {
//  	conn := pulse.NewConnection("myuser", "mypassword", "")
//  }
package pulse

import (
	"code.google.com/p/go-uuid/uuid"
	"fmt"
	"github.com/streadway/amqp"
	"log"
	"os"
	"regexp"
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
	URL         string
	AMQPConn    *amqp.Connection
	connected   bool
	closedAlert chan amqp.Error
}

func match(regex, text string) string {
	if matched, _ := regexp.MatchString(regex, text); matched {
		return regexp.MustCompile(regex).ReplaceAllString(text, "$1")
	}
	return ""
}

// NewConnection returns a connection to the production instance (pulse.mozilla.org).
// In production, users and passwords can be self-managed by Pulse Guardian under
// https://pulse.mozilla.org/profile
// To use a non-production environment, call pulse.SetURL(<alternative_url>) after
// calling NewConnection. Please note, creating the connection does not cause any
// network traffic, the connection is only established when calling Consume function.
func NewConnection(pulseUser string, pulsePassword string, amqpUrl string) connection {
	if amqpUrl == "" {
		amqpUrl = "amqps://pulse.mozilla.org:5671"
	}
	if pulseUser == "" {
		// Regular expression to pull out username from amqp url
		pulseUser = match("^.*://([^:@/]*)(:[^@]*@|@).*$", amqpUrl)
	}
	if pulsePassword == "" {
		// Regular expression to pull out password from amqp url
		pulsePassword = match("^.*://[^:@/]*:([^@]*)@.*$", amqpUrl)
	}
	if pulseUser == "" {
		pulseUser = os.Getenv("PULSE_USERNAME")
	}
	if pulsePassword == "" {
		pulsePassword = os.Getenv("PULSE_PASSWORD")
	}
	if pulseUser == "" {
		pulseUser = "guest"
	}
	if pulsePassword == "" {
		pulsePassword = "guest"
	}

	// now substitute in real username and password into url...
	amqpUrl = regexp.MustCompile("^(.*://)([^@/]*@|)([^@]*)(/.*|$)").ReplaceAllString(amqpUrl, "${1}"+pulseUser+":"+pulsePassword+"@${3}${4}")

	return connection{
		User:     pulseUser,
		Password: pulsePassword,
		URL:      amqpUrl}
}

func (c *connection) connect() {
	var err error
	c.AMQPConn, err = amqp.Dial(c.URL)
	failOnError(err, "Failed to connect to RabbitMQ")
	c.connected = true
}

// Binding interface allows you to create custom types to describe exchange / routing key
// combinations, for example Binding types are generated in Task Cluster go client to
// avoid a library user referencing a non existent exchange, or an invalid routing key.
type Binding interface {
	RoutingKey() string
	ExchangeName() string
}

// Convenience private (unexported) type for binding a routing key/exchange
// to a queue using plain strings for describing the exchange and routing key
type simpleBinding struct {
	rk string
	en string
}

// Convenience function for returning a Binding for the given routing key and exchange
// strings, which can be passed to the Consume method of *connection.
// Typically this is used if you wish to refer to exchanges and routing keys with
// explicit strings, rather than generated types (e.g. Task Cluster go client
// generates custom types to avoid invalid exchange names or invalid routing keys).
func Bind(routingKey, exchangeName string) *simpleBinding {
	return &simpleBinding{rk: routingKey, en: exchangeName}
}

// simpleBindings blindly return the routing key they were passed without validation
func (s simpleBinding) RoutingKey() string {
	return s.rk
}

// simpleBindings blindly return the exchange name they were passed without validation
func (s simpleBinding) ExchangeName() string {
	return s.en
}

func (c *connection) Consume(
	queueName string,
	callback func(amqp.Delivery),
	prefetch int,
	maxLength int,
	autoAck bool,
	bindings ...Binding) pulseQueue {

	if !c.connected {
		c.connect()
	}

	ch, err := c.AMQPConn.Channel()
	failOnError(err, "Failed to open a channel")

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
		log.Printf("Binding %s to %s with routing key %s", q.Name, bindings[i].ExchangeName(), bindings[i].RoutingKey())
		err = ch.QueueBind(
			q.Name, // queue name
			bindings[i].RoutingKey(),   // routing key
			bindings[i].ExchangeName(), // exchange
			false,
			nil)
		failOnError(err, "Failed to bind a queue")
	}

	eventsChan, err := ch.Consume(
		q.Name,  // queue
		"",      // consumer
		autoAck, // auto ack
		false,   // exclusive
		false,   // no local
		false,   // no wait
		nil,     // args
	)
	failOnError(err, "Failed to register a consumer")

	go func() {
		for i := range eventsChan {
			// fmt.Println(string(i.Body))
			callback(i)
		}
		fmt.Println("Seem to have exited events loop?!!!")
	}()
	return pulseQueue{}
}

// TODO: not yet implemented
func (pq *pulseQueue) Pause() {
}

// TODO: not yet implemented
func (pq *pulseQueue) Delete() {
}

// TODO: not yet implemented
func (pq *pulseQueue) Resume() {
}

// TODO: not yet implemented
func (pq *pulseQueue) Close() {
}
