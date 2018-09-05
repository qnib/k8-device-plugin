package qniblib

import (
	"fmt"
	"log"
	"github.com/google/gousb"
	pluginapi "k8s.io/kubernetes/pkg/kubelet/apis/deviceplugin/v1beta1"

)

func getUsbDevices() (result []*pluginapi.Device, err error) {
	result = []*pluginapi.Device{}
	ctx := gousb.NewContext()
	defer ctx.Close()

	devs, err := ctx.OpenDevices(func(desc *gousb.DeviceDesc) bool {
		devId := fmt.Sprintf("%s:%s", desc.Vendor.String(), desc.Product.String())
		//fmt.Printf("%03d.%03d %s:%s %s || %s\n", desc.Bus, desc.Address, desc.Vendor, desc.Product, usbid.Describe(desc), devId)
		result = append(result, &pluginapi.Device{ID: devId, Health: "healthy"})
		return false
	})

	defer func() {
		for _, d := range devs {
			d.Close()
		}
	}()
	if err != nil {
		log.Fatalf("list: %s", err)
	}
	return
}