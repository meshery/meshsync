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
	Publish(string, Message) error
	PublishWithCallback(string, string, Message) error
}

type SubscribeInterface interface {
	Subscribe(string, string) error
	SubscribeWithHandler(string, string) error
}

type Handler interface {
	PublishInterface
	SubscribeInterface
}
