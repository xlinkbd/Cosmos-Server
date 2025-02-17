package docker

import (
	"context"
	"errors"
	"time"
	"github.com/azukaar/cosmos-server/src/utils" 

	"github.com/docker/docker/client"
	// natting "github.com/docker/go-connections/nat"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types"
)

var DockerClient *client.Client
var DockerContext context.Context
var DockerNetworkName = "cosmos-network"

func getIdFromName(name string) (string, error) {
	containers, err := DockerClient.ContainerList(DockerContext, types.ContainerListOptions{})
	if err != nil {
		utils.Error("Docker Container List", err)
		return "", err
	}

	for _, container := range containers {
		if container.Names[0] == name {
			utils.Warn(container.Names[0] + " == " + name + " == " + container.ID)
			return container.ID, nil
		}
	}

	return "", errors.New("Container not found")
}

var DockerIsConnected = false

func Connect() error {
	if DockerClient != nil {
		// check if connection is still alive
		ping, err := DockerClient.Ping(DockerContext)
		if ping.APIVersion != "" && err == nil {
			DockerIsConnected = true
			return nil
		} else {
			DockerIsConnected = false
			DockerClient = nil
			utils.Error("Docker Connection died, will try to connect again", err)
		}
	}
	if DockerClient == nil {
		ctx := context.Background()
		client, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
		if err != nil {
			DockerIsConnected = false
			return err
		}
		defer client.Close()

		DockerClient = client
		DockerContext = ctx

		ping, err := DockerClient.Ping(DockerContext)
		if ping.APIVersion != "" && err == nil {
			DockerIsConnected = true
			utils.Log("Docker Connected")
		} else {
			DockerIsConnected = false
			utils.Error("Docker Connection - Cannot ping Daemon. Is it running?", nil)
			return errors.New("Docker Connection - Cannot ping Daemon. Is it running?")
		}
		
		// if running in Docker, connect to main network
		// if os.Getenv("HOSTNAME") != "" {
		// 	ConnectToNetwork(os.Getenv("HOSTNAME"))
		// }
	}

	return nil
}

func EditContainer(containerID string, newConfig types.ContainerJSON) (string, error) {
	DockerNetworkLock <- true
	defer func() { 
		<-DockerNetworkLock 
		utils.Debug("Unlocking EDIT Container")
	}()

	errD := Connect()
	if errD != nil {
		return "", errD
	}
	utils.Log("EditContainer - Container updating " + containerID)

	// get container informations
	// https://godoc.org/github.com/docker/docker/api/types#ContainerJSON
	oldContainer, err := DockerClient.ContainerInspect(DockerContext, containerID)

	if err != nil {
		return "", err
	}

	// if no name, use the same one, that will force Docker to create a hostname if not set
	newName := oldContainer.Name
	newConfig.Config.Hostname = newName

	// stop and remove container
	stopError := DockerClient.ContainerStop(DockerContext, containerID, container.StopOptions{})
	if stopError != nil {
		return "", stopError
	}

	removeError := DockerClient.ContainerRemove(DockerContext, containerID, types.ContainerRemoveOptions{})
	if removeError != nil {
		return "", removeError
	}

	// wait for container to be destroyed
	//
	for {
		_, err := DockerClient.ContainerInspect(DockerContext, containerID)
		if err != nil {
			break
		} else {
			utils.Log("EditContainer - Waiting for container to be destroyed")
			time.Sleep(1 * time.Second)
		}
	}

	utils.Log("EditContainer - Container stopped " + containerID)

	// recreate container with new informations
	createResponse, createError := DockerClient.ContainerCreate(
		DockerContext,
		newConfig.Config,
		newConfig.HostConfig,
		nil,
		nil,
		newName,
	)

	// is force secure
	isForceSecure := newConfig.Config.Labels["cosmos-force-network-secured"] == "true"
	
	// re-connect to networks
	for networkName, _ := range oldContainer.NetworkSettings.Networks {
		if(isForceSecure && networkName == "bridge") {
			utils.Log("EditContainer - Skipping network " + networkName + " (cosmos-force-network-secured is true)")
			continue
		}
		errNet := ConnectToNetworkSync(networkName, createResponse.ID)
		if errNet != nil {
			utils.Error("EditContainer - Failed to connect to network " + networkName, errNet)
		} else {
			utils.Debug("EditContainer - New Container connected to network " + networkName)
		}
	}

	runError := DockerClient.ContainerStart(DockerContext, createResponse.ID, types.ContainerStartOptions{})

	if runError != nil {
		return "", runError
	}

	utils.Log("EditContainer - Container recreated " + createResponse.ID)

	if createError != nil {
		// attempt to restore container
		_, restoreError := DockerClient.ContainerCreate(DockerContext, oldContainer.Config, nil, nil, nil, oldContainer.Name)
		if restoreError != nil {
			utils.Error("EditContainer - Failed to restore Docker Container after update failure", restoreError)
		}

		return "", createError
	}

	return createResponse.ID, nil
}

func ListContainers() ([]types.Container, error) {
	errD := Connect()
	if errD != nil {
		return nil, errD
	}

	containers, err := DockerClient.ContainerList(DockerContext, types.ContainerListOptions{
		All: true,
	})
	if err != nil {
		return nil, err
	}

	// for _, container := range containers {
	// 	fmt.Println("ID - ", container.ID)
	// 	fmt.Println("ID - ", container.Names)
	// 	fmt.Println("ID - ", container.Image)
	// 	fmt.Println("ID - ", container.Command)
	// 	fmt.Println("ID - ", container.State)
	// 	fmt.Println("Ports - ", container.Ports)
	// 	fmt.Println("HostConfig - ", container.HostConfig)
	// 	fmt.Println("ID - ", container.Labels)
	// 	fmt.Println("NetworkSettings - ", container.NetworkSettings)
	// 	if(container.NetworkSettings.Networks["cosmos-network"] != nil) {
	// 		fmt.Println("IP COSMOS - ", container.NetworkSettings.Networks["cosmos-network"].IPAddress);
	// 	}
	// 	if(container.NetworkSettings.Networks["bridge"] != nil) {
	// 		fmt.Println("IP bridge - ", container.NetworkSettings.Networks["bridge"].IPAddress);
	// 	}
	// }

	return containers, nil
}

func AddLabels(containerConfig types.ContainerJSON, labels map[string]string) error {
	for key, value := range labels {
		containerConfig.Config.Labels[key] = value
	}

	return nil
}

func RemoveLabels(containerConfig types.ContainerJSON, labels []string) error {
	for _, label := range labels {
		delete(containerConfig.Config.Labels, label)
	}

	return nil
}

func IsLabel(containerConfig types.ContainerJSON, label string) bool {
	if containerConfig.Config.Labels[label] == "true" {
		return true
	}
	return false
}
func HasLabel(containerConfig types.ContainerJSON, label string) bool {
	if containerConfig.Config.Labels[label] != "" {
		return true
	}
	return false
}
func GetLabel(containerConfig types.ContainerJSON, label string) string {
	return containerConfig.Config.Labels[label]
}

func Test() error {

	// connect()

	// jellyfin, _ := DockerClient.ContainerInspect(DockerContext, "jellyfin")
	// ports := GetAllPorts(jellyfin)
	// fmt.Println(ports)

	// json jellyfin

	// fmt.Println(jellyfin.NetworkSettings)

	return nil
}