package qniblib

import (
	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"
	"time"
	"path"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"net"
	"os"
	"log"
	"strings"
	"fmt"
)

const (
	resourceName           = "qnib.org/usb"
	serverSock             = pluginapi.DevicePluginPath + "qnib.sock"
	envDisableHealthChecks = "DP_DISABLE_HEALTHCHECKS"
)

// QnibDevicePlugin implements the Kubernetes device plugin API
type QnibDevicePlugin struct {
	devs   []*pluginapi.Device
	socket string

	stop   chan interface{}
	health chan *pluginapi.Device

	server *grpc.Server
}

// NewQnibDevicePlugin returns an initialized NvidiaDevicePlugin
func NewQnibDevicePlugin() *QnibDevicePlugin {
	return &QnibDevicePlugin{
		devs:   GetDevices(),
		socket: serverSock,

		stop:   make(chan interface{}),
		health: make(chan *pluginapi.Device),
	}
}


func GetDevices() []*pluginapi.Device {
	devs, err := getNvidiaDevices()
	if err != nil {
		devs = []*pluginapi.Device{}
	}
	strDevs := []string{}
	for _, d := range devs {
		strDevs = append(strDevs, d.ID)
	}
	log.Printf("Devices found: %s\n", strings.Join(strDevs, ","))
	return devs
}

// ListAndWatch lists devices and update that list according to the health status
func (m *QnibDevicePlugin) ListAndWatch(e *pluginapi.Empty, s pluginapi.DevicePlugin_ListAndWatchServer) error {
	s.Send(&pluginapi.ListAndWatchResponse{Devices: m.devs})

	for {
		select {
		case <-m.stop:
			return nil
		case d := <-m.health:
			// FIXME: there is no way to recover from the Unhealthy state.
			d.Health = pluginapi.Unhealthy
			s.Send(&pluginapi.ListAndWatchResponse{Devices: m.devs})
		}
	}
}

func (m *QnibDevicePlugin) PreStartContainer(context.Context, *pluginapi.PreStartContainerRequest) (*pluginapi.PreStartContainerResponse, error) {
	return &pluginapi.PreStartContainerResponse{}, nil
}


func (m *QnibDevicePlugin) GetDevicePluginOptions(context.Context, *pluginapi.Empty) (*pluginapi.DevicePluginOptions, error) {
	return &pluginapi.DevicePluginOptions{}, nil
}


func (m *QnibDevicePlugin) Register(kubeletEndpoint, resourceName string) error {
	conn, err := Dial(kubeletEndpoint, 5*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()

	client := pluginapi.NewRegistrationClient(conn)
	reqt := &pluginapi.RegisterRequest{
		Version:      pluginapi.Version,
		Endpoint:     path.Base(m.socket),
		ResourceName: resourceName,
	}

	_, err = client.Register(context.Background(), reqt)
	if err != nil {
		return err
	}
	return nil
}

// Start starts the gRPC server of the device plugin
func (m *QnibDevicePlugin) Start() error {
	err := m.cleanup()
	if err != nil {
		return err
	}

	sock, err := net.Listen("unix", m.socket)
	if err != nil {
		return err
	}

	m.server = grpc.NewServer([]grpc.ServerOption{}...)
	pluginapi.RegisterDevicePluginServer(m.server, m)

	go m.server.Serve(sock)

	// Wait for server to start by launching a blocking connexion
	conn, err := dial(m.socket, 5*time.Second)
	if err != nil {
		return err
	}
	conn.Close()

	go m.healthcheck()

	return nil
}


// Stop stops the gRPC server
func (m *QnibDevicePlugin) Stop() error {
	if m.server == nil {
		return nil
	}

	m.server.Stop()
	m.server = nil
	close(m.stop)

	return m.cleanup()
}

// Allocate which return list of devices.
func (m *QnibDevicePlugin) Allocate(ctx context.Context, reqs *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error) {
	devs := m.devs
	responses := pluginapi.AllocateResponse{}
	for _, req := range reqs.ContainerRequests {
		response := pluginapi.ContainerAllocateResponse{
			Envs: map[string]string{
				"NVIDIA_VISIBLE_DEVICES": strings.Join(req.DevicesIDs, ","),
			},
		}

		for _, id := range req.DevicesIDs {
			if !deviceExists(devs, id) {
				return nil, fmt.Errorf("invalid allocation request: unknown device: %s", id)
			}
		}

		responses.ContainerResponses = append(responses.ContainerResponses, &response)
	}

	return &responses, nil
}

// Serve starts the gRPC server and register the device plugin to Kubelet
func (m *QnibDevicePlugin) Serve() error {
	err := m.Start()
	if err != nil {
		log.Printf("Could not start device plugin: %s", err)
		return err
	}
	log.Println("Starting to serve on", m.socket)

	err = m.Register(pluginapi.KubeletSocket, resourceName)
	if err != nil {
		log.Printf("Could not register device plugin: %s", err)
		m.Stop()
		return err
	}
	log.Println("Registered device plugin with Kubelet")

	return nil
}

func (m *QnibDevicePlugin) cleanup() error {
	if err := os.Remove(m.socket); err != nil && !os.IsNotExist(err) {
		return err
	}

	return nil
}


// dial establishes the gRPC communication with the registered device plugin.
func dial(unixSocketPath string, timeout time.Duration) (*grpc.ClientConn, error) {
	c, err := grpc.Dial(unixSocketPath, grpc.WithInsecure(), grpc.WithBlock(),
		grpc.WithTimeout(timeout),
		grpc.WithDialer(func(addr string, timeout time.Duration) (net.Conn, error) {
			return net.DialTimeout("unix", addr, timeout)
		}),
	)

	if err != nil {
		return nil, err
	}

	return c, nil
}


func (m *QnibDevicePlugin) healthcheck() {
	// TODO: Do healthchecks...
}