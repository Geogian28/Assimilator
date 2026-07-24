package main

import (
	pb "github.com/geogian28/Assimilator/proto"
)

func toProtoAppConfig(ac AppConfig) *pb.AppConfig {
	return &pb.AppConfig{
		IsServer: ac.IsServer, // Maps to bool isServer = 1
		IsAgent:  ac.IsAgent,  // Maps to bool isAgent = 2
		// MAAS:           ac.MAAS,                  // Maps to bool mAAS = 3
		GithubUsername: ac.GithubUsername,        // Maps to string GithubUsername = 4
		GithubToken:    ac.GithubToken,           // Maps to string GithubToken = 5
		GithubRepo:     ac.GithubRepo,            // Maps to string GithubRepo = 6
		VerbosityLevel: int32(ac.VerbosityLevel), // Maps to int32 verbosityLevel = 8
	}
}

func toProtoServerVersion(version *ServerVersion) *pb.ServerVersion {
	return &pb.ServerVersion{
		Version:   version.Version,
		Commit:    version.Commit,
		BuildDate: version.BuildDate,
	}
}

func toProtoPackageConfigMap(packages *map[string][]PackageStep) map[string]*pb.PackageConfig {
	res := make(map[string]*pb.PackageConfig, len(*packages))
	for packageName, packageSteps := range *packages {
		res[packageName] = toProtoPackageConfig(packageSteps)
	}
	return res
}

func toProtoPackageConfig(packageSteps []PackageStep) *pb.PackageConfig {
	pbPackageSteps := make([]*pb.PackageSteps, len(packageSteps))

	for i, packageStep := range packageSteps {
		pbPackageSteps[i] = toProtoPackageSteps(&packageStep)
	}
	return &pb.PackageConfig{
		PackageSteps: pbPackageSteps,
		Checksum:     packageSteps[0].Checksum,
	}
}

func toProtoPackageSteps(packageConfig *PackageStep) *pb.PackageSteps {
	if packageConfig.RunAsUser == "" {
		packageConfig.RunAsUser = "root"
	}
	return &pb.PackageSteps{
		Action:    packageConfig.Action,
		Arguments: packageConfig.Arguments,
		Runasuser: packageConfig.RunAsUser,
	}
}

func toProtoMachineConfigMap(machines *map[string]MachineConfig) map[string]*pb.MachineConfig {
	res := make(map[string]*pb.MachineConfig, len(*machines))
	for machineName, machineConfig := range *machines {
		res[machineName] = toProtoMachineConfig(machineConfig)
	}
	return res
}

func toProtoMachineConfig(machineConfig MachineConfig) *pb.MachineConfig {
	return &pb.MachineConfig{
		AppliedProfiles: machineConfig.AppliedProfiles,
		Packages:        toProtoPackageConfigMap(&machineConfig.Packages),
		ConfigOverrides: toProtoAppConfig(machineConfig.Global),
	}
}

// func toProtoUserConfigMap(users *map[string]UserConfig) map[string]*pb.UserConfig {
// 	res := make(map[string]*pb.UserConfig, len(*users))
// 	for userName, userConfig := range *users {
// 		res[userName] = toProtoUserConfig(userConfig)
// 	}
// 	return res
// }

// func toProtoUserConfig(userConfig UserConfig) *pb.UserConfig {
// 	return &pb.UserConfig{
// 		AppliedProfiles: userConfig.AppliedProfiles,
// 		Packages:        toProtoPackageConfigMap(&userConfig.Packages),
// 		//Dotfiles: toProtoDotfilesMap(&user.Dotfiles),
// 	}
// }
