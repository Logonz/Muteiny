#!/bin/bash

#Generates Windows icons
#? Ehh, this just stopped working... but the mic-mute.go and mic.go files are already in the repo
# $GOPATH/bin/2goarray MicMute icons < icons/mic-mute.ico > icons/mic-mute.go
# $GOPATH/bin/2goarray Mic icons < icons/mic.ico > icons/mic.go

# This script is used to build the windowless version of the application
go build -o build/Muteiny.exe -ldflags -H=windowsgui