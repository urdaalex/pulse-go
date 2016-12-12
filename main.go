// pulse-go is a command line tool for listening to events from pulse.mozilla.org. It
// allows you to selectively bind to different exchanges and routing keys, and output
// the received messages to standard out. Run pulse-go -h for more information about
// command line arguments.
// It relies heavily on the pulse go library: "github.com/taskcluster/pulse-go/pulse",
// which is a general purpose library for interacting with pulse exchanges in the go
// language.
package main

import (
	"fmt"
	"log"

	docopt "github.com/docopt/docopt-go"
	"github.com/streadway/amqp"
	"github.com/taskcluster/pulse-go/pulse"
)

var (
	version = "pulse-go 1.0"
	usage   = `
pulse-go
pulse-go is a very simple command line utility that allows you to specify a list of Pulse
exchanges/routing keys that you wish to bind to, and prints the body of the Pulse messages
to standard out.

Derivation of username, password and AMQP server url
====================================================
If no AMQP server is specified, production will be used (amqps://pulse.mozilla.org:5671).

If a pulse username is specified on the command line, it will be used.
Otherwise, if the AMQP server url is provided and contains a username, it will be used.
Otherwise, if a value is set in the environment variable PULSE_USERNAME, it will be used.
Otherwise the value 'guest' will be used.

If a pulse password is specified on the command line, it will be used.
Otherwise, if the AMQP server url is provided and contains a password, it will be used.
Otherwise, if a value is set in the environment variable PULSE_PASSWORD, it will be used.
Otherwise the value 'guest' will be used.

  Usage:
      pulse-go [-u <pulse_user>] [-p <pulse_password>] [-s <amqp_server_url>] (<exchange> <routing_key>)...
      pulse-go -h | --help

  Options:
    -h, --help            Display this help text.
    -u <pulse_user>       The pulse user to connect with (see http://pulse.mozilla.org/).
    -p <pulse_password>   The password to use for connecting to pulse.
    -s <amqp_server_url>  The full amqp/amqps url to use for connecting to the pulse server.

  Examples:
    1)  pulse-go -u pmoore_test1 -p potato123 \
        exchange/build/ '#' \
        exchange/taskcluster-queue/v1/task-defined '*.*.*.*.*.null-provisioner.buildbot-try.#'

    This would display all messages from exchange exchange/build/ and only messages from
    exchange/taskcluster-queue/v1/task-defined with provisionerId = "null-provisioner" and
	workerType = "buildbot-try" (see
    https://docs.taskcluster.net/reference/platform/queue/exchanges#taskDefined for more
    information).

    Remember to quote your routing key strings on the command line, so they are not
    interpreted by your shell!

    Please note if you are interacting with taskcluster exchanges, please consider using one
    of the following libraries, for better handling:

      * https://github.com/taskcluster/taskcluster-client-go
      * https://github.com/taskcluster/taskcluster-client

    2) pulse-go -s amqps://admin:peanuts@localhost:5671 exchange/treeherder/v2/new-result-set '#'

    This would match all messages on the given exchange, published to the local AMQP service
    running on localhost. Notice that the user and password are given as part of the url.

`
)

func main() {
	// Parse the docopt string and exit on any error or help message.
	arguments, err := docopt.Parse(usage, nil, true, version, false, true)
	if err != nil {
		log.Fatalf("Not able to parse command line arguments. Received error:\n%v\n", err)
	}

	amqpUrl := ""
	pulseUser := ""
	pulsePassword := ""
	if x := arguments["-s"]; x != nil {
		amqpUrl = x.(string)
	}
	if x := arguments["-u"]; x != nil {
		pulseUser = x.(string)
	}
	if x := arguments["-p"]; x != nil {
		pulsePassword = x.(string)
	}

	exchanges := arguments["<exchange>"].([]string)
	routingKeys := arguments["<routing_key>"].([]string)

	// generate bindings from the command line arguments supplied
	// the bindings are pairs of routing key + exchange
	bindings := make([]pulse.Binding, len(exchanges))
	for i := range exchanges {
		bindings[i] = pulse.Bind(routingKeys[i], exchanges[i])
	}

	options := pulse.NewConnectionOptions{
		PulseUser:		pulseUser,       
		PulsePassword:  pulsePassword,
		AMQPUrl:  		amqpUrl,
		QueuePrefix:		""}

	p1 := pulse.NewConnection(options)
	// If not connecting to production, you can specify a different url...

	// Simple example callback function to just print message body...
	printMe := func(message interface{}, d amqp.Delivery) {
		fmt.Println(string(d.Body))
		// only ack after printing message to standard out
		err := d.Ack(false)
		if err != nil {
			log.Fatalf("Not able to ack pulse message:\n%v\n", err)
		}
	}

	_, err = p1.Consume(
		"",      // queue name ("" implies uuid should be generated)
		printMe, // callback function to call with each AMQP delivery...
		1,       // prefetch
		false,   // autoAck - we want to acknowledge ourselves
		bindings...)

	if err != nil {
		log.Fatalf("Not able to consume pulse messages from queue. Error occurred:\n%v\n", err)
	}

	// wait for a never-arriving message, to avoid exiting program
	forever := make(chan bool)
	<-forever
}
