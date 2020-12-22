package nats

import (
	"github.com/layer5io/meshsync/pkg/broker"
	nats "github.com/nats-io/nats.go"
)

// Nats will implement Nats subscribe and publish functionality
type Nats struct {
	ec *nats.EncodedConn
}

// New - constructor
func New(serverURL string) (broker.Handler, error) {
	nc, err := nats.Connect(serverURL)
	if err != nil {
		return nil, ErrConnect(err)
	}
	ec, err := nats.NewEncodedConn(nc, nats.JSON_ENCODER)
	if err != nil {
		return nil, ErrEncodedConn(err)
	}
	return &Nats{ec: ec}, nil
}

// Publish - to publish messages
func (n *Nats) Publish(subject string, message *broker.Message) error {
	err := n.ec.Publish(subject, message)
	if err != nil {
		return ErrPublish(err)
	}
	return nil
}

// PublishWithCallback - will implement the request-reply mechanisms
// Arguments:
// request - the subject to which publish a request
// reply - this string will be used by the replier to publish replies
// message - message send by the requestor to replier
// TODO Ques: After this the requestor have to subscribe to the reply subject
func (n *Nats) PublishWithCallback(request, reply string, message *broker.Message) error {
	err := n.ec.PublishRequest(request, reply, message)
	if err != nil {
		return ErrPublishRequest(err)
	}
	return nil
}

// Subscribe - for subscribing messages
// TODO Ques: Do we want to unsubscribe
// TODO will the method-user just subsribe, how will it handle the received messages?
func (n *Nats) Subscribe(subject, queue string, message *broker.Message) error {
	msgch := make(chan *broker.Message)
	sub, err := n.ec.BindRecvQueueChan(subject, queue, msgch)
	if err != nil {
		return ErrQueueSubscribe(err)
	}

	msg := <-msgch
	*message = *msg

	_ = sub.Unsubscribe()
	return nil
}

// SubscribeWithCallback - for handling request-reply protocol
// request is the subject to which the this thing is listening
// when there will be a request
func (n *Nats) SubscribeWithCallback(subject, queue string, message *broker.Message) error {
	msgch := make(chan *broker.Message)
	sub, err := n.ec.BindRecvQueueChan(subject, queue, msgch)
	if err != nil {
		return ErrQueueSubscribe(err)
	}

	msg := <-msgch
	*message = *msg

	_ = sub.Unsubscribe()
	return nil
}

// SubscribeWithChannel will publish all the messages received to the given channel
func (n *Nats) SubscribeWithChannel(subject, queue string, ch chan *broker.Message) error {
	_, err := n.ec.BindRecvQueueChan(subject, queue, ch)
	if err != nil {
		return ErrQueueSubscribe(err)
	}
	return nil
}
