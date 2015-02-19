// Package pulse provides operations for interacting with
// https://pulse.mozilla.org/.  Currently it supports consuming messages, not
// publishing messages. Please note publishing messages is entirely possible
// using package "github.com/streadway/amqp" directly, and indeed this pulse
// library is built on top of the amqp package. If a user so wishes, they can
// also consume pulse messages programming directly with the amqp package too.
// However, for a user that is simply interesting in processing pulse messages
// without wishing to acquire a detailed understanding of how pulse.mozilla.org
// has been designed, or how AMQP 0.9.1 works, this client provides basic
// utility methods to get you started off quickly.
//
// Please note that parent package "github.com/petemoore/pulse-go" provides a
// very simple command line interface into this library too, which can be
// called directly from a shell, for example, so that the user requires no go
// programming expertise, and can directly write e.g. shell scripts that
// process pulse messages.
//
// To get started, let's create an example go program which uses this library.
// Afterwards we will see how it works. Do not worry if none of it makes sense
// now. By the end of this overview it will all be explained.
//
//  package main
//
//  import (
//  	"fmt"
//  	"github.com/petemoore/pulse-go/pulse"
//  	"github.com/streadway/amqp"
//  )
//
//  func main() {
//  	// Passing all empty strings:
//  	// empty user => use PULSE_USERNAME env var
//  	// empty password => use PULSE_PASSWORD env var
//  	// empty url => connect to production
//  	conn := pulse.NewConnection("", "", "")
//  	conn.Consume(
//  		"taskprocessing", // queue name
//  		func(delivery amqp.Delivery) { // callback function to pass messages to
//  			fmt.Println("Received from exchange " + delivery.Exchange + ":")
//  			fmt.Println(string(delivery.Body))
//  			fmt.Println("")
//  			delivery.Ack(false) // acknowledge message *after* processing
//  		},
//  		1,     // prefetch 1 message at a time
//  		false, // don't auto-acknowledge messages
//  		pulse.Bind( // routing key and exchange to get messages from
//  			"*.*.*.*.*.*.gaia.#",
//  			"exchange/taskcluster-queue/v1/task-defined"),
//  		pulse.Bind( // another routing key and exchange to get messages from
//  			"*.*.*.*.*.aws-provisioner.#",
//  			"exchange/taskcluster-queue/v1/task-running"))
//
//  	conn.Consume( // a second workflow to manage concurrently
//  		"", // empty name implies anonymous queue
//  		func(delivery amqp.Delivery) { // simpler callback than before
//  			fmt.Println("Buildbot message received")
//  			fmt.Println("")
//  		},
//  		1,    // prefetch
//  		true, // auto acknowledge, so no need to call delivery.Ack
//  		pulse.Bind( // routing key and exchange to get messages from
//  			"#", // get *all* normalized buildbot messages
//  			"exchange/build/normalized"))
//
//  	// wait forever
//  	forever := make(chan bool)
//  	<-forever
//  }
// The first thing we need to do is provide connection details for connecting
// to the pulse server, which we do like this:
//
//  conn := pulse.NewConnection("", "", "")
//
// In this example, the provided strings (username, password, url) have all
// been left empty. This is because by default, if you provide no username or
// password, the NewConnection function will inspect environment variables
// PULSE_USERNAME and PULSE_PASSWORD, and an empty url will trigger the library
// to use the current production url. Another example call could be:
//
//  conn := pulse.NewConnection("guest", "guest", "amqp://localhost:5672/")
//
// Typically we would set the username and password credentials via environment
// variables to avoid hardcoding them in the go code.  For more details about
// managing the username, password and amqp url, see the documentation for the
// NewConnection function.
//
// A call to NewConnection does not actually create a connection to the pulse
// server, it simply prepares the data that will be needed when we finally make
// the connection.  Users and passwords can be created by going to the Pulse
// Guardian (https://pulse.mozilla.org) and registering an account.
//
// You will see in the code above, that after creating a connection, there is
// only one more method we call - Consume - which we use for processing
// messages. This is the heart of the pulse library, and where all of the
// action happens.
//
// In pulse, all messages are delivered to "topic exchanges" and the way to
// receive these messages is to request the ones you are interested in are
// copied onto a queue you can read from, and then to read them from the queue.
// This is called binding. To bind messages from an exchange to a queue, you
// specify the name of the exchange you want to receive messages from, and a
// matching criteria to define the ones you want. The matching process is
// handled by routing keys, which will now be explained.
//
// Each message that arrives on an exchange has a "routing key" signature.  The
// routing key comprises of several fields. For an example, see:
// http://docs.taskcluster.net/queue/exchanges/#taskDefined.  The fields are
// delimited by dots, and therefore the routing key of a message is represented
// as a '.' delimited string.  In order to select the messages on an exchange
// that you wish to receive, you specify a matching routing key.  For each
// field of the routing key, you can either match against a specific value, or
// match all entries with the '*' wildcard.  Above, we specified the following
// routing key and exchange:
//
//  		pulse.Bind( // routing key and exchange to get messages from
//  		"*.*.*.*.*.*.gaia.#",
//  		"exchange/taskcluster-queue/v1/task-defined"),
//
// This would match all messages on the exchange
// "exchange/taskcluster-queue/v1/task-defined" which have a workerType of
// "gaia" (see the taskDefined link above).  Notice also the '#' at the end of
// the string. This means "match all remaining fields" and can be used to match
// whatever comes after.
//
// To see the list of available exchanges on pulse, visit
// https://wiki.mozilla.org/Auto-tools/Projects/Pulse/Exchanges.
//
// After deciding which exchanges you are interested in, you need a queue to
// have them copied onto. This is also handled by the Consume method, with the
// first argument being the name of the queue to use.  You will notice above
// there are two types of queues we create: named queues, and unnamed queues:
//
//  	conn.Consume(
//  		"taskprocessing", // queue name
//
//  	conn.Consume( // a second workflow to manage concurrently
//  		"", // empty name implies anonymous queue
//
// To understand the difference, first we need to explain the different
// scenarios in which you might want to use them.
//
// Scenario 1) You have one client reading from the queue, and when you
// disconnect, you don't want your queue to receive any more messages
//
// Scenario 2) you have multiple clients that want to feed from the same queue
// (e.g.  when multiple workers can perform the same task, and whichever one
// pops the message off the queue first should process it)
//
// Scenario 3) you only have a single client reading from the queue, but if you
// go offline (crash, network interrupts etc) then you want pulse to keep
// updating your queue so your missed messages are there when you get back.
//
// In scenario 1 above, your client only uses the queue for the scope of the
// connection, and as soon as it disconnects, does not require the queue any
// further. In this case, an unnamed queue can be created, by passing "" as the
// queue name. When the connection closes, the AMQP server will automatically
// delete the queue.
//
// In scenarios 2 it is useful to have a friendly name for the queue that can
// be shared by all the clients using it.  The queue also should not be deleted
// when one client disconnects, it needs to live indefinitely. By providing a
// name for the queue, this signifies to the pulse library, that the queue
// should persist after a disconnect, and pulse should continue to populate the
// queue, even if no pulse clients are connected to consume the messages.
// Please note eventually the Pulse Guardian will delete your queue if you
// leave it collecting messages without consuming them.
//
// Scenario 3 is essentially the same as scenario 2 but with one consumer only.
// Again, a named queue is required.
//
// So, we're nearly done now. We now have a means to consume messages, by
// calling the Consume method, and specifying a queue name, some bindings of
// exchanges and routing keys, but how to actually process messages arriving on
// the queue?
//
// You will notice the Consume method takes a callback function. This can be an
// inline function, or point to any available function in your go code. You
// simply need to have a function that accepts an amqp.Delivery input, and pass
// it into the Consume method. Above, we did it like this:
//
//  		func(delivery amqp.Delivery) { // callback function to pass messages to
//  			fmt.Println("Received from exchange " + delivery.Exchange + ":")
//  			fmt.Println(string(delivery.Body))
//  			fmt.Println("")
//  			delivery.Ack(false) // acknowledge message *after* processing
//  		},
//
// Please see http://godoc.org/github.com/streadway/amqp#Delivery for more
// information on the Delivery type. Most of the time, delivery.Body is all that
// you need, since this contains the pulse message body.
//
// In this example above, we simply output the information we receive, and then
// acknowledge receipt of the message. But why do we need to do this? To explain,
// take a look at the remaining parameters to Consume that we pass in. There
// are two more we have not discussed yet: they are the prefetch size (how many
// messages to fetch at once), and a bool to say whether to auto-acknowledge
// messages or not.
//
//  		1,     // prefetch 1 message at a time
//  		false, // don't auto-acknowledge messages
//
// When you acknowledge a message, it gets popped off the queue. If you don't
// auto-acknowledge, and also don't manually acknowledge, your queue is going
// to grow until it gets deleted by Pulse Guardian, so better to acknowledge
// those messages!  Auto-acknowledge happens when you receive the message; if
// you crash after receiving it but before processing it, you may have a
// problem. If it is important not to lose messages in such a scenario, you can
// acknowledge manually *after* processing the message. See above:
//
//  			delivery.Ack(false) // acknowledge message *after* processing
//
// This is "more work" for you to do, but guarantees that you don't lose
// messages. To handle situation of crashing after processing, but before
// acknowledging, having an idempotent message processing function (the
// callback) should help avoid the problem of processing a message twice.
//
// Please note the Consume method will take care of connecting to the pulse
// server (if no connection has yet been established), creating an AMQP
// channel, creating or connecting to an existing queue, binding it to all the
// exchanges and routing keys that you specify, and spawning a dedicated go
// routine to process the messages from this queue and feed them back to the
// callback method you provide.
//
// The client is implemented in such a way that a new AMQP channel is created
// for each queue that you consume, and that a separate go routine handles
// calling the callback function you specify. This means you can take advantage
// of go's built in concurrency support, and call the Consume method as many
// times as you wish.
//
// The aim of this library is to shield users from this lower-level resource
// management, and provide a simple interface in order to quickly and easily
// develop components that can interact with pulse.
package pulse

import (
	"code.google.com/p/go-uuid/uuid"
	"fmt"
	"github.com/streadway/amqp"
	"log"
	"os"
	"regexp"
)

func FailOnError(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %s", msg, err)
		panic(fmt.Sprintf("%s: %s", msg, err))
	}
}

type pulseQueue struct {
}

// connection is not exported, so that a factory function must be used
// to create an instance, to control variable initialisation
type Connection struct {
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
func NewConnection(pulseUser string, pulsePassword string, amqpUrl string) Connection {
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

	fmt.Println(pulseUser, pulsePassword, amqpUrl)

	return Connection{
		User:     pulseUser,
		Password: pulsePassword,
		URL:      amqpUrl}
}

func (c *Connection) connect() {
	var err error
	c.AMQPConn, err = amqp.Dial(c.URL)
	FailOnError(err, "Failed to connect to RabbitMQ")
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

// Consume registers a callback method against a queue and a set of bindings.
func (c *Connection) Consume(
	queueName string,
	callback func(amqp.Delivery),
	prefetch int,
	autoAck bool,
	bindings ...Binding) pulseQueue {

	if !c.connected {
		c.connect()
	}

	ch, err := c.AMQPConn.Channel()
	FailOnError(err, "Failed to open a channel")

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
		FailOnError(err, "Failed to passively declare exchange "+bindings[i].ExchangeName())
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
	FailOnError(err, "Failed to declare queue")

	for i := range bindings {
		log.Printf("Binding %s to %s with routing key %s", q.Name, bindings[i].ExchangeName(), bindings[i].RoutingKey())
		err = ch.QueueBind(
			q.Name, // queue name
			bindings[i].RoutingKey(),   // routing key
			bindings[i].ExchangeName(), // exchange
			false,
			nil)
		FailOnError(err, "Failed to bind a queue")
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
	FailOnError(err, "Failed to register a consumer")

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
