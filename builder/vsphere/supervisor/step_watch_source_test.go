// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package supervisor_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha512"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	vmopv1 "github.com/vmware-tanzu/vm-operator/api/v1alpha3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/supervisor"
)

func TestWatchSource_Prepare(t *testing.T) {
	config := &supervisor.WatchSourceConfig{}
	if errs := config.Prepare(); len(errs) != 0 {
		t.Fatalf("unexpected failure: expected success, but failed: %v", errs[0])
	}
	if config.WatchSourceTimeoutSec != supervisor.DefaultWatchTimeoutSec {
		t.Fatalf("Default timeout should be %d, but returned %d", supervisor.DefaultWatchTimeoutSec, config.WatchSourceTimeoutSec)
	}
}

func TestWatchSource_RunOVF(t *testing.T) {
	// Initialize the step with required configs.
	config := &supervisor.WatchSourceConfig{
		WatchSourceTimeoutSec: 60,
	}
	step := &supervisor.StepWatchSource{
		Config: config,
	}

	// Set up required state for running this step.
	testNamespace := "test-ns"
	testSourceName := "test-source"
	testVMIP := "1.2.3.4"
	testIngressIP := "5.6.7.8"
	vmObj := newFakeVMObj(testNamespace, testSourceName)
	vmServiceObj := newFakeVMServiceObj(testNamespace, testSourceName)
	kubeClient := newFakeKubeClient(vmObj, vmServiceObj)

	testWriter := new(bytes.Buffer)
	state := newBasicTestState(testWriter)
	state.Put(supervisor.StateKeyKubeClient, kubeClient)
	state.Put(supervisor.StateKeySupervisorNamespace, testNamespace)
	state.Put(supervisor.StateKeySourceName, testSourceName)
	state.Put(supervisor.StateKeyVMServiceCreated, true)
	state.Put(supervisor.StateKeyVMImageType, "OVF")

	// Run this step in a new goroutine as it contains a blocking 'watch' process.
	var wg sync.WaitGroup
	wg.Go(func() {
		action := step.Run(context.Background(), state)
		if action == multistep.ActionHalt {
			if rawErr, ok := state.GetOk("error"); ok {
				t.Errorf("unexpected error: %s", rawErr.(error))
			}
			t.Errorf("unexpected action: expected '%#v', but returned '%#v'", multistep.ActionContinue, action)
			return
		}

		// Check if all the required states are set correctly after the step is run.
		vmIP := state.Get(supervisor.StateKeyVMIP)
		if vmIP != testVMIP {
			t.Errorf("State %q should be %q, but returned %q", supervisor.StateKeyCommunicateIP, testVMIP, vmIP)
		}
		connectIP := state.Get(supervisor.StateKeyCommunicateIP)
		if connectIP != testIngressIP {
			t.Errorf("State %q should be %q, but returned %q", supervisor.StateKeyCommunicateIP, testIngressIP, connectIP)
		}

		// Check the output lines from the step runs.
		expectedOutput := []string{
			"Waiting for the source VM to be ready...",
			fmt.Sprintf("Successfully obtained the source VM IP: %s", testVMIP),
			"Getting source VM ingress IP from the VMService object",
			fmt.Sprintf("Successfully retrieved the source VM ingress IP: %s", testIngressIP),
			"Source VM is now ready in Supervisor cluster",
		}
		checkOutputLines(t, testWriter, expectedOutput)
	})

	// Wait for the watch to be established from Builder before updating the fake VM resource below.
	for i := 0; i < step.Config.WatchSourceTimeoutSec; i++ {
		if supervisor.IsWatchingVM.Load() {
			break
		}
		time.Sleep(time.Second)
	}

	// Update the VM resource in the order of poweredOff => poweredOn => IP assigned.
	// In this way, we can test out the VM watch functionality and all output messages.
	ctx := context.Background()
	opt := &client.UpdateOptions{}

	vmObj.Status.PowerState = vmopv1.VirtualMachinePowerStateOff
	_ = kubeClient.Update(ctx, vmObj, opt)

	vmObj.Status.PowerState = vmopv1.VirtualMachinePowerStateOn
	_ = kubeClient.Update(ctx, vmObj, opt)

	vmObj.Status.Network = &vmopv1.VirtualMachineNetworkStatus{
		PrimaryIP4: testVMIP,
	}
	_ = kubeClient.Update(ctx, vmObj, opt)

	vmServiceObj.Status.LoadBalancer.Ingress = []vmopv1.LoadBalancerIngress{
		{
			IP: testIngressIP,
		},
	}
	_ = kubeClient.Update(ctx, vmServiceObj, opt)

	wg.Wait()
}

func TestWatchSource_RunISO(t *testing.T) {
	// Initialize the step with required configs.
	config := &supervisor.WatchSourceConfig{
		WatchSourceTimeoutSec: 60,
	}
	step := &supervisor.StepWatchSource{
		Config: config,
	}

	// Set up required state for running this step.
	testNamespace := "test-ns"
	testSourceName := "test-source"
	testVMIP := "1.2.3.4"
	vmObj := newFakeVMObj(testNamespace, testSourceName)
	kubeClient := newFakeKubeClient(vmObj)

	testWriter := new(bytes.Buffer)
	state := newBasicTestState(testWriter)
	state.Put(supervisor.StateKeyKubeClient, kubeClient)
	state.Put(supervisor.StateKeySupervisorNamespace, testNamespace)
	state.Put(supervisor.StateKeySourceName, testSourceName)
	state.Put(supervisor.StateKeyVMImageType, "ISO")

	vmNetworkConfig := &vmopv1.VirtualMachineNetworkConfigStatus{
		Interfaces: []vmopv1.VirtualMachineNetworkConfigInterfaceStatus{
			{
				Name: "test-interface",
				IP: &vmopv1.VirtualMachineNetworkConfigInterfaceIPStatus{
					Addresses: []string{testVMIP},
				},
			},
		},
	}

	// Run this step in a new goroutine as it contains a blocking 'watch' process.
	var wg sync.WaitGroup
	wg.Go(func() {
		action := step.Run(context.Background(), state)
		if action == multistep.ActionHalt {
			if rawErr, ok := state.GetOk("error"); ok {
				t.Errorf("unexpected error: %s", rawErr.(error))
			}
			t.Errorf("unexpected action: expected '%#v', but returned '%#v'", multistep.ActionContinue, action)
			return
		}

		// Check if all the required states are set correctly after the step is run.
		vmIP := state.Get(supervisor.StateKeyVMIP)
		if vmIP != testVMIP {
			t.Errorf("State %q should be %q, but returned %q", supervisor.StateKeyCommunicateIP, testVMIP, vmIP)
		}

		c, err := json.MarshalIndent(vmNetworkConfig, "", "")
		if err != nil {
			t.Errorf("Failed to marshal indent the VM network config info: %v", err)
			return
		}

		// Check the output lines from the step runs.
		expectedOutput := []string{
			"Waiting for the source VM to be ready...",
			"Use the following network configuration to install the guest OS",
		}
		networkConfigLines := strings.Split(string(c), "\n")
		expectedOutput = append(expectedOutput, networkConfigLines...)
		expectedOutput = append(expectedOutput, "Generating a web console URL for VM guest OS access...")
		expectedOutput = append(expectedOutput, "Web console URL: https://test-proxy-addr/vm/web-console?host=1.2.3.4&namespace=test-ns&port=80&ticket=test-ticket&uuid=test-uuid")
		expectedOutput = append(expectedOutput, "Use the above URL to complete the guest OS installation.")
		expectedOutput = append(expectedOutput, "Successfully obtained the source VM IP: 1.2.3.4")
		expectedOutput = append(expectedOutput, "Source VM is now ready in Supervisor cluster")
		checkOutputLines(t, testWriter, expectedOutput)
	})

	// Wait for the watch to be established from Builder before updating the fake VM resource below.
	for i := 0; i < step.Config.WatchSourceTimeoutSec; i++ {
		if supervisor.IsWatchingVM.Load() {
			break
		}
		time.Sleep(time.Second)
	}

	// Update the VM resource in the order of poweredOff => poweredOn => IP assigned.
	// In this way, we can test out the VM watch functionality and all output messages.
	ctx := context.Background()
	opt := &client.UpdateOptions{}

	vmObj.Status.PowerState = vmopv1.VirtualMachinePowerStateOff
	_ = kubeClient.Update(ctx, vmObj, opt)

	vmObj.Status.PowerState = vmopv1.VirtualMachinePowerStateOn
	_ = kubeClient.Update(ctx, vmObj, opt)

	// Update VM Status config to display the ISO required network info.
	vmObj.Status.Network = &vmopv1.VirtualMachineNetworkStatus{
		Config: vmNetworkConfig,
	}
	_ = kubeClient.Update(ctx, vmObj, opt)

	// Wait for the watch to be established from Builder before updating the fake VM resource below.
	for i := 0; i < step.Config.WatchSourceTimeoutSec; i++ {
		if supervisor.IsWatchingWCR.Load() {
			break
		}
		time.Sleep(time.Second)
	}

	uuid := "test-uuid"
	responseURL := "wss://1.2.3.4:80/ticket/test-ticket"
	proxyAddr := "test-proxy-addr"
	expiryTime := time.Now().Add(time.Minute)
	if err := updateWcrStatus(ctx, kubeClient, testNamespace, testSourceName, uuid, responseURL, proxyAddr, expiryTime); err != nil {
		t.Fatalf("Failed to update the WCR status: %v", err)
	}

	vmObj.Status.Network.PrimaryIP4 = testVMIP
	_ = kubeClient.Update(ctx, vmObj, opt)

	wg.Wait()
}

func newFakeVMObj(namespace, name string) *vmopv1.VirtualMachine {
	return &vmopv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}
}

func newFakeVMServiceObj(namespace, name string) *vmopv1.VirtualMachineService {
	return &vmopv1.VirtualMachineService{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Status: vmopv1.VirtualMachineServiceStatus{
			LoadBalancer: vmopv1.LoadBalancerStatus{
				Ingress: []vmopv1.LoadBalancerIngress{
					{
						IP: "",
					},
				},
			},
		},
	}
}

// updateWcrStatus puts encrypted responseURL and expiryTime into the status of the given WebConsoleRequest object.
func updateWcrStatus(ctx context.Context, kubeClient client.Client, ns, name, uuid, responseURL, proxyAddr string, expiryTime time.Time) error {
	wcr := vmopv1.VirtualMachineWebConsoleRequest{}
	err := kubeClient.Get(ctx, client.ObjectKey{
		Namespace: ns,
		Name:      name,
	}, &wcr)
	if err != nil {
		return err
	}

	// Get the public key stored in the object's spec.
	block, _ := pem.Decode([]byte(wcr.Spec.PublicKey))
	publicKey, err := x509.ParsePKCS1PublicKey(block.Bytes)
	if err != nil {
		return err
	}

	encryptedResponse, err := rsa.EncryptOAEP(sha512.New(), rand.Reader, publicKey, []byte(responseURL), nil)
	if err != nil {
		return err
	}

	wcr.UID = types.UID(uuid)
	wcr.Status.ProxyAddr = proxyAddr
	wcr.Status.Response = base64.StdEncoding.EncodeToString(encryptedResponse)
	wcr.Status.ExpiryTime = metav1.NewTime(expiryTime)

	opt := &client.UpdateOptions{}
	return kubeClient.Update(ctx, &wcr, opt)
}
