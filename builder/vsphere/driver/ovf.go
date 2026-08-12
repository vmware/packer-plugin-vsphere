// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package driver

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/url"
	"strings"

	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/vmware/govmomi/nfc"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/ovf"
	"github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/govmomi/vim25/types"
)

// OvfAuthConfig contains authentication credentials for remote OVF/OVA sources.
type OvfAuthConfig struct {
	Username string
	Password string
}

// OvfDeployConfig contains configuration for deploying virtual machines from
// OVF/OVA sources. Exactly one of URL or Path must be set.
type OvfDeployConfig struct {
	URL              string // HTTP(S) URL of a remote OVF/OVA file.
	Path             string // Local filesystem path to an OVF/OVA file.
	Authentication   *OvfAuthConfig
	Name             string
	Folder           string
	Cluster          string
	Host             string
	ResourcePool     string
	Datastore        string
	Network          string
	MacAddress       string
	Annotation       string
	VAppProperties   map[string]string
	DeploymentOption string // OVF deployment option such as "small", "medium", or "large".
	Locale           string // Locale for OVF deployment messages and descriptions (defaults to "US" if empty).
	SkipTlsVerify    bool   // Skip TLS certificate verification for HTTPS URLs (for testing environments only).
}

// ovfSourceLabel returns a sanitized label for the OVF source for errors and logs.
func (c *OvfDeployConfig) ovfSourceLabel() string {
	if c == nil {
		return ""
	}
	if c.Path != "" {
		return c.Path
	}
	return SanitizeOvfURL(c.URL)
}

// DeployOvf deploys a virtual machine from an OVF/OVA source.
//
// The plugin reads the OVF descriptor and archive files on the Packer host
// (from an HTTP(S) URL or a local filesystem path), passes the descriptor XML
// to vSphere's OVF Manager, and uploads disk files through an NFC lease.
// vSphere does not fetch the source directly.
func (d *VCenterDriver) DeployOvf(ctx context.Context, config *OvfDeployConfig, ui packersdk.Ui) (VirtualMachine, error) {
	label := config.ovfSourceLabel()
	if err := d.validateOvfDeploymentConfig(config); err != nil {
		return nil, d.wrapOvfError("configuration validation failed", err, label)
	}

	ovfWrapper, err := d.createOvfManagerWrapper(config.Authentication, config.SkipTlsVerify)
	if err != nil {
		return nil, d.wrapOvfError("failed to initialize OVF manager", err, label)
	}

	// Validate OVF accessibility before proceeding with vSphere resource lookup.
	parseResult, err := d.validateOvfAccessibility(ctx, config, ovfWrapper)
	if err != nil {
		return nil, d.wrapOvfError("OVF/OVA source validation failed", err, label)
	}

	folder, err := d.FindFolder(config.Folder)
	if err != nil {
		return nil, d.wrapOvfError("failed to find target folder", err, label)
	}

	resourcePool, err := d.FindResourcePool(config.Cluster, config.Host, config.ResourcePool)
	if err != nil {
		return nil, d.wrapOvfError("failed to find resource pool", err, label)
	}

	datastore, err := d.FindDatastore(config.Datastore, config.Host)
	if err != nil {
		return nil, d.wrapOvfError("failed to find datastore", err, label)
	}

	importParams, err := d.createOvfImportParams(config, parseResult)
	if err != nil {
		return nil, d.wrapOvfError("failed to create import parameters", err, label)
	}

	log.Printf("[INFO] Creating OVF import specification from OVF/OVA source...")

	importSpecResult, err := ovfWrapper.CreateImportSpec(ctx, config, resourcePool.pool, datastore.Reference(), importParams)
	if err != nil {
		return nil, d.wrapOvfError("failed to create import specification", err, label)
	}

	// Handle OVF validation errors with detailed messages.
	if len(importSpecResult.Error) > 0 {
		return nil, d.handleOvfValidationErrors(importSpecResult.Error, label)
	}

	// Handle OVF warnings, if present.
	if len(importSpecResult.Warning) > 0 {
		d.reportOvfWarnings(importSpecResult.Warning, ui)
	}

	log.Printf("[INFO] Starting OVF import operation...")

	// Start the vApp import: vSphere returns an NFC lease through which disk files are uploaded.
	lease, err := resourcePool.pool.ImportVApp(ctx, importSpecResult.ImportSpec, folder.folder, nil)
	if err != nil {
		return nil, d.wrapOvfError("failed to start vApp import", err, label)
	}

	// Upload disk files from the OVF/OVA source and complete the import.
	info, err := d.uploadAndCompleteOvfImport(ctx, lease, config, importSpecResult.FileItem)
	if err != nil {
		return nil, d.wrapOvfError("OVF import operation failed", err, label)
	}

	// Validate that we received a valid virtual machine reference.
	if info == nil || info.Entity.Type != "VirtualMachine" {
		return nil, fmt.Errorf("OVF deployment completed but did not return a valid virtual machine reference")
	}

	// Get the imported virtual machine reference from the lease info.
	vmRef := info.Entity
	vm := d.NewVM(&vmRef)
	if err := d.applyOvfPostImportConfig(vm, config); err != nil {
		return nil, d.wrapOvfError("failed to apply post-import virtual machine configuration", err, label)
	}
	return vm, nil
}

// uploadAndCompleteOvfImport waits for the NFC lease to be ready, uploads all
// disk files from the OVF/OVA source through the Packer host, and signals
// vSphere to complete the import.
func (d *VCenterDriver) uploadAndCompleteOvfImport(ctx context.Context, lease *nfc.Lease, config *OvfDeployConfig, fileItems []types.OvfFileItem) (*nfc.LeaseInfo, error) {
	// Wait for the lease to initialize and return the list of upload targets.
	info, err := lease.Wait(ctx, fileItems)
	if err != nil {
		_ = lease.Abort(ctx, nil)
		return nil, err
	}

	// StartUpdater sends periodic keepalive progress ticks to prevent lease timeout.
	updater := lease.StartUpdater(ctx, info)
	defer updater.Done()

	archive, err := newOvfArchive(config)
	if err != nil {
		_ = lease.Abort(ctx, nil)
		return nil, fmt.Errorf("failed to initialise OVF archive for upload: %s", err)
	}

	for _, item := range info.Items {
		if err := d.uploadOvfFile(ctx, lease, archive, item); err != nil {
			_ = lease.Abort(ctx, &types.LocalizedMethodFault{
				Fault: &types.FileFault{File: item.Path},
			})
			return nil, d.categorizeOvfImportError(fmt.Errorf("upload of '%s' failed: %s", item.Path, err))
		}
		log.Printf("[INFO] Uploaded %s", item.Path)
	}

	if err := lease.Complete(ctx); err != nil {
		return nil, fmt.Errorf("failed to complete OVF import lease: %s", err)
	}

	return info, nil
}

// uploadOvfFile opens the named file from the OVF/OVA archive and uploads it to
// vSphere via the NFC lease.
func (d *VCenterDriver) uploadOvfFile(ctx context.Context, lease *nfc.Lease, archive ovfArchive, item nfc.FileItem) error {
	rc, size, err := archive.Open(item.Path)
	if err != nil {
		return fmt.Errorf("failed to open '%s' from OVF/OVA source: %s", item.Path, err)
	}
	defer rc.Close()

	opts := soap.Upload{
		ContentLength: size,
	}
	return lease.Upload(ctx, item, rc, opts)
}

// applyOvfPostImportConfig applies settings that are not part of the OVF import
// spec.
func (d *VCenterDriver) applyOvfPostImportConfig(vm VirtualMachine, config *OvfDeployConfig) error {
	if config.Annotation == "" && config.MacAddress == "" {
		return nil
	}

	var configSpec types.VirtualMachineConfigSpec

	if config.Annotation != "" {
		configSpec.Annotation = config.Annotation
	}

	if config.MacAddress != "" {
		devices, err := vm.Devices()
		if err != nil {
			return fmt.Errorf("error finding virtual machine devices: %s", err)
		}

		adapter, err := findNetworkAdapter(devices)
		if err != nil {
			return fmt.Errorf("error finding network adapter: %s", err)
		}

		current := adapter.GetVirtualEthernetCard()
		current.AddressType = string(types.VirtualEthernetCardMacTypeManual)
		current.MacAddress = config.MacAddress

		configSpec.DeviceChange = append(configSpec.DeviceChange, &types.VirtualDeviceConfigSpec{
			Device:    adapter.(types.BaseVirtualDevice),
			Operation: types.VirtualDeviceConfigSpecOperationEdit,
		})
	}

	return vm.Reconfigure(configSpec)
}

// GetOvfOptions retrieves OVF deployment options from an OVF/OVA source.
// The descriptor is read on the Packer host before being parsed via vSphere's
// OVF Manager.
func (d *VCenterDriver) GetOvfOptions(ctx context.Context, config *OvfDeployConfig) ([]types.OvfOptionInfo, error) {
	if config == nil {
		return nil, fmt.Errorf("OVF deployment configuration cannot be nil")
	}
	label := config.ovfSourceLabel()
	if err := d.validateOvfSource(config); err != nil {
		return nil, d.wrapOvfError("source validation failed", err, label)
	}

	ovfWrapper, err := d.createOvfManagerWrapper(config.Authentication, config.SkipTlsVerify)
	if err != nil {
		return nil, d.wrapOvfError("failed to initialize OVF manager", err, label)
	}

	locale := config.Locale
	if locale == "" {
		locale = "US"
	}

	parseParams := d.createOvfParseParams(locale)

	parseResult, err := ovfWrapper.ParseDescriptor(ctx, config, parseParams)
	if err != nil {
		return nil, d.wrapOvfError("failed to parse OVF descriptor", err, label)
	}

	// Handle parse errors, if present.
	if len(parseResult.Error) > 0 {
		return nil, d.handleOvfValidationErrors(parseResult.Error, label)
	}

	var optionInfos []types.OvfOptionInfo
	for _, deployOption := range parseResult.DeploymentOption {
		optionInfos = append(optionInfos, types.OvfOptionInfo{
			Option: deployOption.Key,
			Description: types.LocalizableMessage{
				Message: deployOption.Description,
			},
		})
	}

	return optionInfos, nil
}

// OvfManagerWrapper wraps the govmomi OVF Manager with authentication and TLS
// support.
type OvfManagerWrapper struct {
	manager               *ovf.Manager
	auth                  *OvfAuthConfig
	insecureSkipTLSVerify bool
}

// createOvfManagerWrapper creates a new OVF Manager wrapper with authentication
// and TLS support.
func (d *VCenterDriver) createOvfManagerWrapper(auth *OvfAuthConfig, insecureSkipTLSVerify bool) (*OvfManagerWrapper, error) {
	ovfManager := ovf.NewManager(d.VimClient)

	if auth != nil {
		if err := d.validateOvfAuthentication(auth); err != nil {
			return nil, fmt.Errorf("invalid authentication configuration: %s", err)
		}
	}

	return &OvfManagerWrapper{
		manager:               ovfManager,
		auth:                  auth,
		insecureSkipTLSVerify: insecureSkipTLSVerify,
	}, nil
}

// validateOvfAuthentication validates the OVF authentication configuration.
func (d *VCenterDriver) validateOvfAuthentication(auth *OvfAuthConfig) error {
	if auth == nil {
		return nil
	}

	if auth.Username != "" && auth.Password == "" {
		return fmt.Errorf("password must be provided when username is specified")
	}
	if auth.Username == "" && auth.Password != "" {
		return fmt.Errorf("username must be provided when password is specified")
	}

	return nil
}

// validateOvfURL validates that the URL uses supported HTTP/HTTPS protocols.
func (d *VCenterDriver) validateOvfURL(urlStr string) error {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return fmt.Errorf("invalid URL format: %s", err)
	}

	switch parsedURL.Scheme {
	case "http", "https":
		if parsedURL.Host == "" {
			return fmt.Errorf("URL must include a valid host")
		}
		if parsedURL.Path == "" {
			return fmt.Errorf("URL must include a path to the OVF/OVA file")
		}
		return nil
	default:
		return fmt.Errorf("unsupported protocol '%s', only HTTP and HTTPS are supported", parsedURL.Scheme)
	}
}

// isOvfFileExtension checks whether pathOrURL ends with .ovf or .ova.
func (d *VCenterDriver) isOvfFileExtension(pathOrURL string) bool {
	if pathOrURL == "" {
		return false
	}
	lower := strings.ToLower(pathOrURL)
	return strings.HasSuffix(lower, ".ovf") || strings.HasSuffix(lower, ".ova")
}

// isOvfFileURL checks if the URL points to an OVF or OVA file based on file
// extension.
func (d *VCenterDriver) isOvfFileURL(urlStr string) bool {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return false
	}
	return d.isOvfFileExtension(parsedURL.Path)
}

// validateOvfSource validates that exactly one of URL or Path is set and that
// source-specific options are consistent.
func (d *VCenterDriver) validateOvfSource(config *OvfDeployConfig) error {
	if config == nil {
		return fmt.Errorf("OVF deployment configuration cannot be nil")
	}

	hasURL := config.URL != ""
	hasPath := config.Path != ""
	if hasURL && hasPath {
		return fmt.Errorf("OVF source cannot specify both URL and Path")
	}
	if !hasURL && !hasPath {
		return fmt.Errorf("OVF source requires either URL or Path")
	}

	if hasPath {
		if !d.isOvfFileExtension(config.Path) {
			return fmt.Errorf("local OVF/OVA path must point to an OVF (.ovf) or OVA (.ova) file")
		}
		if config.Authentication != nil && (config.Authentication.Username != "" || config.Authentication.Password != "") {
			return fmt.Errorf("authentication is only applicable when using a remote OVF/OVA URL")
		}
		if config.SkipTlsVerify {
			return fmt.Errorf("skip_tls_verify is only applicable when using a remote OVF/OVA URL")
		}
		return nil
	}

	if err := d.validateOvfURL(config.URL); err != nil {
		return err
	}
	if !d.isOvfFileURL(config.URL) {
		return fmt.Errorf("URL must point to an OVF (.ovf) or OVA (.ova) file")
	}
	if err := d.validateOvfAuthentication(config.Authentication); err != nil {
		return err
	}
	if config.SkipTlsVerify {
		parsedURL, _ := url.Parse(config.URL)
		if parsedURL != nil && parsedURL.Scheme == "http" {
			return fmt.Errorf("skip_tls_verify is only applicable for HTTPS URLs, but URL uses HTTP protocol")
		}
	}
	return nil
}

// validateOvfDeploymentConfig validates the complete OVF deployment
// configuration.
func (d *VCenterDriver) validateOvfDeploymentConfig(config *OvfDeployConfig) error {
	if config == nil {
		return fmt.Errorf("OVF deployment configuration cannot be nil")
	}

	if config.Name == "" {
		return fmt.Errorf("virtual machine name is required")
	}

	return d.validateOvfSource(config)
}

// buildOvfNetworkMappings maps OVF descriptor network names to a vSphere network.
// When the OVF defines multiple networks, each name is mapped to the same configured network.
func (d *VCenterDriver) buildOvfNetworkMappings(ovfNetworks []types.OvfNetworkInfo, vsphereNetworkName string) ([]types.OvfNetworkMapping, error) {
	if len(ovfNetworks) == 0 {
		return nil, nil
	}

	if vsphereNetworkName == "" {
		names := make([]string, 0, len(ovfNetworks))
		for _, ovfNet := range ovfNetworks {
			names = append(names, ovfNet.Name)
		}
		return nil, fmt.Errorf("OVF requires network mapping for %s; specify the network configuration option", strings.Join(names, ", "))
	}

	network, err := d.FindNetwork(vsphereNetworkName)
	if err != nil {
		return nil, fmt.Errorf("error finding network: %s", err)
	}

	netRef := network.network.Reference()
	mappings := make([]types.OvfNetworkMapping, 0, len(ovfNetworks))
	for _, ovfNet := range ovfNetworks {
		mappings = append(mappings, types.OvfNetworkMapping{
			Name:    ovfNet.Name,
			Network: netRef,
		})
	}

	return mappings, nil
}

// createOvfImportParams creates import parameters with authentication and
// configuration support.
func (d *VCenterDriver) createOvfImportParams(config *OvfDeployConfig, parseResult *types.OvfParseDescriptorResult) (*types.OvfCreateImportSpecParams, error) {
	locale := config.Locale
	if locale == "" {
		locale = "US"
	}

	importParams := &types.OvfCreateImportSpecParams{
		EntityName: config.Name,
		OvfManagerCommonParams: types.OvfManagerCommonParams{
			DeploymentOption: config.DeploymentOption,
			Locale:           locale,
		},
	}

	var ovfNetworks []types.OvfNetworkInfo
	if parseResult != nil {
		ovfNetworks = parseResult.Network
	}

	networkMappings, err := d.buildOvfNetworkMappings(ovfNetworks, config.Network)
	if err != nil {
		return nil, err
	}
	if len(networkMappings) > 0 {
		importParams.NetworkMapping = networkMappings
	}

	if len(config.VAppProperties) > 0 {
		var propertyMappings []types.KeyValue
		for key, value := range config.VAppProperties {
			propertyMappings = append(propertyMappings, types.KeyValue{
				Key:   key,
				Value: value,
			})
		}
		importParams.PropertyMapping = propertyMappings
	}

	if config.Host != "" {
		host, err := d.FindHost(config.Host)
		if err != nil {
			return nil, fmt.Errorf("error finding host: %s", err)
		}
		hostRef := host.host.Reference()
		importParams.HostSystem = &hostRef
	}

	return importParams, nil
}

// createOvfParseParams creates parse parameters with locale support.
func (d *VCenterDriver) createOvfParseParams(locale string) types.OvfParseDescriptorParams {
	return types.OvfParseDescriptorParams{
		OvfManagerCommonParams: types.OvfManagerCommonParams{
			Locale: locale,
		},
	}
}

// fetchOvfXML reads and returns the OVF descriptor XML bytes on the Packer
// host. For OVA archives the descriptor is extracted from the TAR; for OVF
// sources it is read directly.
func (w *OvfManagerWrapper) fetchOvfXML(config *OvfDeployConfig) ([]byte, error) {
	archive, err := newOvfArchive(config)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize OVF archive: %s", err)
	}

	// OVA: glob inside the TAR for the first .ovf entry.
	// OVF: open the descriptor directly (empty name).
	name := ""
	if isOvfSourceOVA(config) {
		name = "*.ovf"
	}

	rc, _, err := archive.Open(name)
	if err != nil {
		return nil, fmt.Errorf("failed to read OVF descriptor from '%s': %s", config.ovfSourceLabel(), err)
	}
	defer rc.Close()

	return io.ReadAll(rc)
}

// CreateImportSpec reads the OVF descriptor on the Packer host and creates an
// import spec from the XML via vSphere's OVF Manager.
func (w *OvfManagerWrapper) CreateImportSpec(ctx context.Context, config *OvfDeployConfig, rp *object.ResourcePool, ds types.ManagedObjectReference, params *types.OvfCreateImportSpecParams) (*types.OvfCreateImportSpecResult, error) {
	xmlBytes, err := w.fetchOvfXML(config)
	if err != nil {
		return nil, err
	}

	result, err := w.manager.CreateImportSpec(ctx, string(xmlBytes), rp, ds, params)
	if err != nil {
		return nil, w.categorizeOvfManagerError(err, config.ovfSourceLabel())
	}

	return result, nil
}

// ParseDescriptor reads the OVF descriptor on the Packer host and parses the
// XML via vSphere's OVF Manager.
func (w *OvfManagerWrapper) ParseDescriptor(ctx context.Context, config *OvfDeployConfig, params types.OvfParseDescriptorParams) (*types.OvfParseDescriptorResult, error) {
	xmlBytes, err := w.fetchOvfXML(config)
	if err != nil {
		return nil, err
	}

	result, err := w.manager.ParseDescriptor(ctx, string(xmlBytes), params)
	if err != nil {
		return nil, w.categorizeOvfManagerError(err, config.ovfSourceLabel())
	}

	return result, nil
}

// categorizeOvfManagerError provides specific error categorization for OVF
// Manager operations.
func (w *OvfManagerWrapper) categorizeOvfManagerError(err error, url string) error {
	errStr := strings.ToLower(err.Error())

	errorMappings := map[string]string{
		"401":          "authentication failed - please verify username and password are correct",
		"unauthorized": "authentication failed - please verify username and password are correct",
		"404":          "OVF/OVA file not found - please verify the URL is correct",
		"not found":    "OVF/OVA file not found - please verify the URL is correct",
		"timeout":      "network connectivity error - please check network access and firewall settings",
		"connection":   "network connectivity error - please check network access and firewall settings",
		"parse":        "OVF/OVA file format error - the file may be corrupted or in an unsupported format",
		"xml":          "OVF/OVA file format error - the file may be corrupted or in an unsupported format",
		"invalid":      "OVF/OVA file format error - the file may be corrupted or in an unsupported format",
	}

	// Handle TLS certificate errors with context-aware messaging.
	if strings.Contains(errStr, "certificate") || strings.Contains(errStr, "tls") || strings.Contains(errStr, "x509") {
		if w.insecureSkipTLSVerify {
			return fmt.Errorf("TLS certificate error occurred despite skip_tls_verify being enabled; this may indicate a vSphere configuration issue")
		}
		return fmt.Errorf("TLS certificate error - for testing environments, consider using 'skip_tls_verify = true'; for production, ensure valid certificates are configured")
	}

	for pattern, message := range errorMappings {
		if strings.Contains(errStr, pattern) {
			return fmt.Errorf("%s", message)
		}
	}

	return fmt.Errorf("OVF Manager operation failed: %s", err)
}

// categorizeOvfImportError categorizes OVF import errors and provides
// actionable error messages.
func (d *VCenterDriver) categorizeOvfImportError(err error) error {
	errStr := strings.ToLower(err.Error())
	sanitizedErr := SanitizeOvfErrorMessage(err.Error())

	errorChecks := []struct {
		patterns []string
		message  string
	}{
		{
			patterns: []string{"401", "unauthorized", "authentication failed", "invalid credentials"},
			message:  "authentication failed when accessing remote OVF/OVA source. Please verify your username and password are correct",
		},
		{
			patterns: []string{"404", "not found", "no such file", "file does not exist"},
			message:  "remote OVF/OVA file not found. Please verify the URL is correct and the file exists",
		},
		{
			patterns: []string{"timeout", "connection refused", "connection reset", "network unreachable", "dial", "no route to host", "connection timed out"},
			message:  "network connectivity error accessing remote OVF/OVA source. Please check network connectivity and firewall settings",
		},
		{
			patterns: []string{"no such host", "dns", "name resolution", "hostname"},
			message:  "DNS resolution failed for remote OVF/OVA source. Please verify the hostname is correct and DNS is configured properly",
		},
		{
			patterns: []string{"certificate", "tls", "ssl", "x509", "handshake"},
			message:  "TLS/SSL certificate error accessing remote OVF/OVA source. For testing environments, consider using 'skip_tls_verify = true'. For production, ensure valid certificates are configured",
		},
		{
			patterns: []string{"invalid ovf", "corrupt", "malformed", "parse", "xml", "ovf descriptor", "invalid format", "checksum"},
			message:  "OVF/OVA file validation error. The file may be corrupted, incomplete, or in an invalid format. Please verify file integrity and try again",
		},
		{
			patterns: []string{"insufficient", "not enough", "out of space", "disk space", "memory", "cpu", "resource"},
			message:  "insufficient vSphere resources for OVF deployment. Please check available storage, memory, and CPU resources",
		},
		{
			patterns: []string{"permission", "access denied", "forbidden", "403"},
			message:  "insufficient permissions for OVF deployment. Please verify vSphere user has required privileges",
		},
		{
			patterns: []string{"cancel", "abort", "interrupt", "stopped"},
			message:  "OVF deployment was cancelled or interrupted",
		},
		{
			patterns: []string{"vim.fault", "vsphere", "vcenter", "esx"},
			message:  "vSphere error during OVF deployment. Please check vSphere logs for additional details",
		},
	}

	for _, check := range errorChecks {
		if d.containsAny(errStr, check.patterns) {
			return fmt.Errorf("%s. Error: %s", check.message, sanitizedErr)
		}
	}

	// HTTP server errors.
	if strings.Contains(errStr, "http") && d.containsAny(errStr, []string{"500", "502", "503", "504"}) {
		return fmt.Errorf("HTTP server error accessing remote OVF/OVA source. The remote server may be temporarily unavailable. Error: %s", sanitizedErr)
	}

	return fmt.Errorf("OVF deployment failed: %s", sanitizedErr)
}

// containsAny checks if the string contains any of the given patterns.
func (d *VCenterDriver) containsAny(s string, patterns []string) bool {
	for _, pattern := range patterns {
		if strings.Contains(s, pattern) {
			return true
		}
	}
	return false
}

// wrapOvfError wraps errors with context and sanitizes sensitive information.
func (d *VCenterDriver) wrapOvfError(errContext string, err error, source string) error {
	sanitizedSource := SanitizeOvfErrorMessage(source)
	if strings.Contains(source, "://") {
		sanitizedSource = SanitizeOvfURL(source)
	}
	sanitizedErr := SanitizeOvfErrorMessage(err.Error())
	return fmt.Errorf("%s for OVF/OVA source '%s': %s", errContext, sanitizedSource, sanitizedErr)
}

// validateOvfAccessibility reads and parses the OVF descriptor on the Packer
// host before deployment proceeds.
func (d *VCenterDriver) validateOvfAccessibility(ctx context.Context, config *OvfDeployConfig, wrapper *OvfManagerWrapper) (*types.OvfParseDescriptorResult, error) {
	locale := config.Locale
	if locale == "" {
		locale = "US"
	}

	parseParams := d.createOvfParseParams(locale)
	parseResult, err := wrapper.ParseDescriptor(ctx, config, parseParams)
	if err != nil {
		return nil, fmt.Errorf("failed to access or parse OVF descriptor: %s", err)
	}

	if len(parseResult.Error) > 0 {
		return nil, d.handleOvfValidationErrors(parseResult.Error, config.ovfSourceLabel())
	}

	return parseResult, nil
}

// handleOvfValidationErrors processes OVF validation errors and provides
// detailed error messages.
func (d *VCenterDriver) handleOvfValidationErrors(errors []types.LocalizedMethodFault, source string) error {
	sanitizedSource := source
	if strings.Contains(source, "://") {
		sanitizedSource = SanitizeOvfURL(source)
	}

	if len(errors) == 1 {
		return fmt.Errorf("OVF validation failed for '%s': %s", sanitizedSource, errors[0].LocalizedMessage)
	}

	const maxErrors = 5
	errorMessages := make([]string, 0, min(len(errors), maxErrors)+1)

	for i, err := range errors {
		if i >= maxErrors {
			errorMessages = append(errorMessages, fmt.Sprintf("... and %d more errors", len(errors)-i))
			break
		}
		errorMessages = append(errorMessages, fmt.Sprintf("  - %s", err.LocalizedMessage))
	}

	return fmt.Errorf("OVF validation failed for '%s' with %d errors:\n%s",
		sanitizedSource, len(errors), strings.Join(errorMessages, "\n"))
}

// reportOvfWarnings reports OVF warnings to the user interface.
func (d *VCenterDriver) reportOvfWarnings(warnings []types.LocalizedMethodFault, ui packersdk.Ui) {
	if len(warnings) == 0 {
		return
	}

	const maxWarnings = 3
	ui.Sayf("OVF deployment has %d warning(s):", len(warnings))

	for i, warning := range warnings {
		if i >= maxWarnings {
			ui.Sayf("  ... and %d more warnings", len(warnings)-i)
			break
		}
		ui.Sayf("  - %s", warning.LocalizedMessage)
	}
}
