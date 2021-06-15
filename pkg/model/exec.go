package model

type ExecRequest struct {
	ID        string `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	Container string `json:"container,omitempty"`
	Stop      bool   `json:"stop,omitempty"`
	TTY       bool   `json:"tty,omitempty"`
}

type ExecRequests map[string]ExecRequest
