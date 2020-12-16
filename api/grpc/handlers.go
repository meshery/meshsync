package grpc

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"
	controller "github.com/layer5io/meshkit/protobuf/controller"
	proto "github.com/layer5io/meshsync/internal/proto"
)

func (s *Service) Info(context.Context, *empty.Empty) (*controller.ControllerInfo, error) {
	return &controller.ControllerInfo{
		Name:    s.Name,
		Version: s.Version,
	}, nil
}

func (s *Service) Health(context.Context, *empty.Empty) (*controller.ControllerHealth, error) {
	return &controller.ControllerHealth{
		Status: controller.ControllerStatus_RUNNING,
	}, nil
}

func (s *Service) Sync(context.Context, *proto.Request) (*proto.Response, error) {
	return &proto.Response{
		Result: &proto.Response_Message{
			Message: "ok",
		},
	}, nil
}
