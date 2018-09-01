
package main

import (
	"log"
	"os"
	"syscall"
	"github.com/fsnotify/fsnotify"
	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"
	"github.com/qnib/k8-device-plugin/lib"
)

func main() {
	log.Println("Fetching devices.")
	if len(qniblib.GetDevices()) == 0 {
		log.Println("No devices found. Waiting indefinitely.")
		select {}
	}

	log.Println("Starting FS watcher.")
	watcher, err := qniblib.NewFSWatcher(pluginapi.DevicePluginPath)
	if err != nil {
		log.Println("Failed to created FS watcher.")
		os.Exit(1)
	}
	defer watcher.Close()

	log.Println("Starting OS watcher.")
	sigs := qniblib.NewOSWatcher(syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	restart := true
	var devicePlugin *qniblib.QnibDevicePlugin
L:
	for {
		if restart {
			if devicePlugin != nil {
				devicePlugin.Stop()
			}

			devicePlugin = qniblib.NewQnibDevicePlugin()
			if err := devicePlugin.Serve(); err != nil {
				log.Println("Could not contact Kubelet, retrying. Did you enable the device plugin feature gate?")
			} else {
				restart = false
			}
		}

		select {
		case event := <-watcher.Events:
			if event.Name == pluginapi.KubeletSocket && event.Op&fsnotify.Create == fsnotify.Create {
				log.Printf("inotify: %s created, restarting.", pluginapi.KubeletSocket)
				restart = true
			}

		case err := <-watcher.Errors:
			log.Printf("inotify: %s", err)

		case s := <-sigs:
			switch s {
			case syscall.SIGHUP:
				log.Println("Received SIGHUP, restarting.")
				restart = true
			default:
				log.Printf("Received signal \"%v\", shutting down.", s)
				devicePlugin.Stop()
				break L
			}
		}
	}
}
