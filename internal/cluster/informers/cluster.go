package informers

import (
	broker "github.com/layer5io/meshsync/pkg/broker"
	informers "github.com/layer5io/meshsync/pkg/informers"
)

var Subject = "cluster"

type Cluster struct {
	client *informers.Client
	broker broker.Handler
}

func New(client *informers.Client, broker broker.Handler) *Cluster {
	return &Cluster{
		client: client,
		broker: broker,
	}
}
