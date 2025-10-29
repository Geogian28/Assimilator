package server

import (
	"context"
	"fmt"

	pb "github.com/geogian28/Assimilator/proto"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// GetAllConfigs implements AssimilatorService
func (s *AssimilatorServer) GetAllConfigs(ctx context.Context, req *pb.GetAllConfigsRequest) (*pb.GetAllConfigsResponse, error) {
	if DesiredState == nil {
		Warning("Agent attempted to get all configs, but Server has not loaded the configuration yet")
		return nil, fmt.Errorf("server has not loaded the configuration yet")
	}
	response := &pb.GetAllConfigsResponse{
		Config: toProtoDesiredState(DesiredState),
	}
	Info("Returning response to agent.")
	return response, nil
}

// `GetAllConfigs` implements AssimilatorService
func (s *AssimilatorServer) GetSpecificConfig(ctx context.Context, req *pb.GetSpecificConfigRequest) (*pb.GetSpecificConfigResponse, error) {
	Trace("Agent attempting to get config for machine: ", req.MachineName)
	if DesiredState == nil {
		Warning("Agent attempted to get all configs, but Server has not loaded the configuration yet")
		return nil, fmt.Errorf("server has not loaded the configuration yet")
	}
	if len(DesiredState.Machines) == 0 {
		Warning("Configs loaded, but there are no machines.")
		return nil, fmt.Errorf("configs loaded, but there are no machines")
	}
	Trace("Printing DesiredState.Machines[req.MachineName]: \n%v\n", DesiredState.Machines[req.MachineName])
	if machine, okay := DesiredState.Machines[req.MachineName]; okay {
		Trace("Found a machine with name: ", req.MachineName)
		Info("Returning response to ", req.MachineName, "'s agent.")
		return &pb.GetSpecificConfigResponse{
			Machine: toProtoMachineConfig(&machine),
			Version: toProtoServerVersion(ServerVersionInfo),
			// TODO: Add more fields to the response
		}, nil
	}
	Debug("Cannot find a machine with name: ", req.MachineName)
	return nil, status.Errorf(codes.NotFound, "cannot find a machine with name: %v", req.MachineName)
}
