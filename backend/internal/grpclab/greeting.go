package grpclab

import (
	"context"
	"fmt"

	grpcv1 "devlab-board/backend/gen/grpclab/v1"
)

type GreetingServer struct {
	grpcv1.UnimplementedGreetingServiceServer
}

func NewGreetingServer() *GreetingServer {
	return &GreetingServer{}
}

func (s *GreetingServer) Hello(
	ctx context.Context,
	req *grpcv1.HelloRequest,
) (*grpcv1.HelloResponse, error) {
	return &grpcv1.HelloResponse{
		Message: fmt.Sprintf("Hello, %s!", req.GetName()),
	}, nil
}
