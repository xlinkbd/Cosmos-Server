package docker

import (	
	"github.com/azukaar/cosmos-server/src/utils" 
	"github.com/docker/docker/api/types"
	"os"
)

func BootstrapAllContainersFromTags() []error {
	errD := Connect()
	if errD != nil {
		return []error{errD}
	}

	errors := []error{}
	
	containers, err := DockerClient.ContainerList(DockerContext, types.ContainerListOptions{})
	if err != nil {
		utils.Error("Docker Container List", err)
		return []error{err}
	}

	for _, container := range containers {
		errB := BootstrapContainerFromTags(container.ID)
		if errB != nil {
			utils.Error("Bootstrap Container From Tags", errB)
			errors = append(errors, errB)
		}
	}

	return errors
}

func BootstrapContainerFromTags(containerID string) error {
	errD := Connect()
	if errD != nil {
		return errD
	}

	selfContainer := types.ContainerJSON{}
	if os.Getenv("HOSTNAME") != "" {
		var errS error 
		selfContainer, errS = DockerClient.ContainerInspect(DockerContext, os.Getenv("HOSTNAME"))
		if errS != nil {
			utils.Error("DockerContainerBootstrapSelfInspect", errS)
			return errS
		}
	}

	utils.Log("Bootstrap Container From Tags: " + containerID)

	container, err := DockerClient.ContainerInspect(DockerContext, containerID)
	if err != nil {
		utils.Error("DockerContainerBootstrapInspect", err)
		return err
	}

	needsUpdate := false

	if(IsLabel(container, "cosmos-force-network-secured")) {
		utils.Log(container.Name+": Checking Force network secured")

		// check if connected to bridge and to a cosmos network
		isCon := IsConnectedToNetwork(container, "bridge")
		isCosmosCon, _ := IsConnectedToASecureCosmosNetwork(selfContainer, container)
		
		if isCon || !isCosmosCon {
			utils.Log(container.Name+": Needs isolating on a secured network")
			needsRestart := false
			var errCT error
			if !isCosmosCon {
				needsRestart, errCT = ConnectToSecureNetwork(container)
				if errCT != nil {
					return errCT
				}
				if needsRestart {
					utils.Log(container.Name+": Will restart to apply changes")
					needsUpdate = true
				} else {
					utils.Log(container.Name+": Connected to new network")
				}
			}
			if !needsRestart && isCon {
				utils.Log(container.Name+": Disconnecting from bridge network")
				errDisc := DockerClient.NetworkDisconnect(DockerContext, "bridge", containerID, true) 
				if errDisc != nil {
					utils.Error("Docker Network Disconnect", errDisc)
					return errDisc
				}
			}
		}

		if(len(GetAllPorts(container)) > 0) {
			utils.Log("Removing unsecure ports bindings from "+container.Name)
			// remove all ports			
			UnexposeAllPorts(&container)
			needsUpdate = true
		}
	}
	
	if(needsUpdate) {
		_, errEdit := EditContainer(containerID, container)
		if errEdit != nil {
			utils.Error("Docker Boostrap, couldn't update container: ", errEdit)
			return errEdit
		}
		utils.Debug("Done updating Container From Tags after Bootstrapping: " + container.Name)
	}

	utils.Log("Done bootstrapping Container From Tags: " + container.Name)

	return nil
}