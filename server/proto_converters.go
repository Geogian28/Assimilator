package server

import (

	// Import go-git

	config "github.com/geogian28/Assimilator/config"
	// Import go-git
	// Import go-git
	// For basic HTTP auth if needed

	pb "github.com/geogian28/Assimilator/proto"
)

func toProtoAppConfig(ac config.AppConfig) *pb.AppConfig {
	return &pb.AppConfig{
		IsServer:       ac.IsServer,              // Maps to bool isServer = 1
		IsAgent:        ac.IsAgent,               // Maps to bool isAgent = 2
		MAAS:           ac.MAAS,                  // Maps to bool mAAS = 3
		GithubUsername: ac.GithubUsername,        // Maps to string githubUsername = 4
		GithubToken:    ac.GithubToken,           // Maps to string githubToken = 5
		GithubRepo:     ac.GithubRepo,            // Maps to string githubRepo = 6
		TestMode:       ac.TestMode,              // Maps to bool testMode = 7
		VerbosityLevel: int32(ac.VerbosityLevel), // Maps to int32 verbosityLevel = 8

		// Note: packageMap (tag 9) requires a separate helper function
		// if you intend to sync the internal package map state.
	}
}

func toProtoServerVersion(version *ServerVersion) *pb.ServerVersion {
	return &pb.ServerVersion{
		Version:   version.Version,
		Commit:    version.Commit,
		BuildDate: version.BuildDate,
	}
}

// func toProtoDesiredState(DesiredState *config.DesiredState) *pb.DesiredState {
// 	return &pb.DesiredState{ // The 'Config' field of the response
// 		// Global:   toProtoAppConfig(DesiredState.Global),
// 		Profiles: toProtoConfigProfileMap(&DesiredState.Profiles),
// 		Machines: toProtoMachineConfigMap(&DesiredState.Machines),
// 		Users:    toProtoUserConfigMap(&DesiredState.Users),
// 	}
// }

// func toProtoAppConfig(global config.AppConfig) *pb.AppConfig {
// 	return &pb.AppConfig{
// 		IsServer:       global.IsServer,
// 		IsAgent:        global.IsAgent,
// 		MAAS:           global.MAAS,
// 		TestMode:       global.TestMode,
// 		GithubUsername: global.GithubUsername,
// 		GithubToken:    global.GithubToken,
// 		GithubRepo:     global.GithubRepo,
// 	}
// }

// func toProtoConfigProfileMap(profiles *map[string]config.ConfigProfile) map[string]*pb.ConfigProfile {
// 	res := make(map[string]*pb.ConfigProfile, len(*profiles))
// 	for profileName, profile := range *profiles {
// 		res[profileName] = toProtoConfigProfile(&profile)
// 	}
// 	return res
// }

// func toProtoConfigProfile(profile *config.ConfigProfile) *pb.ConfigProfile {
// 	return &pb.ConfigProfile{
// 		Machines: toProtoMachineConfigMap(&profile.Machines),
// 		Users:    toProtoUserConfigMap(&profile.Users),
// 		// Packages: toProtoPackageConfigMap(&profile.Packages),
// 		// Services: toProtoServiceConfigMap(&profile.Services),
// 	}
// }

func toProtoPackageConfigMap(packages *map[string]config.PackageConfig) map[string]*pb.PackageConfig {
	res := make(map[string]*pb.PackageConfig, len(*packages))
	for packageName, packageConfig := range *packages {
		res[packageName] = toProtoPackageConfig(&packageConfig)
	}
	return res
}

func toProtoPackageConfig(packageConfig *config.PackageConfig) *pb.PackageConfig {
	return &pb.PackageConfig{
		State:    packageConfig.State,
		Version:  packageConfig.Version,
		Branch:   packageConfig.Branch,
		Checksum: packageConfig.Checksum,
		// Requires: toProtoDependenciesMap(&packageConfig.Requires),
	}
}

// func toProtoDependenciesMap(deps *map[string]config.Dependencies) map[string]*pb.Dependencies {
// 	res := make(map[string]*pb.Dependencies, len(*deps))
// 	for depName, dep := range *deps {
// 		res[depName] = toProtoDependencies(&dep)
// 	}
// 	return res
// }

// func toProtoDependencies(dep *config.Dependencies) *pb.Dependencies {
// 	return &pb.Dependencies{
// 		Packages: toProtoPackageConfigMap(&dep.Packages),
// 		Files:    toProtoServiceConfigMap(&dep.Files),
// 	}
// }

// func toProtoServiceConfigMap(services *map[string]config.ServiceConfig) map[string]*pb.ServiceConfig {
// 	res := make(map[string]*pb.ServiceConfig, len(*services))
// 	for serviceName, serviceConfig := range *services {
// 		res[serviceName] = toProtoServiceConfig(&serviceConfig)
// 	}
// 	return res
// }

// func toProtoServiceConfig(serviceConfig *config.ServiceConfig) *pb.ServiceConfig {
// 	return &pb.ServiceConfig{
// 		State:  serviceConfig.State,
// 		Config: serviceConfig.Configs,
// 	}
// }

func toProtoMachineConfigMap(machines *map[string]config.MachineConfig) map[string]*pb.MachineConfig {
	res := make(map[string]*pb.MachineConfig, len(*machines))
	for machineName, machineConfig := range *machines {
		res[machineName] = toProtoMachineConfig(&machineConfig)
	}
	return res
}

func toProtoMachineConfig(machineConfig *config.MachineConfig) *pb.MachineConfig {
	return &pb.MachineConfig{
		AppliedProfiles: machineConfig.AppliedProfiles,
		Packages:        toProtoPackageConfigMap(&machineConfig.Packages),
		ConfigOverrides: toProtoAppConfig(machineConfig.Global),
	}
}

func toProtoUserConfigMap(users *map[string]config.UserConfig) map[string]*pb.UserConfig {
	res := make(map[string]*pb.UserConfig, len(*users))
	for userName, userConfig := range *users {
		res[userName] = toProtoUserConfig(&userConfig)
	}
	return res
}

func toProtoUserConfig(userConfig *config.UserConfig) *pb.UserConfig {
	return &pb.UserConfig{
		AppliedProfiles: userConfig.AppliedProfiles,
		Packages:        toProtoPackageConfigMap(&userConfig.Packages),
		//Dotfiles: toProtoDotfilesMap(&user.Dotfiles),
	}
}

// func toProtoDotfilesMap(dotfiles *map[string]config.Dotfiles) map[string]*pb.Dotfiles {
// 	res := make(map[string]*pb.Dotfiles, len(*dotfiles))
// 	for dotfileName, dotfile := range *dotfiles {
// 		res[dotfileName] = toProtoDotfiles(&dotfile)
// 	}
// 	return res
// }

// func toProtoDotfiles(dotfile *config.Dotfiles) *pb.Dotfiles {
// 	return &pb.Dotfiles{
// 		DotfileLocation: dotfile.DotfileLocation,
// 		Requires:        toProtoDependencies(&dotfile.Requires),
// 	}
// }
