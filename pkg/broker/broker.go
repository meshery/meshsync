package broker

var (
	List   ObjectType = "list"
	Single ObjectType = "single"
)

type ObjectType string

type Message struct {
	Type   ObjectType
	Object interface{}
}

type PublishInterface interface {
	Publish(string, *Message) error
	PublishWithCallback(string, string, *Message) error
}

type SubscribeInterface interface {
	Subscribe(string, string, *Message) error
	SubscribeWithCallback(string, string, *Message) error
	SubscribeWithChannel(string, string, chan *Message) error
}

type Handler interface {
	PublishInterface
	SubscribeInterface
}
