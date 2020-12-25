module github.com/layer5io/meshsync

replace (
	github.com/kudobuilder/kuttl => github.com/layer5io/kuttl v0.4.1-0.20200806180306-b7e46afd657f
	golang.org/x/sys => golang.org/x/sys v0.0.0-20200826173525-f9321e4c35a6
	vbom.ml/util => github.com/fvbommel/util v0.0.0-20180919145318-efcd4e0f9787
)

go 1.13

require (
	github.com/golang/protobuf v1.4.3
	github.com/golangci/golangci-lint v1.33.0 // indirect
	github.com/google/uuid v1.1.2
	github.com/grpc-ecosystem/go-grpc-middleware v1.2.2
	github.com/layer5io/meshery-adapter-library v0.1.9
	github.com/layer5io/meshery-operator v0.2.0
	github.com/layer5io/meshkit v0.1.30
	github.com/myntra/pipeline v0.0.0-20180618182531-2babf4864ce8
	github.com/nats-io/nats.go v1.10.0
	google.golang.org/grpc v1.34.0
	google.golang.org/protobuf v1.25.0
	istio.io/client-go v1.8.1
	k8s.io/api v0.18.12
	k8s.io/apimachinery v0.18.12
	k8s.io/client-go v0.18.12
	sigs.k8s.io/controller-runtime v0.6.4
)
