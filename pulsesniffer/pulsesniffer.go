// Package pulsesniffer provides a simple example program that listens to some
// real world pulse messages.
package main

import (
	"fmt"

	"github.com/streadway/amqp"
	"github.com/taskcluster/pulse-go/pulse"
)

func main() {
	// Passing all empty strings:
	// empty user => use PULSE_USERNAME env var
	// empty password => use PULSE_PASSWORD env var
	// empty url => connect to production
	conn := pulse.NewConnection("", "", "")
	conn.Consume(
		"taskprocessing", // queue name
		func(message interface{}, delivery amqp.Delivery) { // callback function to pass messages to
			fmt.Println("Received from exchange " + delivery.Exchange + ":")
			fmt.Println(string(delivery.Body))
			fmt.Println("")
			delivery.Ack(false) // acknowledge message *after* processing
		},
		1,     // prefetch 1 message at a time
		false, // don't auto-acknowledge messages
		pulse.Bind( // routing key and exchange to get messages from
			"*.*.*.*.*.*.gaia.#",
			"exchange/taskcluster-queue/v1/task-defined"),
		pulse.Bind( // another routing key and exchange to get messages from
			"*.*.*.*.*.aws-provisioner.#",
			"exchange/taskcluster-queue/v1/task-running"))
	conn.Consume( // a second workflow to manage concurrently
		"", // empty name implies anonymous queue
		func(message interface{}, delivery amqp.Delivery) { // simpler callback than before
			fmt.Println("Buildbot message received")
			fmt.Println("")
		},
		1,    // prefetch
		true, // auto acknowledge, so no need to call delivery.Ack
		pulse.Bind( // routing key and exchange to get messages from
			"#", // get *all* normalized buildbot messages
			"exchange/build/normalized"))
	// wait forever
	forever := make(chan bool)
	<-forever
}
