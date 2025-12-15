// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

//go:generate packer-sdc struct-markdown
//go:generate packer-sdc mapstructure-to-hcl2 -type Config

package vsphere

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/url"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/hashicorp/hcl/v2/hcldec"
	"github.com/hashicorp/packer-plugin-sdk/common"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/hashicorp/packer-plugin-sdk/retry"
	shelllocal "github.com/hashicorp/packer-plugin-sdk/shell-local"
	"github.com/hashicorp/packer-plugin-sdk/template/config"
	"github.com/hashicorp/packer-plugin-sdk/template/interpolate"
	vspherecommon "github.com/vmware/packer-plugin-vsphere/builder/vsphere/common"
	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/driver"
)

const (
	DefaultMaxRetries = 5
	DefaultDiskMode   = "thick"
	OvftoolWindows    = "ovftool.exe"
)

var ovftool = "ovftool"

var (
	// Regular expression to validate an RFC1035 hostname from an FQDN or simple
	// hostname. For example "esxi-01". Requires proper DNS setup and/or correct DNS
	// search domain setting.
	hostnameRegex = regexp.MustCompile(`^([[:alnum:]][[:alnum:]\-]{0,61}[[:alnum:]]|[[:alpha:]])$`)

	// Simple regular expression to validate IPv4 values.
	// For example "192.168.168.1".
	ipv4Regex = regexp.MustCompile(`^(?:[0-9]{1,3}\.){3}[0-9]{1,3}$`)
)

type Config struct {
	common.PackerConfig `mapstructure:",squash"`
	// The cluster or ESX host to upload the virtual machine.
	// This can be either the name of the vSphere cluster or the fully qualified domain name (FQDN)
	// or IP address of the ESX host.
	Cluster string `mapstructure:"cluster" required:"true"`
	// The name of the vSphere datacenter object to place the virtual machine.
	// This is _not required_ if `resource_pool` is specified.
	Datacenter string `mapstructure:"datacenter" required:"true"`
	// The name of the vSphere datastore to place the virtual machine.
	// Mutually exclusive with `datastore_cluster`.
	Datastore string `mapstructure:"datastore"`
	// The name of the vSphere datastore cluster to place the virtual machine.
	// When specified, Storage DRS will automatically select the optimal datastore.
	// Mutually exclusive with `datastore`.
	DatastoreCluster string `mapstructure:"datastore_cluster"`
	// The disk format of the target virtual machine. One of `thin` or `thick`.
	DiskMode string `mapstructure:"disk_mode"`
	// The fully qualified domain name or IP address of the vCenter instance or ESX host.
	Host string `mapstructure:"host" required:"true"`
	// The fully qualified domain name or IP address of the ESX host to upload the
	// virtual machine. This is _not required_ if `host` is a vCenter instance.
	ESXHost string `mapstructure:"esxi_host"`
	// Skip the verification of the server certificate. Defaults to `false`.
	Insecure bool `mapstructure:"insecure"`
	// Options to send to `ovftool` when uploading the virtual machine.
	// Use `ovftool --help` to list all the options available.
	Options []string `mapstructure:"options"`
	// Overwrite existing files. Defaults to `false`.
	Overwrite bool `mapstructure:"overwrite"`
	// The password to use to authenticate to the vSphere endpoint.
	Password string `mapstructure:"password" required:"true"`
	// The name of the resource pool to place the virtual machine.
	ResourcePool string `mapstructure:"resource_pool"`
	// The username to use to authenticate to the vSphere endpoint.
	Username string `mapstructure:"username" required:"true"`
	// The name of the virtual machine folder path where the virtual machine will be
	// placed.
	VMFolder string `mapstructure:"vm_folder"`
	// The name of the virtual machine to be created on the vSphere endpoint.
	VMName string `mapstructure:"vm_name"`
	// The name of the network in which to place the virtual machine.
	VMNetwork string `mapstructure:"vm_network"`
	// The maximum virtual hardware version for the deployed virtual machine.
	//
	// It does not upgrade the virtual hardware version of the source VM. Instead, it limits the
	// virtual hardware version of the deployed virtual machine  to the specified version.
	// If the source virtual machine's hardware version is higher than the specified version, the
	// deployed virtual machine's hardware version will be downgraded to the specified version.
	//
	// If the source virtual machine's hardware version is lower than or equal to the specified
	// version, the deployed virtual machine's hardware version will be the same as the source
	// virtual machine's.
	//
	// This option is useful when deploying to vCenter instance or an ESX host whose
	// version is different from the one used to create the artifact.
	//
	// Refer to [KB 315655](https://knowledge.broadcom.com/external/article?articleNumber=315655)
	// for more information on supported virtual hardware versions.
	HardwareVersion string `mapstructure:"hardware_version"`
	// The maximum number of times to retry the upload operation if it fails.
	// Defaults to `5`.
	MaxRetries int `mapstructure:"max_retries"`
	// Tags configuration for the virtual machine.
	vspherecommon.TagsConfig `mapstructure:",squash"`

	ctx interpolate.Context
}

type PostProcessor struct {
	config Config
}

func (p *PostProcessor) ConfigSpec() hcldec.ObjectSpec { return p.config.FlatMapstructure().HCL2Spec() }

func (p *PostProcessor) Configure(raws ...interface{}) error {
	err := config.Decode(&p.config, &config.DecodeOpts{
		PluginType:         BuilderId,
		Interpolate:        true,
		InterpolateContext: &p.config.ctx,
		InterpolateFilter: &interpolate.RenderFilter{
			Exclude: []string{},
		},
	}, raws...)
	if err != nil {
		return err
	}

	// Set default value for MaxRetries if not provided.
	if p.config.MaxRetries == 0 {
		p.config.MaxRetries = DefaultMaxRetries
	}

	// Defaults
	if p.config.DiskMode == "" {
		p.config.DiskMode = DefaultDiskMode
	}

	// Accumulate any errors.
	errs := new(packersdk.MultiError)

	if runtime.GOOS == "windows" {
		ovftool = OvftoolWindows
	}

	if _, err := exec.LookPath(ovftool); err != nil {
		errs = packersdk.MultiErrorAppend(
			errs, fmt.Errorf("ovftool not found: %s", err))
	}

	// Define the parameters that are required.
	templates := map[string]*string{
		"cluster":    &p.config.Cluster,
		"datacenter": &p.config.Datacenter,
		"diskmode":   &p.config.DiskMode,
		"host":       &p.config.Host,
		"password":   &p.config.Password,
		"username":   &p.config.Username,
		"vm_name":    &p.config.VMName,
	}
	for key, ptr := range templates {
		if *ptr == "" {
			errs = packersdk.MultiErrorAppend(
				errs, fmt.Errorf("%s must be set", key))
		}
	}

	// Validate tags configuration
	if len(p.config.Tags) > 0 || len(p.config.Tag) > 0 {
		tagManager := vspherecommon.NewTagManager(nil, &p.config.TagsConfig, p.config.ctx)
		if err := tagManager.ValidateConfig(); err != nil {
			errs = packersdk.MultiErrorAppend(errs, fmt.Errorf("tag configuration validation failed: %s", err))
		}
	}

	if len(errs.Errors) > 0 {
		return errs
	}

	return nil
}

func (p *PostProcessor) generateURI() (*url.URL, error) {
	// Use the net/url standard library to encode and escape the URI.
	ovftoolURI := fmt.Sprintf("vi://%s/%s/host/%s",
		p.config.Host,
		p.config.Datacenter,
		p.config.Cluster)

	if p.config.ResourcePool != "" {
		ovftoolURI += "/Resources/" + p.config.ResourcePool
	}

	u, err := url.Parse(ovftoolURI)
	if err != nil {
		return nil, fmt.Errorf("error generating uri for ovftool: %s", err)
	}
	u.User = url.UserPassword(p.config.Username, p.config.Password)

	if p.config.ESXHost != "" {
		q := u.Query()
		if ipv4Regex.MatchString(p.config.ESXHost) {
			q.Add("ip", p.config.ESXHost)
		} else if hostnameRegex.MatchString(p.config.ESXHost) {
			q.Add("dns", p.config.ESXHost)
		}
		u.RawQuery = q.Encode()
	}
	return u, nil
}

func getEncodedPassword(u *url.URL) (string, bool) {
	// Filter the password from the logs.
	password, passwordSet := u.User.Password()
	if passwordSet && password != "" {
		encodedPassword := strings.Split(u.User.String(), ":")[1]
		return encodedPassword, true
	}
	return password, false
}

func (p *PostProcessor) PostProcess(ctx context.Context, ui packersdk.Ui, artifact packersdk.Artifact) (packersdk.Artifact, bool, bool, error) {
	source := ""
	for _, path := range artifact.Files() {
		if strings.HasSuffix(path, ".vmx") || strings.HasSuffix(path, ".ovf") || strings.HasSuffix(path, ".ova") {
			source = path
			break
		}
	}

	if source == "" {
		return nil, false, false, fmt.Errorf("error locating expected .vmx, .ovf, or .ova artifact")
	}

	ovftoolURI, err := p.generateURI()
	if err != nil {
		return nil, false, false, err
	}
	encodedPassword, isSet := getEncodedPassword(ovftoolURI)
	if isSet {
		packersdk.LogSecretFilter.Set(encodedPassword)
	}

	args, err := p.BuildArgs(source, ovftoolURI.String())
	if err != nil {
		return nil, false, false, fmt.Errorf("error building ovftool arguments: %s", err)
	}

	ui.Sayf("Uploading %s to %s", source, p.config.Host)
	ui.Say("Validating username and password...")

	err = p.ValidateOvfTool(args, ovftool, ui)
	if err != nil {
		return nil, false, false, err
	}

	// Validation has passed, so run for real.
	ui.Say("Uploading virtual machine...")
	commandAndArgs := []string{ovftool}
	commandAndArgs = append(commandAndArgs, args...)
	comm := &shelllocal.Communicator{
		ExecuteCommand: commandAndArgs,
	}
	flattenedCmd := strings.Join(commandAndArgs, " ")
	backoff := &retry.Backoff{
		InitialBackoff: 200 * time.Millisecond,
		MaxBackoff:     30 * time.Second,
		Multiplier:     2,
	}
	err = retry.Config{
		Tries: p.config.MaxRetries,
		ShouldRetry: func(err error) bool {
			return err != nil
		},
		RetryDelay: backoff.Exponential,
	}.Run(ctx, func(ctx context.Context) error {
		cmd := &packersdk.RemoteCmd{Command: flattenedCmd}
		err = cmd.RunWithUi(ctx, comm, ui)
		if err != nil {
			return fmt.Errorf("error uploading virtual machine: %w", err)
		}
		if cmd.ExitStatus() != 0 {
			return fmt.Errorf("error uploading virtual machine: exit status %d", cmd.ExitStatus())
		}
		return nil
	})

	// Apply tags if configured
	if len(p.config.Tags) > 0 || len(p.config.Tag) > 0 {
		ui.Say("Applying tags to virtual machine...")
		if err := p.applyTags(ctx, ui); err != nil {
			return nil, false, false, fmt.Errorf("error applying tags: %s", err)
		}
	}

	artifact = NewArtifact(p.config.Datastore, p.config.VMFolder, p.config.VMName, artifact.Files())

	return artifact, false, false, nil
}

func (p *PostProcessor) applyTags(ctx context.Context, ui packersdk.Ui) error {
	// Connect to vSphere to apply tags
	d, err := driver.NewDriver(&driver.ConnectConfig{
		VCenterServer:      p.config.Host,
		Username:           p.config.Username,
		Password:           p.config.Password,
		InsecureConnection: p.config.Insecure,
		Datacenter:         p.config.Datacenter,
	})
	if err != nil {
		return fmt.Errorf("error connecting to vSphere: %s", err)
	}

	// Find the uploaded VM
	vm, err := d.FindVM(p.config.VMName)
	if err != nil {
		return fmt.Errorf("error finding virtual machine: %s", err)
	}

	// Get REST client for tag operations
	restClient := d.GetRestClient()

	// Create tag manager
	tagManager := vspherecommon.NewTagManager(restClient, &p.config.TagsConfig, p.config.ctx)

	// Validate configuration
	if err := tagManager.ValidateConfig(); err != nil {
		return fmt.Errorf("tag configuration validation failed: %s", err)
	}

	// Resolve tag IDs
	tagIDs, err := tagManager.ResolveTagIDs(ctx)
	if err != nil {
		return fmt.Errorf("error resolving tag IDs: %s", err)
	}

	// Apply tags to the VM
	vmRef := vm.Reference()
	if err := tagManager.ApplyTags(ctx, vmRef); err != nil {
		return fmt.Errorf("error applying tags: %s", err)
	}

	ui.Sayf("Applied %d tag(s) to virtual machine", len(tagIDs))
	return nil
}

func (p *PostProcessor) ValidateOvfTool(args []string, ovftool string, ui packersdk.Ui) error {
	args = append([]string{"--verifyOnly"}, args...)
	if p.config.Insecure {
		args = append(args, "--noSSLVerify")
		ui.Say("Skipping SSL thumbprint verification; insecure flag set to true...")
	}
	var out bytes.Buffer
	cmdCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(cmdCtx, ovftool, args...)
	cmd.Stdout = &out

	// Need to manually close stdin or else the ovftool call will hang if the
	// user has provided an invalid credential.
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	defer func() {
		if err := stdin.Close(); err != nil {
			log.Printf("[WARN] Failed to close stdin: %v", err)
		}
	}()

	if err := cmd.Run(); err != nil {
		outString := out.String()
		if strings.Contains(outString, "Enter login information for source") {
			err = fmt.Errorf("error running ovftool with --verifyOnly; the username " +
				"or password you provided may be incorrect")
			return err
		} else if strings.Contains(outString, "Accept SSL fingerprint") {
			err = fmt.Errorf("error running ovftool with --verifyOnly; the ssl thumbprint " +
				"returned by the server is not trusted. manually accept the thumbprint, " +
				"set the insecure flag to true, or pass the --noSSLVerify option")
			return err
		}
	}
	return nil
}

func (p *PostProcessor) BuildArgs(source, ovftoolURI string) ([]string, error) {
	args := []string{
		"--acceptAllEulas",
		fmt.Sprintf(`--name=%s`, p.config.VMName),
		fmt.Sprintf(`--datastore=%s`, p.config.Datastore),
	}

	if p.config.Insecure {
		args = append(args, fmt.Sprintf(`--noSSLVerify=%t`, p.config.Insecure))
	}

	if p.config.DiskMode != "" {
		args = append(args, fmt.Sprintf(`--diskMode=%s`, p.config.DiskMode))
	}

	if p.config.VMFolder != "" {
		args = append(args, fmt.Sprintf(`--vmFolder=%s`, p.config.VMFolder))
	}

	if p.config.HardwareVersion != "" {
		args = append(args, fmt.Sprintf(`--maxVirtualHardwareVersion=%s`, p.config.HardwareVersion))
	}

	if p.config.VMNetwork != "" {
		args = append(args, fmt.Sprintf(`--network=%s`, p.config.VMNetwork))
	}

	if p.config.Overwrite {
		args = append(args, "--overwrite")
	}

	if len(p.config.Options) > 0 {
		args = append(args, p.config.Options...)
	}

	args = append(args, source)
	args = append(args, ovftoolURI)

	return args, nil
}
