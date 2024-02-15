package main

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	// Import the MessageBox function from user32.dll
	user32         = windows.NewLazySystemDLL("user32.dll")
	procMessageBox = user32.NewProc("MessageBoxW")

	// Function to show a MessageBox
	// hWnd is the owner window, lpText is the message text, lpCaption is the window title, uType is the type of message box.
	MessageBox = func(hWnd uintptr, lpText, lpCaption string, uType uint) int {
		ret, _, _ := procMessageBox.Call(
			hWnd,
			uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(lpText))),
			uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(lpCaption))),
			uintptr(uType),
		)
		return int(ret)
	}
)

func InstanceMutex() func() {
	// Attempt to create a named mutex
	mutexName := syscall.StringToUTF16Ptr("Global\\MuteinyAppMutex")
	mutex, err := windows.CreateMutex(nil, false, mutexName)

	if err != nil {
		fmt.Println("Error creating mutex:", err)
		fmt.Println("Another instance of Muteiny is already running.")
		MessageBox(0, "Another instance of Muteiny is already running.", "Error: Muteiny", 0)
		os.Exit(1)
	}

	// If GetLastError returns ERROR_ALREADY_EXISTS, another instance is running
	if windows.GetLastError() == windows.ERROR_ALREADY_EXISTS {
		fmt.Println("Another instance of Muteiny is already running.")
		MessageBox(0, "Another instance of Muteiny is already running.", "Muteiny", 0)
		os.Exit(1)
	}
	return func() {
		fmt.Println("Closing 'Global\\MuteinyAppMutex' mutex")
		windows.CloseHandle(mutex)
	}
}
