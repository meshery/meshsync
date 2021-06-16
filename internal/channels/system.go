package channels

import "os"

var (
	OS     = "os"
	Stop   = "stop"
	ReSync = "resync"
)

func NewStopChannel() StopChannel {
	return make(chan struct{})
}

type StopChannel chan struct{}

func (ch StopChannel) Stop() {
	<-ch
}

func NewOSChannel() OSChannel {
	return make(chan os.Signal, 1)
}

type OSChannel chan os.Signal

func (ch OSChannel) Stop() {
	<-ch
}

func NewReSyncChannel() ReSyncChannel {
	return make(chan struct{})
}

type ReSyncChannel chan struct{}

func (ch ReSyncChannel) Stop() {
	<-ch
}
