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

// Reference to the input device menuitem to change the name of the selected input device
var inputDeviceMenu *systray.MenuItem

// Is systray active
var systrayActive bool

// Keep these as globals, simple program no real use to pass them around everywhere
var keyboardFlag KeyboardFlag
var mouseDownFlag MouseFlag
var mouseUpFlag MouseFlag
var mouseData MouseFlag
var holdFlag HoldFlag
var volumeFlag bool
var bindMode bool

// Locked the whole program, moved to the setmute function instead.
// func init() {
// 	runtime.LockOSThread()
// }

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

func GetMute(aev *wca.IAudioEndpointVolume) bool /*, error*/ {
	var mute bool
	if err := aev.GetMute(&mute); err != nil {
		fmt.Println("Error getting mute state, returning", err)
		return false //, err
	}
	return mute //, nil
}

func main() {
	// ? This is a mutex to prevent multiple instances of the program from running at the same time.
	closeMutex := InstanceMutex()
	defer closeMutex()

	// Store the original mute state of the devices
	deviceStatesMap := make(map[string]bool)

	// ? Set the flags
	log.SetFlags(0)
	log.SetPrefix("error: ")

	// * Load the args
	f := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	// * Keyboard
	f.Var(&keyboardFlag, "keybind", "Specify keybind in format VK_A")
	f.Var(&keyboardFlag, "k", "Alias of -keybind")
	// * Mouse
	f.Var(&mouseDownFlag, "mousedown", "Specify mouse keybind in format 523 (down) !set both mouse up and down for it to work!")
	f.Var(&mouseDownFlag, "md", "Alias of -mousedown")
	f.Var(&mouseUpFlag, "mouseup", "Specify mouse keybind in format 524 (up) !set both mouse up and down for it to work!")
	f.Var(&mouseUpFlag, "mu", "Alias of -mouseup")
	f.Var(&mouseData, "mousedata", "Specify mouse data in format 131072(mouse3)/65536(mouse4), else all data is accepted")
	f.Var(&mouseData, "mdata", "Alias of -mousedata")
	// * Hold time
	f.Var(&holdFlag, "holdtime", "Specify the time in milliseconds to keep the mic open after release (default 500)")
	f.Var(&holdFlag, "h", "Alias of -holdtime")
	// * Volume
	f.BoolVar(&volumeFlag, "volume", false, "Set the volume to 0 instead of muting (Note, this does not actually remove all sound, keeping this even though it isn't useful for the program anymore)")
	// * Bind mode
	f.BoolVar(&bindMode, "keybindmode", false, "Set the program to bind mode, this will not mute the mic but instead write the binds to the console/binds.log to help you find the correct VK/Mouse codes")
	f.Parse(os.Args[1:])

	if bindMode {
		fmt.Println("Bind mode active")
		// ? Set the flags to false so the program doesn't run the mute/volume mode
		volumeFlag = false
		keyboardFlag.IsSet = false
		mouseUpFlag.IsSet = false
		mouseDownFlag.IsSet = false
		mouseData.IsSet = false
		holdFlag.IsSet = false
		// ? Run the bind mode
		go findBindMode()
	} else {
		// ? Set the hold time to 500ms if it's not set
		if !holdFlag.IsSet {
			holdFlag.Set("500")
		}

		// Initialize OLE for this thread
		InitOLE()
		// defer ole.CoUninitialize()

		// ? Get all the devices and their mute state
		devices, releaseAll := GetAllDevices()
		for k, v := range devices {
			mute := GetMute(v)
			fmt.Printf("Device: %s Muted: %t\n", k, mute)
			deviceStatesMap[k] = mute
		}
		releaseAll()

		//? Fetch the default communications device
		aev, release := GetDefaultDevice()

		if !volumeFlag {
			if err := SetMute(aev, true); err != nil {
				fmt.Println("Error setting startup mute state", err)
				return
			}
		} else {
			if err := aev.SetMasterVolumeLevel(0, nil); err != nil {
				return
			}
			fmt.Println("Volume mode")
			fmt.Println("Setting volume to 0!")
		}

		if mouseDownFlag.IsSet && mouseUpFlag.IsSet {
			fmt.Println("Mouse mode active")
			go func() {
				InitOLE()
				if err := runMouse(aev, mouseDownFlag.Value, mouseUpFlag.Value); err != nil {
					log.Fatal(err)
				}
				ole.CoUninitialize()
			}()
		}

		if keyboardFlag.IsSet {
			fmt.Println("Keyboard mode active")
			go func() {
				InitOLE()
				if err := runKeyboard(aev, keyboardFlag.Value); err != nil { //? Mouse3 Down: 523, Mouse3 Up: 524
					log.Fatal(err)
				}
				ole.CoUninitialize()
			}()
		}
		release()
		ole.CoUninitialize()
	}

	go systray.Run(onReady, nil)

	for f := range mainfunc {
		f()
	}

	if !bindMode {
		InitOLE()
		if !volumeFlag {
			// Restore the original mute state of the devices
			fmt.Println("Setting mute to original state before shutdown!")
			devices, releaseAll := GetAllDevices()
			for deviceName, muteState := range deviceStatesMap {
				if usedDevices[deviceName] {
					if devices[deviceName] != nil {
						fmt.Println("Restoring mute state for:", deviceName, "to:", muteState)
						if err := SetMute(devices[deviceName], muteState); err != nil {
							fmt.Println("Error setting mute state for:", deviceName, err)
						}
					} else {
						fmt.Println("Device not found:", deviceName)
					}
				}
			}
			releaseAll()
		} else {
			aev, release := GetDefaultDevice()
			if err := aev.SetMasterVolumeLevel(volumeLevel, nil); err != nil {
				return
			}
			release()
			fmt.Println("Setting volume to original level before shutdown!")
		}
		ole.CoUninitialize()
	}
}

func onReady() {
	systrayActive = true
	systray.SetTemplateIcon(icons.MicMute, icons.MicMute)
	systray.SetTitle("Muteiny")
	systray.SetTooltip("Muteiny")

	//* A little hacky but add information about the program state through menuitems.
	if bindMode {
		systray.AddMenuItem("Bind Mode", "Bind Mode Active")
	} else {
		inputDeviceMenu = systray.AddMenuItem(_lastDeviceName, "Input Device")
	}
	if mouseDownFlag.IsSet && mouseUpFlag.IsSet {
		systray.AddMenuItem("MouseDown: "+fmt.Sprint(mouseDownFlag.Value), "Hooked Mouse Button Down")
		systray.AddMenuItem("MouseUp: "+fmt.Sprint(mouseUpFlag.Value), "Hooked Mouse Button Up")
	}
	if mouseData.IsSet {
		systray.AddMenuItem("MouseData: "+fmt.Sprint(mouseData.Value), "Hooked Mouse Data")
	}
	if keyboardFlag.IsSet {
		systray.AddMenuItem("Hooked Key: '"+keyboardFlag.Value+"'", "Hooked Keyboard Button")
	}
	if holdFlag.IsSet {
		systray.AddMenuItem("Hold Time: "+fmt.Sprint(holdFlag.Value)+"ms", "Mic Hold Time")
	}

	// Ctrl+C to quit
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)
	go func() {
		<-signalChan
		systrayActive = false
		fmt.Println("Received shutdown signal")
		close(mainfunc)
		fmt.Println("Requesting quit")
		systray.Quit()
		fmt.Println("Finished quitting")
	}()

	// Quit button
	mQuitOrig := systray.AddMenuItem("Quit", "Quit Muteify")
	go func() {
		<-mQuitOrig.ClickedCh
		systrayActive = false
		close(mainfunc)
		fmt.Println("Requesting quit")
		systray.Quit()
		fmt.Println("Finished quitting")
	}()
}

func SetMute(aev *wca.IAudioEndpointVolume, mute bool) error {
	var currentMute bool
	if err := aev.GetMute(&currentMute); err != nil {
		return err
	}
	if currentMute != mute {
		do(func() {
			runtime.LockOSThread()
			if err := aev.SetMute(mute, nil); err != nil {
				// fmt.Println("this row is required, wtf?") //? If this row is not here, the program will crash when you try to mute the mic (it is not needed in golang 1.16)
				return
			}
			runtime.UnlockOSThread()
		})
		if systrayActive {
			if !mute {
				systray.SetTemplateIcon(icons.Mic, icons.Mic)
			} else {
				systray.SetTemplateIcon(icons.MicMute, icons.MicMute)
			}
		}
		fmt.Printf("Mute State set to:%v\n", mute)
	}
	return nil
}

var volumeLevel float32

func SetVolumeLevel(aev *wca.IAudioEndpointVolume, volumeLevel float32) error {
	var currentVolumeLevel float32
	if err := aev.GetMasterVolumeLevel(&currentVolumeLevel); err != nil {
		return err
	}
	if currentVolumeLevel != volumeLevel {
		if err := aev.SetMasterVolumeLevel(volumeLevel, nil); err != nil {
			return err
		}
		if systrayActive {
			if volumeLevel != 0 {
				systray.SetTemplateIcon(icons.Mic, icons.Mic)
			} else {
				systray.SetTemplateIcon(icons.MicMute, icons.MicMute)
			}
		}
		fmt.Printf("Volume Level set to:%v\n", volumeLevel)
	}
	return nil
}

func runMouse(aevv *wca.IAudioEndpointVolume, mouseDown int, mouseUp int) error {

	mouseChan := make(chan types.MouseEvent, 1)

	if err := mouse.Install(nil, mouseChan); err != nil {
		return err
	}

	defer mouse.Uninstall()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)

	fmt.Println("Start capturing mouse input")

	for {
		select {
		case <-signalChan:
			fmt.Println("Shutting down mouse listener")
			return nil
		case m := <-mouseChan:
			//? Used to check for specific mouse data, eg. Mouse3 and Mouse4 have the same VK but different data
			if mouseData.IsSet {
				if mouseData.Value != int(m.MouseData) {
					continue
				}
			}

			// Check if the mouse event is the one we are looking for
			keyNumber := int(m.Message)
			if keyNumber == mouseDown {
				// We run this every time to make sure we have the correct device
				aev, release := GetDefaultDevice()
				fmt.Printf("Down VK:%v Data:%v\n", int(m.Message), int(m.MouseData))
				if !volumeFlag {
					SetMute(aev, false)
				} else {
					SetVolumeLevel(aev, volumeLevel)
				}
				release()
			} else if keyNumber == mouseUp {
				// We run this every time to make sure we have the correct device
				aev, release := GetDefaultDevice()
				fmt.Printf("Up VK:%v Data:%v\n", int(m.Message), int(m.MouseData))
				// ? We run this goroutine because otherwise we lock the main thread causing lag due to the time.sleep
				go func() {
					time.Sleep(time.Duration(holdFlag.Value) * time.Millisecond)
					if !volumeFlag {
						SetMute(aev, true)
					} else {
						SetVolumeLevel(aev, 0)
					}
					release()
				}()
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
			fmt.Println("Shutting down keyboard listener")
			return nil
		case k := <-keyboardChan:
			// fmt.Printf("Received %v %v\n", k.Message, k.VKCode)
			if fmt.Sprint(k.VKCode) == keybind {
				if fmt.Sprint(k.Message) == "WM_KEYDOWN" {
					fmt.Printf("Down %v\n", k.VKCode)
					if !volumeFlag {
						SetMute(aev, false)
					} else {
						SetVolumeLevel(aev, volumeLevel)
					}
				} else if fmt.Sprint(k.Message) == "WM_KEYUP" {
					fmt.Printf("Up %v\n", k.VKCode)
					go func() {
						time.Sleep(time.Duration(holdFlag.Value) * time.Millisecond)
						if !volumeFlag {
							SetMute(aev, true)
						} else {
							SetVolumeLevel(aev, 0)
						}
					}()
				}
			}
			continue
		}
	}
}

func findBindMode() {
	//* findBindMode is a function that captures keyboard and mouse input and prints the corresponding key codes.
	//* It installs hooks for both mouse and keyboard events and listens for events until interrupted by a signal.
	//* The function prints the key codes for mouse events and key down/up events.
	//* To exit the function, press Ctrl+C.
	//* This function is used in bind mode to help find the correct VK/Mouse codes for keybinds.
	mouseChan := make(chan types.MouseEvent, 1)
	keyboardChan := make(chan types.KeyboardEvent, 1)

	if err := mouse.Install(nil, mouseChan); err != nil {
		log.Fatal(err)
	}
	if err := keyboard.Install(nil, keyboardChan); err != nil {
		log.Fatal(err)
	}

	defer mouse.Uninstall()
	defer keyboard.Uninstall()

	// Create a file to write the output
	file, err := os.Create("./binds.log")
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	fmt.Println("Start capturing keyboard and mouse input")
	go func() {
		signalChan := make(chan os.Signal, 1)
		signal.Notify(signalChan, os.Interrupt)
		for {
			select {
			case <-signalChan:
				fmt.Println("Shutting down mouse listener")
				return
			case m := <-mouseChan:
				keyNumber := int(m.Message)
				if keyNumber != 512 { //? This is mouse movement, we don't care about this
					fmt.Println("Mouse VK:", keyNumber, "Data:", int(m.MouseData))
					fmt.Fprintf(file, "Mouse VK: %d Data: %d\n", keyNumber, int(m.MouseData))
				}
				continue
			}
		}
	}()

	go func() {
		signalChan := make(chan os.Signal, 1)
		signal.Notify(signalChan, os.Interrupt)
		for {
			select {
			case <-signalChan:
				fmt.Println("Shutting down keyboard listener")
				return
			case k := <-keyboardChan:
				if fmt.Sprint(k.Message) == "WM_KEYDOWN" {
					fmt.Printf("Key Down VK %v %v\n", k.Message, k.VKCode)
					fmt.Fprintf(file, "Key Down VK %v %v\n", k.Message, k.VKCode)
				} else if fmt.Sprint(k.Message) == "WM_KEYUP" {
					fmt.Printf("Key Up %v %v\n", k.Message, k.VKCode)
					fmt.Fprintf(file, "Key Up %v %v\n", k.Message, k.VKCode)
				}
				continue
			}
		}
	}()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)
	fmt.Println("Press Ctrl+C to exit")
	<-signalChan
	fmt.Println("Stopped Bind Mode")
}

// Old code, kept for reference
// func SetMute(aev *wca.IAudioEndpointVolume, mute bool) error {
// 	var currentMute bool
// 	if err := aev.GetMute(&currentMute); err != nil {
// 		return err
// 	}
// 	if currentMute != mute {
// 		do(func() {
// 			runtime.LockOSThread()
// 			if err := aev.SetMute(mute, nil); err != nil {
// 				// fmt.Println("this row is required, wtf?") //? If this row is not here, the program will crash when you try to mute the mic (it is not needed in golang 1.16)
// 				return
// 			}
// 			runtime.UnlockOSThread()
// 		})
// 		if !mute {
// 			systray.SetTemplateIcon(icons.Mic, icons.Mic)
// 		} else {
// 			systray.SetTemplateIcon(icons.MicMute, icons.MicMute)
// 		}
// 		fmt.Printf("Mute State set to:%v\n", mute)
// 	}
// 	return nil
// }
