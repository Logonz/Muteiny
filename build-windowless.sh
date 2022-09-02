#!/bin/bash

#Generates Windows icons
$GOPATH/bin/2goarray MicMute icons < icons/mic-mute.ico > icons/mic-mute.go
$GOPATH/bin/2goarray Mic icons < icons/mic.ico > icons/mic.go

# This script is used to build the windowless version of the application
go build -ldflags -H=windowsgui