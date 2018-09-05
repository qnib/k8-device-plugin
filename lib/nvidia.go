package qniblib


import (
	"io/ioutil"
	"strings"
	"regexp"
	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"

)

func getNvidiaDevices() (devs []*pluginapi.Device, err error) {
	files, err := ioutil.ReadDir("/host/dev/")
	if err != nil {
		return
	}
	for _, f := range files {
		if strings.HasPrefix(f.Name(), "nvidia") {
			match, _ := regexp.MatchString(`nvidia\d+$`, f.Name())
			if ! match { continue }
			devs = append(devs, &pluginapi.Device{ID: f.Name(), Health: "healthy"})
		}
	}
	return
}
