package model

type ExecRequest struct {
	ID        string `json:"id,omitempty"`
	Name      string `json:"name,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	Container string `json:"container,omitempty"`
	Stop      bool   `json:"stop,omitempty"`
}

type ExecObject struct {
	ID   string `json:"id,omitempty"`
	Data string `json:"data,omitempty"`
}

type ExecRequests map[string]ExecRequest
