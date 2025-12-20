package tests

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/meshery/meshkit/broker"
	"github.com/nats-io/nats.go"

	"github.com/meshery/meshsync/internal/config"
)

func TestIntegrationResyncFlowViaBroker(t *testing.T) {
	brokerURL := "nats://localhost:4222"

	// connect to broker
	nc, err := nats.Connect(brokerURL)
	if err != nil {
		t.Fatalf("failed to connect to broker: %v", err)
	}
	defer nc.Drain()

	// load config
	cfg, err := config.New("viper")
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// listener config (loaded even if unused)
	_ = cfg

	// subjects
	requestSubject := "meshery.meshsync.request"
	responseSubject := config.DefaultPublishingSubject

	// subscribe to resource events
	msgs := make(chan *nats.Msg, 200)
	sub, err := nc.ChanSubscribe(responseSubject, msgs)
	if err != nil {
		t.Fatalf("failed to subscribe to resource subject: %v", err)
	}
	defer sub.Unsubscribe()

	// STEP 1 — wait for first discovery event (MeshSync must warm up)
	warmupTimeout := time.After(120 * time.Second)
	receivedInitial := false
	for !receivedInitial {
		select {
		case msg := <-msgs:
			var m broker.Message
			if json.Unmarshal(msg.Data, &m) == nil && m.Object != nil {
				receivedInitial = true
			}
		case <-warmupTimeout:
			t.Fatalf("timeout: initial discovery events never arrived — MeshSync never warmed up")
		}
	}

	// STEP 2 — send resync AFTER MeshSync is initialized
	req := &broker.Message{
		Request: &broker.RequestObject{
			Entity: broker.ReSyncDiscoveryEntity,
		},
	}
	payload, _ := json.Marshal(req)
	if err := nc.Publish(requestSubject, payload); err != nil {
		t.Fatalf("failed to publish resync request: %v", err)
	}

	// STEP 3 — expect NEW resource events after resync
	afterTimeout := time.After(120 * time.Second)
	for {
		select {
		case msg := <-msgs:
			var m broker.Message
			if json.Unmarshal(msg.Data, &m) == nil && m.Object != nil {
				// resync confirmed
				return
			}
		case <-afterTimeout:
			t.Fatalf("timeout: no resource events were received after resync request")
		}
	}
}
