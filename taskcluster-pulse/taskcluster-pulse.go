package taskcluster-pulse

import(
    "os"
    "errors"

    tcclient "github.com/taskcluster/taskcluster-client-go"
    "github.com/taskcluster/taskcluster-client-go/pulse"
    pulsewrapper "github.com/taskcluster/pulse-go/pulse"
)

func NewConnection(clientID string, accessToken string, amqpUrl string, namespace string) (pulsewrapper.Connection, error) {
	if clientID == "" {
		clientID = os.Getenv("CLIENT_ID")  
	}
    if accessToken  == "" {
		accessToken = os.Getenv("ACCESS_TOKEN") 
	}
    if amqpUrl == "" {
		amqpUrl = "amqps://pulse.mozilla.org:5671"
	}
    if namespace == "" {
		return pulsewrapper.Connection{}, errors.New("No namespace provided.")
	}
    if clientID == "" {
		return pulsewrapper.Connection{}, errors.New("No clientID provided.")
	}
    if accessToken  == "" {
		return pulsewrapper.Connection{}, errors.New("No accessToken provided.")
	}

    //get new credentials
    creds := &tcclient.Credentials{
        ClientID:    clientID,
        AccessToken: accessToken,
    }
    tcPulse := pulse.New(creds)

    tcCreds, err := tcPulse.Namespace(namespace, &pulse.NamespaceCreationRequest{/*TODO provide contact info*/})
    
    if err != nil {
        return pulsewrapper.Connection{}, err
    }
    prefix := "taskcluster/queues"+tcCreds.Namespace
    pulseUser := tcCreds.Username
    pulsePassword := tcCreds.Password

    options := pulsewrapper.NewConnectionOptions{
			PulseUser:		pulseUser,       
			PulsePassword:  pulsePassword,
			AMQPUrl:  		amqpUrl,
			Namespace:		prefix}

    //supply creds to pulse wrapper
    pulseGoConnection := pulsewrapper.NewConnection(options)
    
    return pulseGoConnection, nil
}
