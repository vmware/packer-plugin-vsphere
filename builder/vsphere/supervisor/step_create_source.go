// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

//go:generate packer-sdc struct-markdown
//go:generate packer-sdc mapstructure-to-hcl2 -type CreateSourceConfig

package supervisor

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/hashicorp/packer-plugin-sdk/communicator"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	vmopv1 "github.com/vmware-tanzu/vm-operator/api/v1alpha3"
	vmopv1common "github.com/vmware-tanzu/vm-operator/api/v1alpha3/common"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	DefaultSourceNamePrefix = "source"
	VMSelectorLabelKey      = DefaultSourceNamePrefix + "-selector"

	StateKeySourceName                = "source_name"
	StateKeyKeepInputArtifact         = "keep_input_artifact"
	StateKeyVMCreated                 = "vm_created"
	StateKeyVMServiceCreated          = "vm_service_created"
	StateKeyISOBootDiskPVCCreated     = "iso_boot_disk_pvc_created"
	StateKeyOVFBootstrapSecretCreated = "ovf_bootstrap_secret_created"
	StateKeyVMImageType               = "vm_image_type"

	ProviderCloudInit  = "CloudInit"
	ProviderSysprep    = "Sysprep"
	ProviderVAppConfig = "vAppConfig"
)

type CreateSourceConfig struct {
	// Name of the VM class that describes virtual hardware settings.
	ClassName string `mapstructure:"class_name" required:"true"`
	// Name of the storage class that configures storage-related attributes.
	StorageClass string `mapstructure:"storage_class" required:"true"`
	// Name of the source virtual machine (VM) image. If it is specified, the image with the name will be used for the
	// source VM, otherwise the image name from imported image will be used.
	ImageName string `mapstructure:"image_name"`
	// Name of the source VM. Limited to 15 characters. Defaults to `source-<random-5-digit-suffix>`.
	SourceName string `mapstructure:"source_name"`
	// Preserve all the created objects in Supervisor cluster after the build finishes. Defaults to `false`.
	KeepInputArtifact bool `mapstructure:"keep_input_artifact"`
	// Name of the bootstrap provider to use for configuring the source VM.
	// Supported values are `CloudInit`, `Sysprep`, and `vAppConfig`. Defaults to `CloudInit`.
	BootstrapProvider string `mapstructure:"bootstrap_provider"`
	// Path to a file with bootstrap configuration data. Required if `bootstrap_provider` is not set to `CloudInit`.
	// Defaults to a basic cloud config that sets up the user account from the SSH communicator config.
	BootstrapDataFile string `mapstructure:"bootstrap_data_file"`
	// The guest operating system identifier for the VM.
	// Defaults to `otherGuest`.
	GuestOSType string `mapstructure:"guest_os_type"`
	// Size of the PVC that will be used as the boot disk when deploying an ISO VM.
	// Supported units are `Gi`, `Mi`, `Ki`, `G`, `M`, `K`, etc.
	// Defaults to `20Gi`.
	IsoBootDiskSize string `mapstructure:"iso_boot_disk_size"`
}

func (c *CreateSourceConfig) Prepare() []error {
	var errs []error

	if c.ClassName == "" {
		errs = append(errs, fmt.Errorf("'class_name' is required for creating the source VM"))
	}
	if c.StorageClass == "" {
		errs = append(errs, fmt.Errorf("'storage_class' is required for creating the source VM"))
	}

	bp := c.BootstrapProvider
	if bp == "" {
		c.BootstrapProvider = ProviderCloudInit
	} else if bp != ProviderCloudInit && bp != ProviderSysprep && bp != ProviderVAppConfig {
		errs = append(errs, fmt.Errorf("'bootstrap_provider' must be one of %q, %q, %q",
			ProviderCloudInit, ProviderSysprep, ProviderVAppConfig))
	} else if bp != ProviderCloudInit && c.BootstrapDataFile == "" {
		errs = append(errs, fmt.Errorf("'bootstrap_data_file' is required when 'bootstrap_provider' is %q", bp))
	}

	if c.SourceName == "" {
		c.SourceName = fmt.Sprintf("%s-%s", DefaultSourceNamePrefix, rand.String(5))
	}

	if len(c.SourceName) > 15 {
		errs = append(errs, fmt.Errorf("'source_name' must not exceed 15 characters (length: %d): %s", len(c.SourceName), c.SourceName))
	}

	if c.GuestOSType == "" {
		c.GuestOSType = "otherGuest"
	}

	if c.IsoBootDiskSize == "" {
		c.IsoBootDiskSize = "20Gi"
	} else {
		_, err := resource.ParseQuantity(c.IsoBootDiskSize)
		if err != nil {
			errs = append(errs, fmt.Errorf("'iso_boot_disk_size' must be a valid quantity with units: %s", err))
		}
	}

	return errs
}

type StepCreateSource struct {
	Config             *CreateSourceConfig
	CommunicatorConfig *communicator.Config

	Namespace  string
	KubeClient client.Client
}

func (s *StepCreateSource) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	logger := state.Get("logger").(*PackerLogger)

	var err error
	defer func() {
		if err != nil {
			state.Put("error", err)
		}
	}()

	if err = s.initStep(state, logger); err != nil {
		return multistep.ActionHalt
	}

	logger.Info("Creating source objects with name %q in namespace %q", s.Config.SourceName, s.Namespace)

	// Check VM image (OVF or ISO) first to create required source objects accordingly.
	logger.Info("Checking source VM image %q", s.Config.ImageName)
	var imgKind, imgType string
	imgKind, imgType, err = s.getImageInfo(ctx, logger)
	if err != nil {
		return multistep.ActionHalt
	}
	state.Put(StateKeyVMImageType, imgType)

	switch imgType {
	case "ISO":
		logger.Info("Deploying VM from ISO image")
		if err = s.createISO(ctx, logger, state, imgKind); err != nil {
			return multistep.ActionHalt
		}
	case "OVF":
		logger.Info("Deploying VM from OVF image")
		if err = s.createOVF(ctx, logger, state); err != nil {
			return multistep.ActionHalt
		}
	default:
		logger.Error("Unsupported image type: %s", imgType)
		return multistep.ActionHalt
	}

	if s.CommunicatorConfig.Type == "none" {
		logger.Info("Skip creating VirtualMachineService as communicator type is 'none'")
	} else {
		if err = s.createVMService(ctx, logger); err != nil {
			return multistep.ActionHalt
		}
		state.Put(StateKeyVMServiceCreated, true)
	}

	// Make the source_name and keep_input_artifact retrievable in later step.
	state.Put(StateKeySourceName, s.Config.SourceName)
	state.Put(StateKeyKeepInputArtifact, s.Config.KeepInputArtifact)

	logger.Info("Finished creating all required source objects")
	return multistep.ActionContinue
}

func (s *StepCreateSource) Cleanup(state multistep.StateBag) {
	logger := state.Get("logger").(*PackerLogger)

	if s.Config.KeepInputArtifact {
		logger.Info("Skip cleaning up the source objects as specified in config")
		return
	}

	ctx := context.Background()
	objMeta := metav1.ObjectMeta{
		Name:      s.Config.SourceName,
		Namespace: s.Namespace,
	}
	if state.Get(StateKeyVMServiceCreated) == true {
		logger.Info("Deleting the VirtualMachineService object from Supervisor cluster")
		vmServiceObj := &vmopv1.VirtualMachineService{
			ObjectMeta: objMeta,
		}
		if err := s.KubeClient.Delete(ctx, vmServiceObj); err != nil {
			logger.Error("Failed to delete the VirtualMachineService object")
		} else {
			logger.Info("Successfully deleted the VirtualMachineService object")
		}
	}

	if state.Get(StateKeyVMCreated) == true {
		logger.Info("Deleting the VirtualMachine object from Supervisor cluster")
		vmObj := &vmopv1.VirtualMachine{
			ObjectMeta: objMeta,
		}
		if err := s.KubeClient.Delete(ctx, vmObj); err != nil {
			logger.Error("Failed to delete the VirtualMachine object")
		} else {
			logger.Info("Successfully deleted the VirtualMachine object")
		}
	}

	if state.Get(StateKeyOVFBootstrapSecretCreated) == true {
		logger.Info("Deleting the K8s Secret object from Supervisor cluster")
		secretObj := &corev1.Secret{
			ObjectMeta: objMeta,
		}
		err := s.KubeClient.Delete(ctx, secretObj)
		if err != nil {
			logger.Error("Failed to delete the K8s Secret object: %s", err)
		} else {
			logger.Info("Successfully deleted the K8s Secret object")
		}
	}

	if state.Get(StateKeyISOBootDiskPVCCreated) == true {
		logger.Info("Deleting the PVC object from Supervisor cluster")
		pvcObj := &corev1.PersistentVolumeClaim{
			ObjectMeta: objMeta,
		}
		if err := s.KubeClient.Delete(ctx, pvcObj); err != nil {
			logger.Error("Failed to delete the PVC object")
		} else {
			logger.Info("Successfully deleted the PVC object")
		}
	}
}

func (s *StepCreateSource) initStep(state multistep.StateBag, logger *PackerLogger) error {
	if err := CheckRequiredStates(state,
		StateKeyKubeClient,
		StateKeySupervisorNamespace,
	); err != nil {
		return err
	}

	var (
		ok         bool
		namespace  string
		kubeClient client.Client
	)

	importedImageName, _ := state.Get(StateKeyImportedImageName).(string)
	if s.Config.ImageName == "" {
		if importedImageName == "" {
			return fmt.Errorf("the image name should be specified in config 'image_name' or generated from image import")
		} else {
			s.Config.ImageName = importedImageName
		}
	} else if importedImageName != "" {
		// If both are set, the image name specified in the config will be used for the source image.
		logger.Info("The configured image with name %s will be used to create the source VirtualMachine object instead of the imported image %s",
			s.Config.ImageName, importedImageName)
	}

	if namespace, ok = state.Get(StateKeySupervisorNamespace).(string); !ok {
		return fmt.Errorf("failed to cast %q from state bag as type 'string'", StateKeySupervisorNamespace)
	}
	if kubeClient, ok = state.Get(StateKeyKubeClient).(client.Client); !ok {
		return fmt.Errorf("failed to cast %q from state bag as type 'client.Client'", StateKeyKubeClient)
	}

	s.Namespace = namespace
	s.KubeClient = kubeClient
	return nil
}

func (s *StepCreateSource) getImageInfo(ctx context.Context, logger *PackerLogger) (imgKind, imgType string, err error) {
	// First try to get VMI (namespaced scope).
	vmi := vmopv1.VirtualMachineImage{}
	if err = s.KubeClient.Get(ctx, client.ObjectKey{
		Namespace: s.Namespace,
		Name:      s.Config.ImageName,
	}, &vmi); err == nil {
		logger.Info("Found namespace scoped VM image of type %q", vmi.Status.Type)
		imgKind = vmi.Kind
		imgType = vmi.Status.Type
		return
	}

	if !apierrors.IsNotFound(err) {
		return
	}

	// VMI not found, try CVMI (cluster scope).
	cvmi := vmopv1.ClusterVirtualMachineImage{}
	if err = s.KubeClient.Get(ctx, client.ObjectKey{
		Name: s.Config.ImageName,
	}, &cvmi); err == nil {
		logger.Info("Found cluster scoped VM image of type %q", cvmi.Status.Type)
		imgKind = cvmi.Kind
		imgType = cvmi.Status.Type
		return
	}

	if !apierrors.IsNotFound(err) {
		return
	}

	// Neither VMI nor CVMI found.
	err = fmt.Errorf("source image %q not found", s.Config.ImageName)
	return
}

func (s *StepCreateSource) createISO(ctx context.Context, logger *PackerLogger, state multistep.StateBag, imgKind string) error {
	logger.Info("Creating a PVC object for ISO VM boot disk")
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.Config.SourceName,
			Namespace: s.Namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(s.Config.IsoBootDiskSize),
				},
			},
			StorageClassName: &s.Config.StorageClass,
		},
	}
	if err := s.KubeClient.Create(ctx, pvc); err != nil {
		return err
	}
	state.Put(StateKeyISOBootDiskPVCCreated, true)

	logger.Info("Creating a VM object with PVC and CD-ROM attached")
	vm := &vmopv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.Config.SourceName,
			Namespace: s.Namespace,
			Labels: map[string]string{
				VMSelectorLabelKey: s.Config.SourceName,
			},
		},
		Spec: vmopv1.VirtualMachineSpec{
			ClassName:    s.Config.ClassName,
			StorageClass: s.Config.StorageClass,
			Cdrom: []vmopv1.VirtualMachineCdromSpec{
				{
					Name: "cdrom",
					Image: vmopv1.VirtualMachineImageRef{
						Kind: imgKind,
						Name: s.Config.ImageName,
					},
					Connected:         &[]bool{true}[0],
					AllowGuestControl: &[]bool{true}[0],
				},
			},
			GuestID: s.Config.GuestOSType,
			Volumes: []vmopv1.VirtualMachineVolume{
				{
					Name: "vm-boot-disk",
					VirtualMachineVolumeSource: vmopv1.VirtualMachineVolumeSource{
						PersistentVolumeClaim: &vmopv1.PersistentVolumeClaimVolumeSource{
							PersistentVolumeClaimVolumeSource: corev1.PersistentVolumeClaimVolumeSource{
								ClaimName: s.Config.SourceName,
							},
						},
					},
				},
			},
		},
	}
	if err := s.KubeClient.Create(ctx, vm); err != nil {
		return err
	}
	state.Put(StateKeyVMCreated, true)

	return nil
}

func (s *StepCreateSource) createOVF(ctx context.Context, logger *PackerLogger, state multistep.StateBag) error {
	stringData, err := s.getBootstrapStringData(ctx, logger)
	if err != nil {
		logger.Error("Failed to get the bootstrap data from file %q", s.Config.BootstrapDataFile)
		return err
	}

	logger.Info("Creating a Secret object for OVF VM bootstrap")
	kubeSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.Config.SourceName,
			Namespace: s.Namespace,
		},
		StringData: stringData,
	}

	if err := s.KubeClient.Create(ctx, kubeSecret); err != nil {
		return err
	}
	state.Put(StateKeyOVFBootstrapSecretCreated, true)

	logger.Info("Creating a VM object with bootstrap provider %q", s.Config.BootstrapProvider)
	vm := &vmopv1.VirtualMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.Config.SourceName,
			Namespace: s.Namespace,
			Labels: map[string]string{
				VMSelectorLabelKey: s.Config.SourceName,
			},
		},
		Spec: vmopv1.VirtualMachineSpec{
			ImageName:    s.Config.ImageName,
			ClassName:    s.Config.ClassName,
			StorageClass: s.Config.StorageClass,
		},
	}

	bootstrap := &vmopv1.VirtualMachineBootstrapSpec{}
	switch s.Config.BootstrapProvider {
	case ProviderCloudInit:
		bootstrap.CloudInit = &vmopv1.VirtualMachineBootstrapCloudInitSpec{
			RawCloudConfig: &vmopv1common.SecretKeySelector{
				Key:  "user-data",
				Name: s.Config.SourceName,
			},
		}
	case ProviderSysprep:
		bootstrap.Sysprep = &vmopv1.VirtualMachineBootstrapSysprepSpec{
			RawSysprep: &vmopv1common.SecretKeySelector{
				Key:  "unattend",
				Name: s.Config.SourceName,
			},
		}
	case ProviderVAppConfig:
		bootstrap.VAppConfig = &vmopv1.VirtualMachineBootstrapVAppConfigSpec{
			RawProperties: s.Config.SourceName,
		}
	}
	vm.Spec.Bootstrap = bootstrap

	if err := s.KubeClient.Create(ctx, vm); err != nil {
		return err
	}
	state.Put(StateKeyVMCreated, true)

	return nil
}

func (s *StepCreateSource) getBootstrapStringData(ctx context.Context, logger *PackerLogger) (map[string]string, error) {
	if s.Config.BootstrapDataFile != "" {
		logger.Info("Loading bootstrap data from file: %s", s.Config.BootstrapDataFile)
		content, err := os.ReadFile(s.Config.BootstrapDataFile)
		if err != nil {
			return nil, err
		}
		var bootstrapData map[string]string
		err = yaml.Unmarshal(content, &bootstrapData)
		return bootstrapData, err
	}

	logger.Info("Using default cloud-init user data as the 'bootstrap_data_file' is not specified")

	cloudInitFmt := `#cloud-config
ssh_pwauth: true
users:
  - name: %s
    plain_text_passwd: %s
    lock_passwd: false
    sudo: ALL=(ALL) NOPASSWD:ALL
    shell: /bin/bash
    ssh_authorized_keys:
    - %s
`
	cloudInitStr := fmt.Sprintf(cloudInitFmt,
		s.CommunicatorConfig.SSHUsername,
		s.CommunicatorConfig.SSHPassword,
		strings.TrimSpace(string(s.CommunicatorConfig.SSHPublicKey)),
	)
	defaultData := map[string]string{
		"user-data": cloudInitStr,
	}

	return defaultData, nil
}

func (s *StepCreateSource) createVMService(ctx context.Context, logger *PackerLogger) error {
	logger.Info("Creating a VirtualMachineService object for network connection")

	var commPort int
	switch s.CommunicatorConfig.Type {
	case "ssh":
		commPort = s.CommunicatorConfig.SSHPort
	case "winrm":
		commPort = s.CommunicatorConfig.WinRMPort
	default:
		return fmt.Errorf("unsupported communicator type: %q", s.CommunicatorConfig.Type)
	}

	vmServiceObj := &vmopv1.VirtualMachineService{
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.Config.SourceName,
			Namespace: s.Namespace,
		},
		Spec: vmopv1.VirtualMachineServiceSpec{
			Type: vmopv1.VirtualMachineServiceTypeLoadBalancer,
			Ports: []vmopv1.VirtualMachineServicePort{
				{
					Name:       s.CommunicatorConfig.Type,
					Protocol:   "TCP",
					Port:       int32(commPort),
					TargetPort: int32(commPort),
				},
			},
			Selector: map[string]string{
				VMSelectorLabelKey: s.Config.SourceName,
			},
		},
	}

	if err := s.KubeClient.Create(ctx, vmServiceObj); err != nil {
		return err
	}

	return nil
}
