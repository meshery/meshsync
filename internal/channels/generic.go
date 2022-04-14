package channels

const (
	Struct = "struct"
)

func NewStructChannel() StructChannel {
	return make(chan struct{})
}

type StructChannel chan struct{}

func (ch StructChannel) Stop() {
	<-ch
}
