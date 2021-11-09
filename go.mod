module github.com/layer5io/meshsync

replace (
	github.com/docker/docker => github.com/moby/moby v17.12.0-ce-rc1.0.20200618181300-9dc6525e6118+incompatible
	github.com/kudobuilder/kuttl => github.com/layer5io/kuttl v0.4.1-0.20200806180306-b7e46afd657f
	golang.org/x/sys => golang.org/x/sys v0.0.0-20200826173525-f9321e4c35a6
	vbom.ml/util => github.com/fvbommel/util v0.0.0-20180919145318-efcd4e0f9787
)

go 1.13

require (
	github.com/buger/jsonparser v1.1.1
	github.com/google/uuid v1.1.1
	github.com/layer5io/meshkit v0.2.14
	github.com/myntra/pipeline v0.0.0-20180618182531-2babf4864ce8
	github.com/spf13/viper v1.7.1
	gorm.io/gorm v1.20.10
	k8s.io/api v0.18.12
	k8s.io/apimachinery v0.18.12
	k8s.io/client-go v0.18.12
	k8s.io/kubectl v0.18.8
)
