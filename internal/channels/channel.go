package channels

type GenericChannel interface {
	Stop()
}

func NewChannelPool() map[string]GenericChannel {
	return map[string]GenericChannel{
		Stop:   NewStopChannel(),
		OS:     NewOSChannel(),
		ReSync: NewReSyncChannel(),
	}
}
