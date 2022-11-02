package main

import (
	"Muteiny/icons"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"time"

	"github.com/getlantern/systray"
	"github.com/go-ole/go-ole"
	"github.com/moutend/go-hook/pkg/keyboard"
	"github.com/moutend/go-hook/pkg/mouse"
	"github.com/moutend/go-hook/pkg/types"
	"github.com/moutend/go-wca/pkg/wca"
)

func SetMute(aev *wca.IAudioEndpointVolume, mute bool) error {
	var currentMute bool
	if err := aev.GetMute(&currentMute); err != nil {
		return err
	}
	if currentMute != mute {
		do(func() {
			if err := aev.SetMute(mute, nil); err != nil {
				// fmt.Println("this row is required, wtf?") //? If this row is not here, the program will crash when you try to mute the mic (it is not needed in golang 1.16)
				return
			}
		})
		if !mute {
			systray.SetTemplateIcon(icons.Mic, icons.Mic)
		} else {
			systray.SetTemplateIcon(icons.MicMute, icons.MicMute)
		}
		fmt.Printf("Mute State set to:%v\n", mute)
	}
	return nil
}

// Keep these as globals, simple program no real use to pass them around everywhere
var keyboardFlag KeyboardFlag
var mouseDownFlag MouseFlag
var mouseUpFlag MouseFlag
var holdFlag HoldFlag = HoldFlag{Value: 150, IsSet: false}

func init() {
	runtime.LockOSThread()
}

// queue of work to run in main thread.
var mainfunc = make(chan func())

// do runs f on the main thread.
func do(f func()) {
	done := make(chan bool, 1)
	mainfunc <- func() {
		f()
		done <- true
	}
	<-done
}

func main() {

	log.SetFlags(0)
	log.SetPrefix("error: ")

	// Load the args
	f := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	f.Var(&keyboardFlag, "keybind", "Specify keybind in format VK_A")
	f.Var(&keyboardFlag, "k", "Alias of -keybind")
	f.Var(&mouseDownFlag, "mousedown", "Specify mouse keybind in format 523 (down) !set both mouse up and down for it to work!")
	f.Var(&mouseDownFlag, "md", "Alias of -mousedown")
	f.Var(&mouseUpFlag, "mouseup", "Specify mouse keybind in format 524 (up) !set both mouse up and down for it to work!")
	f.Var(&mouseUpFlag, "mu", "Alias of -mouseup")
	f.Var(&holdFlag, "holdtime", "Specify the time in milliseconds to keep the mic open after release (default 150)")
	f.Var(&holdFlag, "h", "Alias of -holdtime")
	f.Parse(os.Args[1:])

	//? Here start the fetching of the default communications device
	if err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED); err != nil {
		return
	}
	defer ole.CoUninitialize()

	var mmde *wca.IMMDeviceEnumerator
	if err := wca.CoCreateInstance(wca.CLSID_MMDeviceEnumerator, 0, wca.CLSCTX_ALL, wca.IID_IMMDeviceEnumerator, &mmde); err != nil {
		return
	}
	defer mmde.Release()

	//? Get the default communications device
	var mmd *wca.IMMDevice
	if err := mmde.GetDefaultAudioEndpoint(wca.ECapture, wca.DEVICE_STATE_ACTIVE, &mmd); err != nil {
		return
	}
	defer mmd.Release()

	var ps *wca.IPropertyStore
	if err := mmd.OpenPropertyStore(wca.STGM_READ, &ps); err != nil {
		return
	}
	defer ps.Release()

	//? Get the name of the communication device
	var pv wca.PROPVARIANT
	if err := ps.GetValue(&wca.PKEY_Device_FriendlyName, &pv); err != nil {
		return
	}

	fmt.Printf("%s\n", pv.String())

	//? Get the audio endpoint to control the settings of the device.
	var aev *wca.IAudioEndpointVolume
	if err := mmd.Activate(wca.IID_IAudioEndpointVolume, wca.CLSCTX_ALL, nil, &aev); err != nil {
		return
	}
	defer aev.Release()

	// Mute the mic on startup
	var mute bool
	if err := aev.GetMute(&mute); err != nil {
		return
	}
	if !mute {
		if err := aev.SetMute(true, nil); err != nil {
			return
		}
		fmt.Println("Muting mic!")
	}

	if mouseDownFlag.IsSet && mouseUpFlag.IsSet {
		go func() {
			if err := runMouse(aev, mouseDownFlag.Value, mouseUpFlag.Value); err != nil {
				log.Fatal(err)
			}
		}()
	}

	if keyboardFlag.IsSet {
		go func() {
			if err := runKeyboard(aev, keyboardFlag.Value); err != nil { //? Mouse3 Down: 523, Mouse3 Up: 524
				log.Fatal(err)
			}
		}()
	}

	go systray.Run(onReady, nil)

	for f := range mainfunc {
		f()
	}

	//? Unmute the microphone on exit
	if err := aev.GetMute(&mute); err != nil {
		return
	}
	if mute {
		if err := aev.SetMute(false, nil); err != nil {
			return
		}
		fmt.Println("Unmuting mic before shutdown!")
	}
}

func onReady() {
	systray.SetTemplateIcon(icons.MicMute, icons.MicMute)
	systray.SetTitle("Muteiny")
	systray.SetTooltip("Muteiny")

	//* A little hacky but add information about the program state through menuitems.
	if mouseDownFlag.IsSet && mouseUpFlag.IsSet {
		systray.AddMenuItem("MouseDown: "+fmt.Sprint(mouseDownFlag.Value), "Hooked Mouse Button Down")
		systray.AddMenuItem("MouseUp: "+fmt.Sprint(mouseUpFlag.Value), "Hooked Mouse Button Up")
	}
	if keyboardFlag.IsSet {
		systray.AddMenuItem("Hooked Key: '"+keyboardFlag.Value+"'", "Hooked Keyboard Button")
	}
	systray.AddMenuItem("Hold Time: "+fmt.Sprint(holdFlag.Value)+"ms", "Mic Hold Time")

	mQuitOrig := systray.AddMenuItem("Quit", "Quit Muteify")
	go func() {
		<-mQuitOrig.ClickedCh
		close(mainfunc)
		fmt.Println("Requesting quit")
		systray.Quit()
		fmt.Println("Finished quitting")
	}()
}

func runMouse(aev *wca.IAudioEndpointVolume, mouseDown int, mouseUp int) error {
	mouseChan := make(chan types.MouseEvent, 1)

	if err := mouse.Install(nil, mouseChan); err != nil {
		return err
	}

	defer mouse.Uninstall()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)

	fmt.Println("Start capturing mouse input")

	var mute bool
	if err := aev.GetMute(&mute); err != nil {
		return err
	}
	fmt.Printf("Mute State: %v\n", mute)

	for {
		select {
		case <-signalChan:
			fmt.Println("Received shutdown signal")
			return nil
		case m := <-mouseChan:
			keyNumber := int(m.Message)
			if keyNumber == mouseDown {
				fmt.Printf("Down %v\n", int(m.Message))
				SetMute(aev, false)
			} else if keyNumber == mouseUp {
				fmt.Printf("Up %v\n", int(m.Message))
				time.Sleep(time.Duration(holdFlag.Value) * time.Millisecond)
				SetMute(aev, true)
			}
			continue
		}
	}
}

func runKeyboard(aev *wca.IAudioEndpointVolume, keybind string) error {
	keyboardChan := make(chan types.KeyboardEvent, 1)

	if err := keyboard.Install(nil, keyboardChan); err != nil {
		return err
	}

	defer keyboard.Uninstall()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)

	fmt.Println("Start capturing keyboard input")

	var mute bool
	if err := aev.GetMute(&mute); err != nil {
		return err
	}
	fmt.Printf("Mute State: %v\n", mute)

	for {
		select {
		case <-signalChan:
			fmt.Println("Received shutdown signal")
			return nil
		case k := <-keyboardChan:
			// fmt.Printf("Received %v %v\n", k.Message, k.VKCode)
			if fmt.Sprint(k.VKCode) == keybind {
				if fmt.Sprint(k.Message) == "WM_KEYDOWN" {
					fmt.Printf("Down %v\n", k.VKCode)
					SetMute(aev, false)
				} else if fmt.Sprint(k.Message) == "WM_KEYUP" {
					fmt.Printf("Up %v\n", k.VKCode)
					time.Sleep(time.Duration(holdFlag.Value) * time.Millisecond)
					SetMute(aev, true)
				}
			}
			continue
		}
	}
}
