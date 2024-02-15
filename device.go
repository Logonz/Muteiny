package main

import (
	"fmt"
	"os"

	"github.com/go-ole/go-ole"
	"github.com/moutend/go-wca/pkg/wca"
)

// Used to store the name of the selected input device
// Which is used to restore the mute state
var usedDevices map[string]bool = make(map[string]bool)

var _lastDeviceName string = "Unknown"

func SetDefaultDeviceName(name string) {
	// inputDeviceMenu is of type *systray.MenuItem
	if inputDeviceMenu != nil {
		inputDeviceMenu.SetTitle(name)
	}
}

func InitOLE() {
	if err := ole.CoInitializeEx(0, ole.COINIT_MULTITHREADED); err != nil {
		fmt.Println("Error initializing COM", err)
		os.Exit(1)
	}
}

func GetAllDevices() (map[string]*wca.IAudioEndpointVolume, func()) {
	var devices map[string]*wca.IAudioEndpointVolume = make(map[string]*wca.IAudioEndpointVolume)
	var releaseFuncs []func()

	var mmde *wca.IMMDeviceEnumerator
	if err := wca.CoCreateInstance(wca.CLSID_MMDeviceEnumerator, 0, wca.CLSCTX_ALL, wca.IID_IMMDeviceEnumerator, &mmde); err != nil {
		fmt.Println("Error creating device enumerator", err)
		os.Exit(1)
	}

	var pDevices *wca.IMMDeviceCollection
	if err := mmde.EnumAudioEndpoints(wca.ECapture, wca.DEVICE_STATE_ACTIVE, &pDevices); err != nil {
		fmt.Println("Error enumerating devices", err)
		os.Exit(1)
	}

	var count uint32
	if err := pDevices.GetCount(&count); err != nil {
		fmt.Println("Error getting device count", err)
		os.Exit(1)
	}
	releaseFuncs = append(releaseFuncs, func() {
		mmde.Release()
		pDevices.Release()
	})

	for i := uint32(0); i < count; i++ {
		var pDevice *wca.IMMDevice
		if err := pDevices.Item(i, &pDevice); err != nil {
			fmt.Println("Error getting device", err)
			os.Exit(1)
		}

		// Do something with pDevice
		var ps *wca.IPropertyStore
		if err := pDevice.OpenPropertyStore(wca.STGM_READ, &ps); err != nil {
			fmt.Println("Error opening property store", err)
			os.Exit(1)
		}

		//? Get the name of the communication device
		var pv wca.PROPVARIANT
		if err := ps.GetValue(&wca.PKEY_Device_FriendlyName, &pv); err != nil {
			fmt.Println("Error getting device friendly name", err)
			os.Exit(1)
		}
		inputDevice := fmt.Sprint(pv.String())

		var aev *wca.IAudioEndpointVolume
		if err := pDevice.Activate(wca.IID_IAudioEndpointVolume, wca.CLSCTX_ALL, nil, &aev); err != nil {
			fmt.Println("Error activating audio endpoint", err)
			os.Exit(1)
		}
		// var mute bool
		// err := aev.GetMute(&mute)
		// if err != nil {
		// 	fmt.Println("Error getting mute", err)
		// 	os.Exit(1)
		// }
		// fmt.Println(inputDevice, mute)
		releaseFuncs = append(releaseFuncs, func() {
			pDevice.Release()
			ps.Release()
			aev.Release()
		})
		devices[inputDevice] = aev
	}

	return devices, func() {
		// This is releases all the devices
		for _, fn := range releaseFuncs {
			fn()
		}
	}
}

func GetDefaultDevice() (*wca.IAudioEndpointVolume, func()) {
	// //? Here start the fetching of the default communications device
	// if err := ole.CoInitializeEx(0, ole.COINIT_MULTITHREADED); err != nil {
	// 	fmt.Println("Error initializing COM", err)
	// 	os.Exit(1)
	// }
	// if err := ole.CoInitialize(0); err != nil {
	// 	fmt.Println("Error initializing COM", err)
	// 	os.Exit(1)
	// }

	var mmde *wca.IMMDeviceEnumerator
	if err := wca.CoCreateInstance(wca.CLSID_MMDeviceEnumerator, 0, wca.CLSCTX_ALL, wca.IID_IMMDeviceEnumerator, &mmde); err != nil {
		fmt.Println("Error creating device enumerator", err)
		os.Exit(1)
	}

	//? Get the default communications device
	var mmd *wca.IMMDevice
	if err := mmde.GetDefaultAudioEndpoint(wca.ECapture, wca.DEVICE_STATE_ACTIVE, &mmd); err != nil {
		fmt.Println("Error getting default audio endpoint", err)
		os.Exit(1)
	}

	var ps *wca.IPropertyStore
	if err := mmd.OpenPropertyStore(wca.STGM_READ, &ps); err != nil {
		fmt.Println("Error opening property store", err)
		os.Exit(1)
	}

	//? Get the name of the communication device
	var pv wca.PROPVARIANT
	if err := ps.GetValue(&wca.PKEY_Device_FriendlyName, &pv); err != nil {
		fmt.Println("Error getting device friendly name", err)
		os.Exit(1)
	}

	_lastDeviceName = fmt.Sprint(pv.String())
	fmt.Printf("Input Device: %s\n", _lastDeviceName)
	// Set the device as used, used when restoring mute state
	usedDevices[_lastDeviceName] = true
	SetDefaultDeviceName(_lastDeviceName)

	//? Get the audio endpoint to control the settings of the device.
	var aev *wca.IAudioEndpointVolume
	if err := mmd.Activate(wca.IID_IAudioEndpointVolume, wca.CLSCTX_ALL, nil, &aev); err != nil {
		fmt.Println("Error activating audio endpoint", err)
		os.Exit(1)
	}
	return aev,
		func() {
			// defer ole.CoUninitialize()
			defer mmde.Release()
			defer mmd.Release()
			defer ps.Release()
			defer aev.Release()
		}
}
