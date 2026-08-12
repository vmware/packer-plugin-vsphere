// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

//go:generate packer-sdc struct-markdown
//go:generate packer-sdc mapstructure-to-hcl2 -type WatchSourceConfig

package supervisor

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha512"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/url"
	"path"
	"sync/atomic"
	"time"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/retry"
	vmopv1 "github.com/vmware-tanzu/vm-operator/api/v1alpha3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	DefaultWatchTimeoutSec = 3600

	StateKeyVMIP          = "vm_ip"
	StateKeyCommunicateIP = "ip"
)

var (
	// IsWatchingVM tracks if the VM watch process has started.
	// Used only in tests to mock the VM watch process.
	IsWatchingVM atomic.Bool
	// IsWatchingWCR tracks if the WebConsoleRequest watch process has started.
	// Used only in tests to mock the WebConsoleRequest watch process.
	IsWatchingWCR atomic.Bool
)

type WatchSourceConfig struct {
	// The timeout in seconds to wait for the source VM to be ready. Defaults to `3600`.
	WatchSourceTimeoutSec int `mapstructure:"watch_source_timeout_sec"`
}

func (c *WatchSourceConfig) Prepare() []error {
	if c.WatchSourceTimeoutSec == 0 {
		c.WatchSourceTimeoutSec = DefaultWatchTimeoutSec
	}

	return nil
}

type StepWatchSource struct {
	Config *WatchSourceConfig

	SourceName, Namespace, ImageType string
	KubeWatchClient                  client.WithWatch
}

func (s *StepWatchSource) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	logger := state.Get("logger").(*PackerLogger)
	logger.Info("Waiting for the source VM to be ready...")

	var err error
	defer func() {
		if err != nil {
			state.Put("error", err)
		}
	}()

	if err = s.initStep(state); err != nil {
		return multistep.ActionHalt
	}

	timedCtx, cancel := context.WithTimeout(ctx, time.Duration(s.Config.WatchSourceTimeoutSec)*time.Second)
	defer cancel()

	vmIP := ""
	vmIP, err = s.waitForVMReady(timedCtx, logger)
	if err != nil {
		return multistep.ActionHalt
	}
	state.Put(StateKeyVMIP, vmIP)

	// Only get the VM ingress IP if the VM service has been created (i.e. communicator is not 'none').
	if state.Get(StateKeyVMServiceCreated) == true {
		ingressIP := ""
		ingressIP, err = s.getVMIngressIP(timedCtx, logger)
		if err != nil {
			return multistep.ActionHalt
		}
		state.Put(StateKeyCommunicateIP, ingressIP)
	}

	logger.Info("Source VM is now ready in Supervisor cluster")
	return multistep.ActionContinue
}

func (s *StepWatchSource) Cleanup(state multistep.StateBag) {}

func (s *StepWatchSource) initStep(state multistep.StateBag) error {
	if err := CheckRequiredStates(state,
		StateKeyKubeClient,
		StateKeySupervisorNamespace,
		StateKeySourceName,
		StateKeyVMImageType,
	); err != nil {
		return err
	}

	var (
		ok                               bool
		sourceName, namespace, imageType string
		kubeWatchClient                  client.WithWatch
	)

	if sourceName, ok = state.Get(StateKeySourceName).(string); !ok {
		return fmt.Errorf("failed to cast %s to type string", StateKeySourceName)
	}
	if namespace, ok = state.Get(StateKeySupervisorNamespace).(string); !ok {
		return fmt.Errorf("failed to cast %s to type string", StateKeySupervisorNamespace)
	}
	if kubeWatchClient, ok = state.Get(StateKeyKubeClient).(client.WithWatch); !ok {
		return fmt.Errorf("failed to cast %s to type client.WithWatch", StateKeyKubeClient)
	}
	if imageType, ok = state.Get(StateKeyVMImageType).(string); !ok {
		return fmt.Errorf("failed to cast %s to type string", StateKeyVMImageType)
	}

	s.SourceName = sourceName
	s.Namespace = namespace
	s.KubeWatchClient = kubeWatchClient
	s.ImageType = imageType

	return nil
}

func (s *StepWatchSource) waitForVMReady(ctx context.Context, logger *PackerLogger) (string, error) {
	vmWatch, err := s.KubeWatchClient.Watch(ctx, &vmopv1.VirtualMachineList{}, &client.ListOptions{
		FieldSelector: fields.OneTermEqualSelector("metadata.name", s.SourceName),
		Namespace:     s.Namespace,
	})

	if err != nil {
		logger.Error("Failed to watch the VM object in Supervisor cluster")
		return "", err
	}

	defer func() {
		vmWatch.Stop()
		IsWatchingVM.Store(false)
	}()

	IsWatchingVM.Store(true)

	var isoInfoDisplayed bool

	for {
		select {
		case event := <-vmWatch.ResultChan():
			if event.Object == nil {
				continue
			}

			vmObj, ok := event.Object.(*vmopv1.VirtualMachine)
			if !ok {
				continue
			}

			if vmObj.Status.PowerState != vmopv1.VirtualMachinePowerStateOn {
				continue
			}

			if vmObj.Status.Network == nil {
				continue
			}

			if s.ImageType == "ISO" && !isoInfoDisplayed {
				c, err := json.MarshalIndent(vmObj.Status.Network.Config, "", "    ")
				if err != nil {
					logger.Error("Failed to pretty print the network configuration info")
				}
				logger.Info("Use the following network configuration to install the guest OS\n%s", string(c))

				consoleURL, err := s.getVMWebConsoleRequestURL(ctx, logger)
				if err != nil {
					logger.Error("Failed to generate a web console URL for ISO VM")
					return "", err
				}

				logger.Info("Web console URL: %s", consoleURL)
				logger.Info("Use the above URL to complete the guest OS installation.")
				isoInfoDisplayed = true
			}

			if vmIP := vmObj.Status.Network.PrimaryIP4; vmIP != "" {
				logger.Info("Successfully obtained the source VM IP: %s", vmIP)
				return vmIP, nil
			}

		case <-ctx.Done():
			return "", fmt.Errorf("timed out watching for source VM's IP")
		}
	}
}

// getVMWebConsoleRequestURL generates a web console URL for the VM guest OS access.
// It creates a VirtualMachineWebConsoleRequest object and waits for it to be processed.
// Once ready, it decrypts the response and returns the URL.
func (s *StepWatchSource) getVMWebConsoleRequestURL(ctx context.Context, logger *PackerLogger) (string, error) {
	logger.Info("Generating a web console URL for VM guest OS access...")
	privateKey, publicKeyPem, err := generateKeyPair()
	if err != nil {
		return "", err
	}

	wcr := &vmopv1.VirtualMachineWebConsoleRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.SourceName,
			Namespace: s.Namespace,
		},
		Spec: vmopv1.VirtualMachineWebConsoleRequestSpec{
			Name:      s.SourceName,
			PublicKey: string(publicKeyPem),
		},
	}
	if err := s.KubeWatchClient.Create(ctx, wcr); err != nil {
		return "", err
	}

	timedCtx, cancel := context.WithTimeout(ctx, time.Duration(s.Config.WatchSourceTimeoutSec)*time.Second)
	defer cancel()

	wcr, err = s.waitForWCRStatusChange(timedCtx, logger)
	if err != nil {
		return "", err
	}

	return s.parseURL(wcr, privateKey)
}

func (s *StepWatchSource) waitForWCRStatusChange(ctx context.Context, logger *PackerLogger) (*vmopv1.VirtualMachineWebConsoleRequest, error) {
	watcher, err := s.KubeWatchClient.Watch(ctx, &vmopv1.VirtualMachineWebConsoleRequestList{}, &client.ListOptions{
		FieldSelector: fields.OneTermEqualSelector("metadata.name", s.SourceName),
		Namespace:     s.Namespace,
	})

	if err != nil {
		logger.Error("Failed to watch the WebConsoleRequest in Supervisor cluster")
		return nil, err
	}

	defer func() {
		watcher.Stop()
		IsWatchingWCR.Store(false)
	}()

	IsWatchingWCR.Store(true)

	for {
		select {
		case event := <-watcher.ResultChan():
			if event.Object == nil {
				continue
			}

			wcrObj, ok := event.Object.(*vmopv1.VirtualMachineWebConsoleRequest)
			if !ok {
				continue
			}

			wcrStatus := wcrObj.Status
			if wcrStatus.Response != "" {
				return wcrObj, nil
			}

		case <-ctx.Done():
			return nil, fmt.Errorf("timed out waiting for VirtualMachineWebConsoleRequest status to be processed")
		}
	}
}

func (s *StepWatchSource) parseURL(wcr *vmopv1.VirtualMachineWebConsoleRequest, privateKey *rsa.PrivateKey) (string, error) {
	decoded, err := base64.StdEncoding.DecodeString(wcr.Status.Response)
	if err != nil {
		return "", err
	}

	decrypted, err := rsa.DecryptOAEP(sha512.New(), rand.Reader, privateKey, decoded, nil)
	if err != nil {
		return "", err
	}

	wcrURL, err := url.Parse(string(decrypted))
	if err != nil {
		return "", fmt.Errorf("failed to decrypt the VirtualMachineWebConsoleRequest response URL: %w", err)
	}

	query := url.Values{
		"namespace": {wcr.Namespace},
		"uuid":      {string(wcr.UID)},
		"host":      {wcrURL.Hostname()},
		"port":      {wcrURL.Port()},
		"ticket":    {path.Base(wcrURL.Path)},
	}

	outputURL := url.URL{
		Scheme:     "https",
		Host:       wcr.Status.ProxyAddr,
		Path:       "vm/web-console",
		ForceQuery: true,
		RawQuery:   query.Encode(),
	}

	return outputURL.String(), nil
}

func (s *StepWatchSource) getVMIngressIP(ctx context.Context, logger *PackerLogger) (string, error) {
	logger.Info("Getting source VM ingress IP from the VMService object")

	vmServiceObj := &vmopv1.VirtualMachineService{}
	vmServiceObjKey := client.ObjectKey{
		Namespace: s.Namespace,
		Name:      s.SourceName,
	}

	var vmIngressIP string
	err := retry.Config{
		RetryDelay: func() time.Duration {
			return 5 * time.Second
		},
		ShouldRetry: func(err error) bool {
			return !errors.Is(err, context.DeadlineExceeded)
		},
	}.Run(ctx, func(ctx context.Context) error {

		if err := s.KubeWatchClient.Get(ctx, vmServiceObjKey, vmServiceObj); err != nil {
			logger.Error("Failed to get the VMService object in Supervisor cluster")
			return err
		}

		ingress := vmServiceObj.Status.LoadBalancer.Ingress
		if len(ingress) == 0 || ingress[0].IP == "" {
			logger.Info("VMService object's ingress IP is empty, continue checking...")
			return errors.New("vmservice object's ingress IP address is empty")
		}

		logger.Info("Successfully retrieved the source VM ingress IP: %s", ingress[0].IP)
		vmIngressIP = ingress[0].IP
		return nil
	})

	if err != nil {
		return "", fmt.Errorf("timed out checking for VMService object's ingress IP")
	}

	return vmIngressIP, nil
}

// generateKeyPair generates a new RSA key pair used for the web console URL.
func generateKeyPair() (*rsa.PrivateKey, []byte, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	publicKey := privateKey.PublicKey
	publicKeyPem := pem.EncodeToMemory(
		&pem.Block{
			Type:  "PUBLIC KEY",
			Bytes: x509.MarshalPKCS1PublicKey(&publicKey),
		},
	)

	return privateKey, publicKeyPem, nil
}
