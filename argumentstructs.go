package main

import (
	"fmt"
	"strconv"
)

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
