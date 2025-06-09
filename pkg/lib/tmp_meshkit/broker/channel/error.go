package channel

import "github.com/meshery/meshkit/errors"

// TODO run error-util
const (
	ErrPublishCode = "replace_me"
)

func ErrPublish(err error) error {
	return errors.New(ErrPublishCode, errors.Alert, []string{"Publish failed"}, []string{err.Error()}, []string{"Publish to channel failed", "Subject channel buffer is full"}, []string{"Make sure there is a consumer from the subject"})
}
