package config

import "time"

var (
	Server = map[string]string{
		"name":      "meshery-meshsync",
		"port":      "11000",
		"version":   "v0.0.1-alpha3",
		"startedat": time.Now().String(),
	}

	Pipelines = map[string]PipelineConfigs{
		GlobalResourceKey: []PipelineConfig{
			{
				Group:          "",
				Version:        "v1",
				Resource:       "namespaces",
				Namespace:      "",
				PublishSubject: "meshery.meshsync.discovery",
			},
		},
		LocalResourceKey: []PipelineConfig{
			{
				Group:          "",
				Version:        "v1",
				Resource:       "pods",
				Namespace:      "",
				PublishSubject: "meshery.meshsync.discovery",
			},
		},
	}
)
