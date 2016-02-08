package main

import (
	"fmt"
	"os"
	"testing"

	"github.com/taskcluster/pulse-go/pulse"
)

func TestConnectionURLDetermination(t *testing.T) {
	testUrl := func(user, password, url, resolvedUrl string) {
		conn := pulse.NewConnection(user, password, url)
		if conn.URL != resolvedUrl {
			t.Fail()
		}
	}
	testUrl("pmoore_test1", "donkey123", "amqps://pmoore_test1:donkey123@localhost:5671", "amqps://pmoore_test1:donkey123@localhost:5671")
	testUrl("pmoore_test1", "donkey123", "amqp://x@localhost:5671/dsds", "amqp://pmoore_test1:donkey123@localhost:5671/dsds")
	testUrl("pmoore_test1", "donkey123", "amqp://localhost:5671/dsds", "amqp://pmoore_test1:donkey123@localhost:5671/dsds")
	testUrl("pmoore_test1", "donkey123", "amqp://a:bx@localhost:5671/dsds", "amqp://pmoore_test1:donkey123@localhost:5671/dsds")
	testUrl("pmoore_test1", "donkey123", "amqp:fdf://x:2@localhost:5671/dsds", "amqp:fdf://pmoore_test1:donkey123@localhost:5671/dsds")
	testUrl("pmoore_test1", "donkey123", "amqp:////dsds", "amqp://pmoore_test1:donkey123@//dsds")
	testUrl("pmoore_test1", "donkey123", "amqp://:@localhost:5671/dsds", "amqp://pmoore_test1:donkey123@localhost:5671/dsds")
}

func TestUsernameDetermination(t *testing.T) {
	testUser := func(url, resolvedUser string) {
		conn := pulse.NewConnection("", "", url)
		if conn.User != resolvedUser {
			fmt.Println("Username: " + conn.User + " != " + resolvedUser + " (derived from " + url + ")")
			t.Fail()
		}
	}
	err := os.Setenv("PULSE_USERNAME", "")
	if err != nil {
		t.Fatalf("Could not unset env variable PULSE_PASSWORD")
	}
	testUser("amqps://pmoore_test1:donkey123@loc:4561/sdf/343", "pmoore_test1")
	testUser("amqps://:@loc:4561/sdf/343", "guest")
	testUser("amqps://@loc:4561/sdf/343", "guest")
	testUser("amqps://loc:4561/sdf/343", "guest")
	testUser("amqps://loc:4561/s@df/343", "loc")
	err = os.Setenv("PULSE_USERNAME", "zog")
	if err != nil {
		t.Fatalf("Could not set env variable PULSE_USERNAME to 'zog'")
	}
	testUser("amqps://loc:4561/sdf/343", "zog")
}

func TestPasswordDetermination(t *testing.T) {
	testPassword := func(url, resolvedPassword string) {
		conn := pulse.NewConnection("", "", url)
		if conn.Password != resolvedPassword {
			fmt.Println("Password: " + conn.Password + " != " + resolvedPassword + " (derived from " + url + ")")
			t.Fail()
		}
	}
	err := os.Setenv("PULSE_PASSWORD", "")
	if err != nil {
		t.Fatalf("Could not unset env variable PULSE_PASSWORD")
	}
	testPassword("amqps://pmoore_test1:donkey123@loc:4561/sdf/343", "donkey123")
	testPassword("amqps://:@loc:4561/sdf/343", "guest")
	testPassword("amqps://@loc:4561/sdf/343", "guest")
	testPassword("amqps://loc:4561/sdf/343", "guest")
	testPassword("amqps://loc:4561/s@df/343", "4561/s")
	testPassword("amqps://:x@loc:4561/sdf/343", "x")
	testPassword("amqps://@:x@loc:4561/sdf/343", "guest")
	testPassword("amqps://:x/@loc:4561/sdf/343", "x/")
	testPassword("amqps://:@@loc:4561/sdf/343", "guest")
	err = os.Setenv("PULSE_PASSWORD", "moobit")
	if err != nil {
		t.Fatalf("Could not set env variable PULSE_PASSWORD to 'moobit'")
	}
	testPassword("amqps://@:x@loc:4561/sdf/343", "moobit")
}
