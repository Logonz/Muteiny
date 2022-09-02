package main

import (
	"Muteiny/icons"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"time"

	"github.com/getlantern/systray"
	"github.com/go-ole/go-ole"
	"github.com/moutend/go-hook/pkg/keyboard"
	"github.com/moutend/go-hook/pkg/mouse"
	"github.com/moutend/go-hook/pkg/types"
	"github.com/moutend/go-wca/pkg/wca"
)

type MuteFlag struct {
	Value bool
	IsSet bool
}

func (f *MuteFlag) Set(value string) (err error) {
	if value != "true" && value != "false" {
		err = fmt.Errorf("set 'true' or 'false'")
		return
	}
	if value == "true" {
		f.Value = true
	}
	f.IsSet = true
	return
}

func (f *MuteFlag) String() string {
	return fmt.Sprintf("%v", f.Value)
}

func SetMute(aev *wca.IAudioEndpointVolume, mute bool) error {
	var currentMute bool
	if err := aev.GetMute(&currentMute); err != nil {
		return err
	}
	if currentMute != mute {
		if err := aev.SetMute(mute, nil); err != nil {
			return err
		}
		if !mute {
			systray.SetTemplateIcon(icons.Mic, icons.Mic)
		} else {
			systray.SetTemplateIcon(icons.MicMute, icons.MicMute)
		}
		fmt.Printf("Mute State set to:%v\n", mute)
	}
	return nil
}

type KeyboardFlag struct {
	Value string
	IsSet bool
}

func (f *KeyboardFlag) Set(value string) (err error) {
	f.Value = value
	f.IsSet = true
	return
}

func (f *KeyboardFlag) String() string {
	return fmt.Sprintf("%v", f.Value)
}

type MouseFlag struct {
	Value int
	IsSet bool
}

func (f *MouseFlag) Set(value string) (err error) {
	f.Value, _ = strconv.Atoi(value)
	f.IsSet = true
	return
}

func (f *MouseFlag) String() string {
	return fmt.Sprintf("%v", f.Value)
}

type HoldFlag struct {
	Value int
	IsSet bool
}

func (f *HoldFlag) Set(value string) (err error) {
	f.Value, _ = strconv.Atoi(value)
	f.IsSet = true
	return
}

func (f *HoldFlag) String() string {
	return fmt.Sprintf("%v", f.Value)
}

var keyboardFlag KeyboardFlag
var mouseDownFlag MouseFlag
var mouseUpFlag MouseFlag
var holdFlag HoldFlag = HoldFlag{Value: 150, IsSet: false}

func main() {

	log.SetFlags(0)
	log.SetPrefix("error: ")

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

	fmt.Println(keyboardFlag)

	if err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED); err != nil {
		return
	}
	defer ole.CoUninitialize()

	var mmde *wca.IMMDeviceEnumerator
	if err := wca.CoCreateInstance(wca.CLSID_MMDeviceEnumerator, 0, wca.CLSCTX_ALL, wca.IID_IMMDeviceEnumerator, &mmde); err != nil {
		return
	}
	defer mmde.Release()

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

	var pv wca.PROPVARIANT
	if err := ps.GetValue(&wca.PKEY_Device_FriendlyName, &pv); err != nil {
		return
	}

	fmt.Printf("%s\n", pv.String())

	var aev *wca.IAudioEndpointVolume
	if err := mmd.Activate(wca.IID_IAudioEndpointVolume, wca.CLSCTX_ALL, nil, &aev); err != nil {
		return
	}
	defer aev.Release()

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

	// go func() {
	// 	if err := runMouse(aev, 523, 524); err != nil {
	// 		log.Fatal(err)
	// 	}
	// }()

	if mouseDownFlag.IsSet && mouseUpFlag.IsSet {
		go func() {
			if err := runMouse(aev, mouseDownFlag.Value, mouseUpFlag.Value); err != nil {
				log.Fatal(err)
			}
		}()
	}

	if keyboardFlag.IsSet {
		go func() {
			if err := runKeyboard(aev, keyboardFlag.Value); err != nil {
				log.Fatal(err)
			}
		}()
	}

	systray.Run(onReady, onExit)

	// reader := bufio.NewReader(os.Stdin)
	// _, _, err := reader.ReadRune() // print out the unicode value i.e. A -> 65, a -> 97
	// if err != nil {
	// 	log.Fatal(err)
	// }

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
	systray.SetTemplateIcon(icons.Mic, icons.Mic)
	systray.SetTitle("Muteiny")
	systray.SetTooltip("Muteiny")
	if mouseDownFlag.IsSet && mouseUpFlag.IsSet {
		systray.AddMenuItem("MouseDown: "+fmt.Sprint(mouseDownFlag.Value), "Hooked Mouse Button Down")
		systray.AddMenuItem("MouseUp: "+fmt.Sprint(mouseUpFlag.Value), "Hooked Mouse Button Up")
	}
	if keyboardFlag.IsSet {
		systray.AddMenuItem("Hooked Key: '"+keyboardFlag.Value+"'", "Hooked Keyboard Button")
	}
	systray.AddMenuItem("Hold Time: "+fmt.Sprint(holdFlag.Value)+"ms", "Mic Hold Time")
	// go func() {
	// 	<-test.ClickedCh
	// 	systray.SetTemplateIcon(icons.MicMute, icons.MicMute)
	// }()
	mQuitOrig := systray.AddMenuItem("Quit", "Quit Muteify")
	go func() {
		<-mQuitOrig.ClickedCh
		fmt.Println("Requesting quit")
		systray.Quit()
		fmt.Println("Finished quitting")
	}()
}

func onExit() {
	// clean up here
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
