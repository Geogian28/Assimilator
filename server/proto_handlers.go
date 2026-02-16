package server

import (
	"context"
	"fmt"
	"io"
	"os"

	assctl "github.com/geogian28/Assimilator/proto"
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
		Machines: toProtoMachineConfigMap(&DesiredState.Machines),
		Users:    toProtoUserConfigMap(&DesiredState.Users),
	}
	Info("Returning response to agent.")
	return response, nil
}

// GetAllConfigs implements AssimilatorService
func (s *AssimilatorServer) GetSpecificConfig(ctx context.Context, req *pb.GetSpecificConfigRequest) (*pb.GetSpecificConfigResponse, error) {
	Trace("Agent attempting to get config for machine: ", req.MachineName)
	if DesiredState == nil {
		Warning("Agent attempted to get a specific config, but Server has not loaded the configuration yet")
		return nil, fmt.Errorf("server has not loaded the configuration yet")
	}
	if len(DesiredState.Machines) == 0 {
		Warning("Configs loaded, but there are no machines.")
		return nil, fmt.Errorf("configs loaded, but there are no machines")
	}
	// Trace("Printing DesiredState.Machines[req.MachineName]: \n%v\n", DesiredState.Machines[req.MachineName])
	if machine, okay := DesiredState.Machines[req.MachineName]; okay {
		Trace("Found a machine with name: ", req.MachineName)
		Info("Returning response to ", req.MachineName, "'s agent.")
		Trace("")
		return &pb.GetSpecificConfigResponse{
			Machine: toProtoMachineConfig(&machine),
			Version: toProtoServerVersion(&s.ServerVersion),
			Users:   toProtoUserConfigMap(&DesiredState.Users),
		}, nil
	}
	Debug("Cannot find a machine with name: ", req.MachineName)
	return nil, status.Errorf(codes.NotFound, "cannot find a machine with name: %v", req.MachineName)
}

// func (s *AssimilatorServer) DownloadPackage(req *assctl.PackageRequest, stream pb.Assimilator_DownloadPackageServer) error {
// 	Info("Client requested ", req.Category, " package: ", req.Name)
// 	if s.PackageDir == "" {
// 		return status.Error(codes.Internal, "Server repository directory is not configured")
// 	}

// 	// 1. Find the package
// 	filepath := packagesMap[req.Category][req.Name].packagePermPath

// 	// 2. Upload the package
// 	err := stream.Send(&assctl.PackageResponse{
// 		TotalSize: int64(len(filepath)),
// 		Content:   []byte(filepath),
// 	})
// 	if err != nil {
// 		return err
// 	}

// 	return nil
// }

func (s *AssimilatorServer) DownloadPackage(req *assctl.PackageRequest, stream pb.Assimilator_DownloadPackageServer) error {
	Info("Client requested ", req.Category, " package: ", req.Name)
	if s.PackageDir == "" {
		return status.Error(codes.Internal, "Server repository directory is not configured")
	}

	// 1. Validate that the package exists in our map
	catMap, ok := packagesMap[req.Category]
	if !ok {
		return status.Errorf(codes.NotFound, "category %s not found", req.Category)
	}
	pkgInfo, ok := catMap[req.Name]
	if !ok {
		return status.Errorf(codes.NotFound, "package %s not found in category %s", req.Name, req.Category)
	}

	// 2. Open the file
	file, err := os.Open(pkgInfo.packagePermPath)
	if err != nil {
		Error("Failed to open package file: ", err)
		return status.Errorf(codes.Internal, "failed to open package file")
	}
	defer file.Close()

	// 3. Get file size for the progress bar
	stat, err := file.Stat()
	if err != nil {
		return status.Errorf(codes.Internal, "failed to stat file")
	}
	totalSize := stat.Size()

	// 4. Stream the file in 32KB chunks
	buffer := make([]byte, 32*1024)
	sentFirstChunk := false

	for {
		n, err := file.Read(buffer)
		if err == io.EOF {
			break
		}
		if err != nil {
			return status.Errorf(codes.Internal, "failed to read file: %v", err)
		}

		resp := &assctl.PackageResponse{
			Content: buffer[:n],
		}

		// Only send TotalSize on the very first packet
		if !sentFirstChunk {
			resp.TotalSize = totalSize
			sentFirstChunk = true
		}

		if err := stream.Send(resp); err != nil {
			return status.Errorf(codes.Internal, "failed to send chunk: %v", err)
		}
	}

	Info("Successfully sent package: ", req.Name)
	return nil
}
