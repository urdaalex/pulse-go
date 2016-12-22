package taskclusterpulse

import (
	"errors"
	"os"

	pulsewrapper "github.com/taskcluster/pulse-go/pulse"
	tcclient "github.com/taskcluster/taskcluster-client-go"
	"github.com/taskcluster/taskcluster-client-go/pulse"
)

// Options for creating a new connection using taskcluster pulse credentials
type NewConnectionOptions struct {
	ClientID    string
	AccessToken string
	AMQPUrl     string
	Namespace   string
}

// NewConnection prepares a Connection object with a ClientID, AccessToken and an
// AMQP URL, but does not actually make an outbound connection to the service.
// An actual network connection will be made the first time the Consume method
// is called on the returned connection. All arguments are encapsulated in the
// NewConnectionOptions struct, which is a wrapper for the parameters listed below
//
// The ClientID and AccessToken are determined as follows:
//
// If the provided parameter is a non-empty string, it will be used for
// connecting to Taskcluster-Pulse. Otherwise, if environment variable CLIENT_ID and ACCESS_TOKEN
// is non empty, they will be used.  Otherwise, an error will be returned.
//
// The logic for deriving the AMQP url is as follows:
//
// If the provided AMQPUrl is a non-empty string, it will be used to set the
// AMQP URL.  Otherwise, production will be used
// ("amqps://pulse.mozilla.org:5671")
//
// Finally, the AMQP url is adjusted, by stripping out any user/password
// contained inside it.
//
// Typically, a call to this method would look like:
//		options := taskclusterpulse.NewConnectionOptions{
//			ClientID:		"",
//			AccessToken:    "",
//			AMQPUrl:  		"",
//			Namespace:      "mynamespace",
//          }

//  	conn := taskclusterpulse.NewConnection(options)
//
// whereby the client program would export CLIENT_ID and ACCESS_TOKEN
// environment variables before calling the go program, and the empty url would
// signify that the client should connect to the production instance.
//
// NOTE: Any pulse username and password embeded in the AMQPUrl will not be used when preparing
// connections using taskcluster-pulse credentials
func NewConnection(options NewConnectionOptions) (pulsewrapper.Connection, error) {
	if options.ClientID == "" {
		options.ClientID = os.Getenv("CLIENT_ID")
	}
	if options.AccessToken == "" {
		options.AccessToken = os.Getenv("ACCESS_TOKEN")
	}
	if options.AMQPUrl == "" {
		options.AMQPUrl = "amqps://pulse.mozilla.org:5671"
	}
	if options.Namespace == "" {
		return pulsewrapper.Connection{}, errors.New("No Namespace provided.")
	}
	if options.ClientID == "" {
		return pulsewrapper.Connection{}, errors.New("No ClientID provided.")
	}
	if options.AccessToken == "" {
		return pulsewrapper.Connection{}, errors.New("No AccessToken provided.")
	}

	//get new pulse credentials
	creds := &tcclient.Credentials{
		ClientID:    options.ClientID,
		AccessToken: options.AccessToken,
	}
	tcPulse := pulse.New(creds)

	//TODO: NOT COMPLETE..still being developed, email/irc options must be parameterized
	//TODO: remove below line when complete
	return pulsewrapper.Connection{}, errors.New("Taskcluster-Pulse implementation not complete")
	tcCreds, err := tcPulse.Namespace(options.Namespace,
		&pulse.NamespaceCreationRequest{
			Contact: pulse.SendEmailRequest{
				Method: "email",
				Payload: struct {
					Address string `json:"address"`
					ReplyTo string `json:"replyTo,omitempty"`
				}{
					Address: "test@falskjdhfasdkf.com",
					ReplyTo: "testreply@email.com",
				},
			},
		})

	if err != nil {
		return pulsewrapper.Connection{}, err
	}

	//prefix required for all taskcluster queues
	prefix := "taskcluster/queues" + tcCreds.Namespace
	pulseUser := tcCreds.Username
	pulsePassword := tcCreds.Password

	pulseOptions := pulsewrapper.NewConnectionOptions{
		PulseUser:     pulseUser,
		PulsePassword: pulsePassword,
		AMQPUrl:       options.AMQPUrl,
		QueuePrefix:   prefix,
	}

	//supply pulse credentials to pulse wrapper
	pulseGoConnection := pulsewrapper.NewConnection(pulseOptions)

	//return the connection
	return pulseGoConnection, nil
}
