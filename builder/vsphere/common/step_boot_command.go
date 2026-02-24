// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

//go:generate packer-sdc struct-markdown
package common

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/hashicorp/packer-plugin-sdk/bootcommand"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/hashicorp/packer-plugin-sdk/template/interpolate"
	"github.com/pkg/errors"
	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/driver"
	"golang.org/x/mobile/event/key"
)

type BootConfig struct {
	bootcommand.BootConfig `mapstructure:",squash"`
	// The IP address to use for the HTTP server to serve the `http_directory`.
	HTTPIP string `mapstructure:"http_ip"`
}

type bootCommandTemplateData struct {
	HTTPIP   string
	HTTPPort int
	Name     string
}

func (c *BootConfig) Prepare(ctx *interpolate.Context) []error {
	if c.BootWait == 0 {
		c.BootWait = 10 * time.Second
	}

	return c.BootConfig.Prepare(ctx)
}

type StepBootCommand struct {
	Config *BootConfig
	VMName string
	Ctx    interpolate.Context
}

func (s *StepBootCommand) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	debug := state.Get("debug").(bool)
	ui := state.Get("ui").(packersdk.Ui)
	vm := state.Get("vm").(*driver.VirtualMachineDriver)

	if s.Config.BootCommand == nil {
		return multistep.ActionContinue
	}

	// Wait for the vm to boot.
	if int64(s.Config.BootWait) > 0 {
		ui.Sayf("Waiting %s for boot...", s.Config.BootWait.String())
		select {
		case <-time.After(s.Config.BootWait):
			break
		case <-ctx.Done():
			return multistep.ActionHalt
		}
	}

	var pauseFn multistep.DebugPauseFn
	if debug {
		pauseFn = state.Get("pauseFn").(multistep.DebugPauseFn)
	}

	var ip string
	var err error
	port, ok := state.Get("http_port").(int)
	if !ok {
		ui.Error("error retrieving 'http_port' from state")
		return multistep.ActionHalt
	}

	// If the port is set, we will use the HTTP server to serve the boot command.
	if port > 0 {

		keys := []string{"http_bind_address", "http_interface", "http_ip"}
		for _, key := range keys {
			value, ok := state.Get(key).(string)
			if !ok || value == "" {
				continue
			}

			switch key {
			case "http_bind_address":
				ip = value
				log.Printf("Using IP address %s from %s.", ip, key)
			case "http_interface":
				ip, err = hostIP(value)
				if err != nil {
					err := fmt.Errorf("error using interface %s: %s", value, err)
					state.Put("error", err)
					ui.Errorf("%s", err)
					return multistep.ActionHalt
				}
				log.Printf("Using IP address %s from %s %s.", ip, key, value)
			case "http_ip":
				if err := ValidateHTTPAddress(value); err != nil {
					err := fmt.Errorf("error using IP address %s: %s", value, err)
					state.Put("error", err)
					ui.Errorf("%s", err)
					return multistep.ActionHalt
				}
				ip = value
				log.Printf("Using IP address %s from %s.", ip, key)
			}
		}

		// Check if the IP address was determined.
		if ip == "" {
			err := fmt.Errorf("error determining IP address")
			state.Put("error", err)
			ui.Errorf("%s", err)
			return multistep.ActionHalt
		}

		s.Ctx.Data = &bootCommandTemplateData{
			ip,
			port,
			s.VMName,
		}

		ui.Sayf("Serving HTTP requests at http://%v:%v/.", ip, port)
	}

	var keyAlt, keyCtrl, keyShift bool
	sendCodes := func(code key.Code, down bool) error {
		switch code {
		case key.CodeLeftAlt:
			keyAlt = down
		case key.CodeLeftControl:
			keyCtrl = down
		case key.CodeLeftShift:
			keyShift = down
		}

		shift := down
		if keyShift {
			shift = keyShift
		}

		_, err := vm.TypeOnKeyboard(driver.KeyInput{
			Scancode: code,
			Ctrl:     keyCtrl,
			Alt:      keyAlt,
			Shift:    shift,
		})
		if err != nil {
			// retry once if error
			ui.Errorf("error typing a boot command (code, down) `%d, %t`: %v", code, down, err)
			ui.Say("Trying boot command again...")
			time.Sleep(s.Config.BootGroupInterval)
			_, err = vm.TypeOnKeyboard(driver.KeyInput{
				Scancode: code,
				Ctrl:     keyCtrl,
				Alt:      keyAlt,
				Shift:    shift,
			})
			if err != nil {
				return fmt.Errorf("error typing a boot command (code, down) `%d, %t`: %w", code, down, err)
			}
		}
		return nil
	}
	d := bootcommand.NewUSBDriver(sendCodes, s.Config.BootGroupInterval)

	ui.Say("Typing boot command...")
	flatBootCommand := s.Config.FlatBootCommand()
	command, err := interpolate.Render(flatBootCommand, &s.Ctx)
	if err != nil {
		err := fmt.Errorf("error preparing boot command: %s", err)
		state.Put("error", err)
		ui.Errorf("%s", err)
		return multistep.ActionHalt
	}

	seq, err := bootcommand.GenerateExpressionSequence(command)
	if err != nil {
		err := fmt.Errorf("error generating boot command: %s", err)
		state.Put("error", err)
		ui.Errorf("%s", err)
		return multistep.ActionHalt
	}

	if err := seq.Do(ctx, d); err != nil {
		err := fmt.Errorf("error running boot command: %s", err)
		state.Put("error", err)
		ui.Errorf("%s", err)
		return multistep.ActionHalt
	}

	if pauseFn != nil {
		pauseFn(multistep.DebugLocationAfterRun, fmt.Sprintf("boot_command: %s", command), state)
	}

	return multistep.ActionContinue
}

func (s *StepBootCommand) Cleanup(_ multistep.StateBag) {}

func hostIP(ifname string) (string, error) {
	var addrs []net.Addr
	var err error

	if ifname != "" {
		iface, err := net.InterfaceByName(ifname)
		if err != nil {
			return "", err
		}
		addrs, err = iface.Addrs()
		if err != nil {
			return "", err
		}
	} else {
		addrs, err = net.InterfaceAddrs()
		if err != nil {
			return "", err
		}
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String(), nil // IPv4 address
			} else if ipnet.IP.To16() != nil {
				return ipnet.IP.String(), nil // IPv6 address
			}
		}
	}
	return "", errors.New("error returning host ip address")
}
