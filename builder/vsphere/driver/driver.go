// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package driver

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/find"
	"github.com/vmware/govmomi/nfc"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/govmomi/ovf"
	"github.com/vmware/govmomi/session"
	"github.com/vmware/govmomi/vapi/library"
	"github.com/vmware/govmomi/vapi/rest"
	"github.com/vmware/govmomi/vim25"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/soap"
	"github.com/vmware/govmomi/vim25/types"
)

// Driver defines the interface for vSphere operations including VM management and OVF deployment.
type Driver interface {
	NewVM(ref *types.ManagedObjectReference) VirtualMachine
	FindVM(name string) (VirtualMachine, error)
	FindCluster(name string) (*Cluster, error)
	PreCleanVM(ui packersdk.Ui, vmPath string, force bool, vsphereCluster string, vsphereHost string, vsphereResourcePool string) error
	CreateVM(config *CreateConfig) (VirtualMachine, error)

	NewDatastore(ref *types.ManagedObjectReference) Datastore
	FindDatastore(name string, host string) (Datastore, error)
	GetDatastoreName(id string) (string, error)
	GetDatastoreFilePath(datastoreID, dir, filename string) (string, error)

	NewFolder(ref *types.ManagedObjectReference) *Folder
	FindFolder(name string) (*Folder, error)
	NewHost(ref *types.ManagedObjectReference) *Host
	FindHost(name string) (*Host, error)
	NewNetwork(ref *types.ManagedObjectReference) *Network
	FindNetwork(name string) (*Network, error)
	FindNetworks(name string) ([]*Network, error)
	NewResourcePool(ref *types.ManagedObjectReference) *ResourcePool
	FindResourcePool(cluster string, host string, name string) (*ResourcePool, error)

	FindContentLibraryByName(name string) (*Library, error)
	FindContentLibraryItem(libraryId string, name string) (*library.Item, error)
	FindContentLibraryFileDatastorePath(isoPath string) (string, error)
	UpdateContentLibraryItem(item *library.Item, name string, description string) error
	GetRestClient() *rest.Client
	DeleteContentLibraryItem(itemID string) error
	DeployOvf(ctx context.Context, config *OvfDeployConfig, ui packersdk.Ui) (VirtualMachine, error)
	GetOvfOptions(ctx context.Context, url string, auth *OvfAuthConfig, locale string, skipTlsVerify bool) ([]types.OvfOptionInfo, error)
	Cleanup() (error, error)
}

// VCenterDriver implements the Driver interface for vCenter Server operations.
type VCenterDriver struct {
	Ctx        context.Context
	Client     *govmomi.Client
	VimClient  *vim25.Client
	RestClient *RestClient
	Finder     *find.Finder
	Datacenter *object.Datacenter
}

func NewVCenterDriver(ctx context.Context, client *govmomi.Client, vimClient *vim25.Client, user *url.Userinfo, finder *find.Finder, datacenter *object.Datacenter) *VCenterDriver {
	return &VCenterDriver{
		Ctx:       ctx,
		Client:    client,
		VimClient: vimClient,
		RestClient: &RestClient{
			client:      rest.NewClient(vimClient),
			credentials: user,
		},
		Datacenter: datacenter,
		Finder:     finder,
	}
}

// ConnectConfig contains the configuration for connecting to vCenter.
type ConnectConfig struct {
	VCenterServer      string
	Username           string
	Password           string
	InsecureConnection bool
	Datacenter         string
}

// OvfAuthConfig contains authentication credentials for remote OVF/OVA sources.
type OvfAuthConfig struct {
	Username string
	Password string
}

// OvfDeployConfig contains configuration for deploying VMs from remote OVF/OVA sources.
type OvfDeployConfig struct {
	URL              string
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

const ovfProgressStallThreshold = 30 * time.Second

// OvfProgressMonitor provides progress monitoring capabilities for OVF deployment operations.
type OvfProgressMonitor struct {
	ui                packersdk.Ui
	ctx               context.Context
	cancelFunc        context.CancelFunc
	progressInterval  time.Duration
	startedAt         time.Time
	lastProgressTime  time.Time
	lastProgressValue int32
	lastLeaseState    types.HttpNfcLeaseState
	loggedStallNotice bool
}

// NewOvfProgressMonitor creates a new progress monitor for OVF deployment operations.
func NewOvfProgressMonitor(ui packersdk.Ui, ctx context.Context) *OvfProgressMonitor {
	ctx, cancel := context.WithCancel(ctx)
	return &OvfProgressMonitor{
		ui:               ui,
		ctx:              ctx,
		cancelFunc:       cancel,
		progressInterval: 5 * time.Second,
		startedAt:        time.Now(),
	}
}

// maybeLogProgressStall logs once when transfer progress has not started after a delay.
func (m *OvfProgressMonitor) maybeLogProgressStall(progress int32, now time.Time) {
	if progress > 0 || m.loggedStallNotice {
		return
	}

	if now.Sub(m.startedAt) >= ovfProgressStallThreshold {
		log.Printf("[INFO] OVF deployment is still waiting for transfer progress from the remote source...")
		m.loggedStallNotice = true
	}
}

// Cancel stops progress monitoring.
func (m *OvfProgressMonitor) Cancel() {
	if m.cancelFunc != nil {
		m.cancelFunc()
	}
}

// reportLeaseProgress reports NFC lease state and transfer progress to the user interface.
func (m *OvfProgressMonitor) reportLeaseProgress(leaseProps mo.HttpNfcLease) {
	now := time.Now()

	progress := leaseProps.TransferProgress
	if progress == 0 {
		progress = leaseProps.InitializeProgress
	}

	m.maybeLogProgressStall(progress, now)

	if now.Sub(m.lastProgressTime) < m.progressInterval {
		return
	}

	switch leaseProps.State {
	case types.HttpNfcLeaseStateError:
		if leaseProps.Error != nil {
			m.ui.Errorf("OVF deployment lease error: %s", leaseProps.Error.LocalizedMessage)
		}
		m.lastProgressTime = now
		return

	case types.HttpNfcLeaseStateDone:
		log.Printf("[INFO] OVF deployment transfer completed")
		m.lastProgressTime = now
		return
	}

	if progress > 0 && progress != m.lastProgressValue {
		switch leaseProps.State {
		case types.HttpNfcLeaseStateInitializing:
			m.ui.Sayf("OVF deployment initializing... %d%%", progress)
		case types.HttpNfcLeaseStateReady:
			m.ui.Sayf("OVF deployment in progress... %d%%", progress)
		}
		m.lastProgressValue = progress
		m.lastProgressTime = now
		return
	}

	if progress == 0 && m.lastLeaseState != leaseProps.State {
		m.lastLeaseState = leaseProps.State
		switch leaseProps.State {
		case types.HttpNfcLeaseStateInitializing:
			log.Printf("[INFO] OVF deployment initializing...")
		case types.HttpNfcLeaseStateReady:
			log.Printf("[INFO] OVF deployment in progress...")
		}
	}

	m.lastProgressTime = now
}

// monitorLeaseProgress monitors NFC lease state and transfer progress until import completes.
func (d *VCenterDriver) monitorLeaseProgress(ctx context.Context, lease *nfc.Lease, progressMonitor *OvfProgressMonitor) (*nfc.LeaseInfo, error) {
	resultChan := make(chan *nfc.LeaseInfo, 1)
	errorChan := make(chan error, 1)

	go func() {
		defer close(resultChan)
		defer close(errorChan)

		info, err := lease.Wait(ctx, []types.OvfFileItem{})
		if err != nil {
			errorChan <- err
			return
		}
		resultChan <- info
	}()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case info := <-resultChan:
			return info, nil

		case err := <-errorChan:
			return nil, err

		case <-ticker.C:
			if leaseProps, err := lease.Properties(ctx, "state", "initializeProgress", "transferProgress", "error"); err == nil {
				progressMonitor.reportLeaseProgress(leaseProps)
			}

		case <-ctx.Done():
			log.Printf("[INFO] OVF deployment context cancelled")
			return nil, fmt.Errorf("OVF deployment context cancelled")
		}
	}
}

func NewDriver(config *ConnectConfig) (Driver, error) {
	ctx := context.TODO()

	vcenterUrl, err := url.Parse(fmt.Sprintf("https://%v/sdk", config.VCenterServer))
	if err != nil {
		return nil, err
	}
	credentials := url.UserPassword(config.Username, config.Password)
	vcenterUrl.User = credentials

	soapClient := soap.NewClient(vcenterUrl, config.InsecureConnection)
	vimClient, err := vim25.NewClient(ctx, soapClient)
	if err != nil {
		return nil, err
	}

	vimClient.RoundTripper = session.KeepAlive(vimClient.RoundTripper, 10*time.Minute)
	client := &govmomi.Client{
		Client:         vimClient,
		SessionManager: session.NewManager(vimClient),
	}

	err = client.SessionManager.Login(ctx, credentials)
	if err != nil {
		return nil, err
	}

	restClient := &RestClient{
		client:      rest.NewClient(vimClient),
		credentials: credentials,
	}

	if err := restClient.Login(ctx); err != nil {
		return nil, fmt.Errorf("failed to login to REST API: %w", err)
	}

	finder := find.NewFinder(client.Client, false)
	datacenter, err := finder.DatacenterOrDefault(ctx, config.Datacenter)
	if err != nil {
		return nil, err
	}
	finder.SetDatacenter(datacenter)

	d := &VCenterDriver{
		Ctx:        ctx,
		Client:     client,
		VimClient:  vimClient,
		RestClient: restClient,
		Datacenter: datacenter,
		Finder:     finder,
	}
	return d, nil
}

func (d *VCenterDriver) GetRestClient() *rest.Client {
	return d.RestClient.client
}

// DeployOvf deploys a virtual machine from a remote OVF/OVA source using vSphere's native pull method.
func (d *VCenterDriver) DeployOvf(ctx context.Context, config *OvfDeployConfig, ui packersdk.Ui) (VirtualMachine, error) {
	if err := d.validateOvfDeploymentConfig(config); err != nil {
		return nil, d.wrapOvfError("configuration validation failed", err, config.URL)
	}

	ovfWrapper, err := d.createOvfManagerWrapper(config.Authentication, config.SkipTlsVerify)
	if err != nil {
		return nil, d.wrapOvfError("failed to initialize OVF manager", err, config.URL)
	}

	// Validate remote OVF accessibility before proceeding with vSphere resource lookup.
	parseResult, err := d.validateRemoteOvfAccessibility(ctx, config, ovfWrapper)
	if err != nil {
		return nil, d.wrapOvfError("remote OVF/OVA source validation failed", err, config.URL)
	}

	folder, err := d.FindFolder(config.Folder)
	if err != nil {
		return nil, d.wrapOvfError("failed to find target folder", err, config.URL)
	}

	resourcePool, err := d.FindResourcePool(config.Cluster, config.Host, config.ResourcePool)
	if err != nil {
		return nil, d.wrapOvfError("failed to find resource pool", err, config.URL)
	}

	datastore, err := d.FindDatastore(config.Datastore, config.Host)
	if err != nil {
		return nil, d.wrapOvfError("failed to find datastore", err, config.URL)
	}

	importParams, err := d.createOvfImportParams(config, parseResult)
	if err != nil {
		return nil, d.wrapOvfError("failed to create import parameters", err, config.URL)
	}

	log.Printf("[INFO] Creating OVF import specification from remote source...")

	// Use vSphere's native URL-based OVF deployment with authentication.
	importSpecResult, err := ovfWrapper.CreateImportSpecFromURL(ctx, config.URL, resourcePool.pool, datastore.Reference(), importParams)
	if err != nil {
		return nil, d.wrapOvfError("failed to create import specification", err, config.URL)
	}

	// Handle OVF validation errors with detailed messages.
	if len(importSpecResult.Error) > 0 {
		return nil, d.handleOvfValidationErrors(importSpecResult.Error, config.URL)
	}

	// Handle OVF warnings, if present.
	if len(importSpecResult.Warning) > 0 {
		d.reportOvfWarnings(importSpecResult.Warning, ui)
	}

	log.Printf("[INFO] Starting OVF import operation...")

	// Import the vApp using the generated spec with progress monitoring.
	lease, err := resourcePool.pool.ImportVApp(ctx, importSpecResult.ImportSpec, folder.folder, nil)
	if err != nil {
		return nil, d.wrapOvfError("failed to start vApp import", err, config.URL)
	}

	// Wait for the lease to be ready with enhanced progress monitoring.
	info, err := d.waitForOvfImportWithProgress(ctx, lease, ui)
	if err != nil {
		return nil, d.wrapOvfError("OVF import operation failed", err, config.URL)
	}

	// Validate that we received a valid VM reference
	if info == nil || info.Entity.Type != "VirtualMachine" {
		return nil, fmt.Errorf("OVF deployment completed but did not return a valid virtual machine reference")
	}

	// Get the imported VM reference from the lease info.
	vmRef := info.Entity
	vm := d.NewVM(&vmRef)
	if err := d.applyOvfPostImportConfig(vm, config); err != nil {
		return nil, d.wrapOvfError("failed to apply post-import virtual machine configuration", err, config.URL)
	}
	return vm, nil
}

// applyOvfPostImportConfig applies settings that are not part of the OVF import spec.
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

// GetOvfOptions retrieves OVF deployment options from a remote OVF/OVA source using vSphere's native pull method.
func (d *VCenterDriver) GetOvfOptions(ctx context.Context, url string, auth *OvfAuthConfig, locale string, skipTlsVerify bool) ([]types.OvfOptionInfo, error) {
	if err := d.validateOvfURL(url); err != nil {
		return nil, d.wrapOvfError("URL validation failed", err, url)
	}

	ovfWrapper, err := d.createOvfManagerWrapper(auth, skipTlsVerify)
	if err != nil {
		return nil, d.wrapOvfError("failed to initialize OVF manager", err, url)
	}

	if locale == "" {
		locale = "US"
	}

	parseParams := d.createOvfParseParams(locale)

	parseResult, err := ovfWrapper.ParseDescriptorFromURL(ctx, url, parseParams)
	if err != nil {
		return nil, d.wrapOvfError("failed to parse OVF descriptor", err, url)
	}

	// Handle parse errors, if present.
	if len(parseResult.Error) > 0 {
		return nil, d.handleOvfValidationErrors(parseResult.Error, url)
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

// OvfManagerWrapper wraps the govmomi OVF Manager with authentication and TLS support.
type OvfManagerWrapper struct {
	manager               *ovf.Manager
	auth                  *OvfAuthConfig
	insecureSkipTLSVerify bool
}

// createOvfManagerWrapper creates a new OVF Manager wrapper with authentication and TLS support.
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

// isOvfFileURL checks if the URL points to an OVF or OVA file based on file extension.
func (d *VCenterDriver) isOvfFileURL(urlStr string) bool {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return false
	}

	path := parsedURL.Path
	return strings.HasSuffix(strings.ToLower(path), ".ovf") || strings.HasSuffix(strings.ToLower(path), ".ova")
}

// validateOvfDeploymentConfig validates the complete OVF deployment configuration.
func (d *VCenterDriver) validateOvfDeploymentConfig(config *OvfDeployConfig) error {
	if config == nil {
		return fmt.Errorf("OVF deployment configuration cannot be nil")
	}

	if config.URL == "" {
		return fmt.Errorf("OVF URL is required")
	}
	if config.Name == "" {
		return fmt.Errorf("VM name is required")
	}

	if err := d.validateOvfURL(config.URL); err != nil {
		return err
	}

	if err := d.validateOvfAuthentication(config.Authentication); err != nil {
		return err
	}

	if !d.isOvfFileURL(config.URL) {
		return fmt.Errorf("URL must point to an OVF (.ovf) or OVA (.ova) file")
	}

	// Validate TLS configuration.
	if config.SkipTlsVerify {
		parsedURL, _ := url.Parse(config.URL)
		if parsedURL.Scheme == "http" {
			return fmt.Errorf("skip_tls_verify is only applicable for HTTPS URLs, but URL uses HTTP protocol")
		}
	}

	return nil
}

// createOvfImportParams creates import parameters with authentication and configuration support.
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

// CreateImportSpecFromURL creates an import spec from a remote URL with authentication and TLS support.
func (w *OvfManagerWrapper) CreateImportSpecFromURL(ctx context.Context, url string, rp *object.ResourcePool, ds types.ManagedObjectReference, params *types.OvfCreateImportSpecParams) (*types.OvfCreateImportSpecResult, error) {
	authenticatedURL, err := w.prepareAuthenticatedURL(url)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare authenticated URL: %s", err)
	}

	// Configure TLS settings if needed
	if w.insecureSkipTLSVerify {
		ctx = w.configureTLSContext(ctx)
	}

	result, err := w.manager.CreateImportSpec(ctx, authenticatedURL, rp, ds, params)
	if err != nil {
		return nil, w.categorizeOvfManagerError(err, url)
	}

	return result, nil
}

// ParseDescriptorFromURL parses an OVF descriptor from a remote URL with authentication and TLS support.
func (w *OvfManagerWrapper) ParseDescriptorFromURL(ctx context.Context, url string, params types.OvfParseDescriptorParams) (*types.OvfParseDescriptorResult, error) {
	authenticatedURL, err := w.prepareAuthenticatedURL(url)
	if err != nil {
		return nil, fmt.Errorf("failed to prepare authenticated URL: %s", err)
	}

	// Configure TLS settings, if needed.
	if w.insecureSkipTLSVerify {
		ctx = w.configureTLSContext(ctx)
	}

	result, err := w.manager.ParseDescriptor(ctx, authenticatedURL, params)
	if err != nil {
		return nil, w.categorizeOvfManagerError(err, url)
	}

	return result, nil
}

// categorizeOvfManagerError provides specific error categorization for OVF Manager operations.
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

// prepareAuthenticatedURL prepares a URL with authentication credentials if provided.
func (w *OvfManagerWrapper) prepareAuthenticatedURL(originalURL string) (string, error) {
	if w.auth == nil || (w.auth.Username == "" && w.auth.Password == "") {
		return originalURL, nil
	}

	parsedURL, err := url.Parse(originalURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL format: %s", err)
	}

	if w.auth.Username != "" && w.auth.Password != "" {
		parsedURL.User = url.UserPassword(w.auth.Username, w.auth.Password)
	}

	return parsedURL.String(), nil
}

// configureTLSContext adds TLS configuration to the context for OVF Manager operations.
// The govmomi OVF Manager delegates HTTP requests to vSphere, so TLS configuration
// is primarily handled by vSphere's internal HTTP client.
func (w *OvfManagerWrapper) configureTLSContext(ctx context.Context) context.Context {
	// Add TLS configuration to context for potential use by custom transports.
	type tlsConfigKey struct{}
	tlsConfig := &tls.Config{
		InsecureSkipVerify: w.insecureSkipTLSVerify,
	}
	return context.WithValue(ctx, tlsConfigKey{}, tlsConfig)
}

// waitForOvfImportWithProgress waits for OVF import completion with enhanced progress monitoring.
func (d *VCenterDriver) waitForOvfImportWithProgress(ctx context.Context, lease *nfc.Lease, ui packersdk.Ui) (*nfc.LeaseInfo, error) {
	importCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	progressMonitor := NewOvfProgressMonitor(ui, importCtx)
	defer progressMonitor.Cancel()

	resultChan := make(chan *nfc.LeaseInfo, 1)
	errorChan := make(chan error, 1)

	go func() {
		defer close(resultChan)
		defer close(errorChan)

		info, err := d.monitorLeaseProgress(importCtx, lease, progressMonitor)
		if err != nil {
			errorChan <- err
			return
		}

		resultChan <- info
	}()

	select {
	case info := <-resultChan:
		log.Printf("[INFO] OVF/OVA deployment completed successfully")
		return info, nil

	case err := <-errorChan:
		d.cleanupOvfDeploymentResources(importCtx, lease, ui, "deployment error")
		return nil, d.categorizeOvfImportError(err)

	case <-ctx.Done():
		ui.Say("OVF deployment cancelled, cleaning up...")
		cancel()

		d.cleanupOvfDeploymentResources(context.Background(), lease, ui, "deployment cancellation")
		return nil, fmt.Errorf("OVF deployment was cancelled")
	}
}

// cleanupOvfDeploymentResources performs comprehensive cleanup of vSphere resources during OVF deployment failures.
func (d *VCenterDriver) cleanupOvfDeploymentResources(ctx context.Context, lease *nfc.Lease, ui packersdk.Ui, reason string) {
	log.Printf("[INFO] Cleaning up vSphere resources due to %s...", reason)

	cleanupCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	var cleanupErrors []string

	if lease != nil {
		log.Printf("[INFO] Aborting vSphere NFC lease...")
		if abortErr := lease.Abort(cleanupCtx, nil); abortErr != nil {
			errorMsg := fmt.Sprintf("Failed to abort NFC lease: %s", abortErr)
			ui.Errorf("Failed to abort NFC lease: %s", abortErr)
			cleanupErrors = append(cleanupErrors, errorMsg)
		} else {
			log.Printf("[INFO] Successfully aborted NFC lease")
		}

		log.Printf("[INFO] Waiting for lease state to stabilize...")
		time.Sleep(2 * time.Second)
	}

	if len(cleanupErrors) > 0 {
		ui.Errorf("Resource cleanup completed with %d error(s):", len(cleanupErrors))
		for i, err := range cleanupErrors {
			ui.Errorf("  %d. %s", i+1, err)
		}
	} else {
		log.Printf("[INFO] Resource cleanup completed successfully")
	}
}

// categorizeOvfImportError categorizes OVF import errors and provides actionable error messages.
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
func (d *VCenterDriver) wrapOvfError(context string, err error, url string) error {
	sanitizedURL := SanitizeOvfURL(url)
	sanitizedErr := SanitizeOvfErrorMessage(err.Error())
	return fmt.Errorf("%s for OVF source '%s': %s", context, sanitizedURL, sanitizedErr)
}

// validateRemoteOvfAccessibility performs early validation of remote OVF accessibility.
func (d *VCenterDriver) validateRemoteOvfAccessibility(ctx context.Context, config *OvfDeployConfig, wrapper *OvfManagerWrapper) (*types.OvfParseDescriptorResult, error) {
	locale := config.Locale
	if locale == "" {
		locale = "US"
	}

	parseParams := d.createOvfParseParams(locale)
	parseResult, err := wrapper.ParseDescriptorFromURL(ctx, config.URL, parseParams)
	if err != nil {
		return nil, fmt.Errorf("failed to access or parse remote OVF descriptor: %s", err)
	}

	if len(parseResult.Error) > 0 {
		return nil, d.handleOvfValidationErrors(parseResult.Error, config.URL)
	}

	return parseResult, nil
}

// handleOvfValidationErrors processes OVF validation errors and provides detailed error messages.
func (d *VCenterDriver) handleOvfValidationErrors(errors []types.LocalizedMethodFault, url string) error {
	sanitizedURL := SanitizeOvfURL(url)

	if len(errors) == 1 {
		return fmt.Errorf("OVF validation failed for '%s': %s", sanitizedURL, errors[0].LocalizedMessage)
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
		sanitizedURL, len(errors), strings.Join(errorMessages, "\n"))
}

// min returns the minimum of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
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

func (d *VCenterDriver) Cleanup() (error, error) {
	return d.RestClient.client.Logout(d.Ctx), d.Client.SessionManager.Logout(d.Ctx)
}

// RestClient manages RESTful interactions with vCenter.
type RestClient struct {
	client      *rest.Client
	credentials *url.Userinfo
}

func (r *RestClient) Login(ctx context.Context) error {
	return r.client.Login(ctx, r.credentials)
}

func (r *RestClient) Logout(ctx context.Context) error {
	return r.client.Logout(ctx)
}

// Client returns the underlying rest.Client for direct API access.
func (r *RestClient) Client() *rest.Client {
	return r.client
}
