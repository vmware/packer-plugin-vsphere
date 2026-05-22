Type: `vsphere-iso`

Artifact BuilderId: `vmware.vsphere`

This builder starts from a guest operating system ISO file and builds a virtual machine image on a
vSphere cluster or an ESX host using the vSphere API.

-> **Note:** This builder is developed to maintain compatibility with VMware vSphere versions until
their respective End of General Support dates. For detailed information, refer to the
[Broadcom Product Lifecycle](https://support.broadcom.com/group/ecx/productlifecycle).

## Examples

- Examples are available in the [examples](https://github.com/vmware/packer-plugin-vsphere/tree/main/builder/vsphere/examples/)
  directory of the GitHub repository.

- Advanced examples are available in the [`vmware/packer-examples-for-vsphere`](https://github.com/vmware/packer-examples-for-vsphere)
  GitHub repository.

## Configuration Reference

### HTTP Configuration

<!-- Code generated from the comments of the HTTPConfig struct in multistep/commonsteps/http_config.go; DO NOT EDIT MANUALLY -->

Packer will create an http server serving `http_directory` when it is set, a
random free port will be selected and the architecture of the directory
referenced will be available in your builder.

Example usage from a builder:

```
wget http://{{ .HTTPIP }}:{{ .HTTPPort }}/foo/bar/preseed.cfg
```

<!-- End of code generated from the comments of the HTTPConfig struct in multistep/commonsteps/http_config.go; -->


**Optional**:

<!-- Code generated from the comments of the HTTPConfig struct in multistep/commonsteps/http_config.go; DO NOT EDIT MANUALLY -->

- `http_directory` (string) - Path to a directory to serve using an HTTP server. The files in this
  directory will be available over HTTP that will be requestable from the
  virtual machine. This is useful for hosting kickstart files and so on.
  By default this is an empty string, which means no HTTP server will be
  started. The address and port of the HTTP server will be available as
  variables in `boot_command`. This is covered in more detail below.

- `http_content` (map[string]string) - Key/Values to serve using an HTTP server. `http_content` works like and
  conflicts with `http_directory`. The keys represent the paths and the
  values contents, the keys must start with a slash, ex: `/path/to/file`.
  `http_content` is useful for hosting kickstart files and so on. By
  default this is empty, which means no HTTP server will be started. The
  address and port of the HTTP server will be available as variables in
  `boot_command`. This is covered in more detail below.
  Example:
  ```hcl
    http_content = {
      "/a/b"     = file("http/b")
      "/foo/bar" = templatefile("${path.root}/preseed.cfg", { packages = ["nginx"] })
    }
  ```

- `http_port_min` (int) - These are the minimum and maximum port to use for the HTTP server
  started to serve the `http_directory`. Because Packer often runs in
  parallel, Packer will choose a randomly available port in this range to
  run the HTTP server. If you want to force the HTTP server to be on one
  port, make this minimum and maximum port the same. By default the values
  are `8000` and `9000`, respectively.

- `http_port_max` (int) - HTTP Port Max

- `http_bind_address` (string) - This is the bind address for the HTTP server. Defaults to 0.0.0.0 so that
  it will work with any network interface.

- `http_network_protocol` (string) - Defines the HTTP Network protocol. Valid options are `tcp`, `tcp4`, `tcp6`,
  `unix`, and `unixpacket`. This value defaults to `tcp`.

<!-- End of code generated from the comments of the HTTPConfig struct in multistep/commonsteps/http_config.go; -->


- `http_interface` (string) - The network interface (for example, `en0`, `ens192`, etc.) that the
  HTTP server will use to serve the `http_directory`. The plugin will identify the IP address
  associated with this network interface and bind to it.

<!-- Code generated from the comments of the BootConfig struct in builder/vsphere/common/step_boot_command.go; DO NOT EDIT MANUALLY -->

- `http_ip` (string) - The IP address to use for the HTTP server to serve the `http_directory`.

<!-- End of code generated from the comments of the BootConfig struct in builder/vsphere/common/step_boot_command.go; -->


~> **Notes:**
  - The options `http_bind_address` and `http_interface` are mutually exclusive.
  - Both `http_bind_address` and `http_interface` have higher priority than `http_ip`.
  - The `http_bind_address` is matched against the IP addresses of the host's network interfaces. If
    no match is found, the plugin will terminate.
  - Similarly, `http_interface` is compared with the host's network interfaces. If there's no
    corresponding network interface, the plugin will also terminate.
  - If neither `http_bind_address`, `http_interface`, and `http_ip` are provided, the plugin will
    automatically find and use the IP address of the first non-loopback interface for `http_ip`.

### Connection Configuration

**Optional**:

<!-- Code generated from the comments of the ConnectConfig struct in builder/vsphere/common/step_connect.go; DO NOT EDIT MANUALLY -->

- `vcenter_server` (string) - The fully qualified domain name or IP address of the vCenter instance.

- `username` (string) - The username to authenticate with the vCenter instance.

- `password` (string) - The password to authenticate with the vCenter instance.

- `insecure_connection` (bool) - Do not validate the certificate of the vCenter instance.
  Defaults to `false`.
  
  -> **Note:** This option is beneficial in scenarios where the certificate
  is self-signed or does not meet standard validation criteria.

- `datacenter` (string) - The name of the datacenter object in the vSphere inventory.
  
  -> **Note:** Required if more than one datacenter object exists in the
  vSphere inventory.

<!-- End of code generated from the comments of the ConnectConfig struct in builder/vsphere/common/step_connect.go; -->


### Location Configuration

**Optional**:

<!-- Code generated from the comments of the LocationConfig struct in builder/vsphere/common/config_location.go; DO NOT EDIT MANUALLY -->

- `vm_name` (string) - The name of the virtual machine.

- `folder` (string) - The virtual machine folder where the virtual machine is created.

- `cluster` (string) - The cluster where the virtual machine is created.
  Refer to the [Working With Clusters And Hosts](#working-with-clusters-and-hosts)
  section for more details.

- `host` (string) - The ESX host where the virtual machine is created. A full path must be specified
  if the ESX host is in a folder. For example `folder/host`.
  Refer to the [Working With Clusters And Hosts](#working-with-clusters-and-hosts)
  section for more details.

- `resource_pool` (string) - The resource pool where the virtual machine is created.
  If this is not specified, the root resource pool associated with the
  `host` or `cluster` is used.
  
  ~> **Note:**  The full path to the resource pool must be provided.
  For example, a simple resource pool path might resemble `rp-packer` and
  a nested path might resemble 'rp-packer/rp-linux-images'.

- `datastore` (string) - The datastore where the virtual machine is created.
  Required if `host` is a cluster or if `host` has multiple datastores,
  unless `datastore_cluster` is specified.
  
  ~> **Note:** Cannot be used with `datastore_cluster`.

- `datastore_cluster` (string) - The datastore cluster where the virtual machine is created.
  When specified, Storage DRS will automatically select the optimal datastore.
  
  ~> **Note:** Cannot be used with `datastore`.

- `set_host_for_datastore_uploads` (bool) - The ESX host used for uploading files to the datastore.
  Defaults to `false`.

<!-- End of code generated from the comments of the LocationConfig struct in builder/vsphere/common/config_location.go; -->


<!-- Code generated from the comments of the Config struct in builder/vsphere/iso/config.go; DO NOT EDIT MANUALLY -->

- `vapp` (common.VAppConfig) - The vApp Options for the virtual machine. For more information, refer to the
  [vApp Options Configuration](/packer/integrations/hashicorp/vmware/latest/components/builder/vsphere-iso#vapp-options-configuration)
  section.

- `create_snapshot` (bool) - Create a snapshot of the virtual machine to use as a base for linked clones.
  Defaults to `false`.

- `snapshot_name` (string) - The name of the snapshot when `create_snapshot` is `true`.
  Defaults to `Created By Packer`.

- `convert_to_template` (bool) - Convert the virtual machine to a template after the build is complete.
  Defaults to `false`.
  If set to `true`, the virtual machine cannot be imported into a content library.

- `export` (\*common.ExportConfig) - The configuration for exporting the virtual machine to an OVF.
  The virtual machine is not exported if [export configuration](#export-configuration) is not specified.

- `content_library_destination` (\*common.ContentLibraryDestinationConfig) - Import the virtual machine as a VM template or OVF template to a content library.
  The template will not be imported if no [content library import configuration](#content-library-import-configuration) is specified.
  If set, `convert_to_template` must be set to `false`.

- `local_cache_overwrite` (bool) - Overwrite files in the local cache if they already exist.
  Defaults to `false`.

- `remote_cache_cleanup` (bool) - Cleanup items added to the remote cache after the build is complete.
  Defaults to `false`.
  
  -> **Note:** If the local cache overwrite flag is set to `true`, `RemoteCacheOverwrite` will
  implicitly be set to `true`. This is to ensure consistency between the local and remote
  cache.

- `remote_cache_overwrite` (bool) - Overwrite files in the remote cache if they already exist.
  Defaults to `false`.

- `remote_cache_datastore` (string) - The remote cache datastore to use for the build.
  If not set, the datastore of the virtual machine is used.

- `remote_cache_path` (string) - The directory path on the remote cache datastore to use for the build.
  If not set, the default path is `packer_cache/`.

<!-- End of code generated from the comments of the Config struct in builder/vsphere/iso/config.go; -->


### Hardware Configuration

**Optional**:

<!-- Code generated from the comments of the HardwareConfig struct in builder/vsphere/common/step_hardware.go; DO NOT EDIT MANUALLY -->

- `CPUs` (int32) - The number of virtual CPUs cores for the virtual machine.

- `cpu_cores` (int32) - The number of virtual CPU cores per socket for the virtual machine.

- `CPU_reservation` (int64) - The CPU reservation in MHz.

- `CPU_limit` (int64) - The upper limit of available CPU resources in MHz.

- `CPU_hot_plug` (bool) - Enable CPU hot plug setting for virtual machine. Defaults to `false`

- `RAM` (int64) - The amount of memory for the virtual machine in MB.

- `RAM_reservation` (int64) - The guaranteed minimum allocation of memory for the virtual machine in MB.

- `RAM_reserve_all` (bool) - Reserve all allocated memory. Defaults to `false`.
  
  -> **Note:** May not be used together with `RAM_reservation`.

- `RAM_hot_plug` (bool) - Enable memory hot add setting for virtual machine. Defaults to `false`.

- `video_ram` (int64) - The amount of video memory in KB. Defaults to 4096 KB.
  
  -> **Note:** Refer to the [vSphere documentation](https://techdocs.broadcom.com/us/en/vmware-cis/vsphere/vsphere/8-0/vsphere-virtual-machine-administration-guide-8-0/configuring-virtual-machine-hardwarevsphere-vm-admin/virtual-machine-compatibilityvsphere-vm-admin/hardware-features-available-with-virtual-machine-compatibility-levelsvsphere-vm-admin.html)
  for supported maximums.

- `displays` (int32) - The number of video displays. Defaults to `1`.
  
  `-> **Note:** Refer to the [vSphere documentation](https://techdocs.broadcom.com/us/en/vmware-cis/vsphere/vsphere/8-0/vsphere-virtual-machine-administration-guide-8-0/configuring-virtual-machine-hardwarevsphere-vm-admin/virtual-machine-compatibilityvsphere-vm-admin/hardware-features-available-with-virtual-machine-compatibility-levelsvsphere-vm-admin.html)
  for supported maximums.

- `pci_passthrough_allowed_device` ([]PCIPassthroughAllowedDevice) - Configure Dynamic DirectPath I/O [PCI Passthrough](#pci-passthrough-configuration) for
  virtual machine. Refer to the [vSphere documentation](https://techdocs.broadcom.com/us/en/vmware-cis/vsphere/vsphere/8-0/vsphere-virtual-machine-administration-guide-8-0/configuring-virtual-machine-hardwarevsphere-vm-admin/other-virtual-machine-device-configurationvsphere-vm-admin/add-a-pci-device-to-a-virutal-machinevsphere-vm-admin.html)

- `vgpu_profile` (string) - vGPU profile for accelerated graphics. Refer to the [NVIDIA GRID vGPU documentation](https://docs.nvidia.com/grid/latest/grid-vgpu-user-guide/index.html#configure-vmware-vsphere-vm-with-vgpu)
  for examples of profile names. Defaults to none.

- `NestedHV` (bool) - Enable nested hardware virtualization for the virtual machine.

- `firmware` (string) - The firmware for the virtual machine.
  
  The available options for this setting are: 'bios', 'efi', and
  'efi-secure'.
  
  -> **Note:** Use `efi-secure` for UEFI Secure Boot.

- `force_bios_setup` (bool) - Force entry into the BIOS setup screen during boot. Defaults to `false`.

- `vTPM` (bool) - Enable virtual trusted platform module (TPM) device for the virtual
  machine. Defaults to `false`.

- `precision_clock` (string) - The virtual precision clock device for the virtual machine.
  Defaults to `none`.
  
  The available options for this setting are: `none`, `ntp`, and `ptp`.

<!-- End of code generated from the comments of the HardwareConfig struct in builder/vsphere/common/step_hardware.go; -->


### Create Configuration

**Optional**:

<!-- Code generated from the comments of the CreateConfig struct in builder/vsphere/iso/step_create.go; DO NOT EDIT MANUALLY -->

- `vm_version` (uint) - Specifies the virtual machine hardware version. Defaults to the most
  current virtual machine hardware version supported by the ESX host.
  Refer to [KB 315655](https://knowledge.broadcom.com/external/article?articleNumber=315655)
  for more information on supported virtual hardware versions.

- `guest_os_type` (string) - The guest operating system identifier for the virtual machine.
  Defaults to `otherGuest`.
  
  To get a list of supported guest operating system identifiers for your
  ESX host, run the following PowerShell command using `VMware.PowerCLI`:
  
  ```powershell
  Connect-VIServer -Server "vc01.example.com" -User "administrator@vsphere.local" -Password "password"
  $esxHost = Get-VMHost -Name "esx01.example.com"
  $environmentBrowser = Get-View -Id $esxHost.ExtensionData.Parent.ExtensionData.ConfigManager.EnvironmentBrowser
  $vmxVersion = ($environmentBrowser.QueryConfigOptionDescriptor() | Where-Object DefaultConfigOption).Key
  $osDescriptor = $environmentBrowser.QueryConfigOption($vmxVersion, $null).GuestOSDescriptor
  $osDescriptor | Select-Object Id, Fullname
  ```

- `network_adapters` ([]NIC) - The network adapters for the virtual machine.
  
  -> **Note:** If no network adapter is defined, all network-related
  operations are skipped.

- `usb_controller` ([]string) - The USB controllers for the virtual machine.
  
  The available options for this setting are: `usb` and `xhci`.
  
  - `usb`: USB 2.0
  - `xhci`: USB 3.0
  
  -> **Note:** A maximum of one of each controller type can be defined.

- `notes` (string) - The annotations for the virtual machine.

- `destroy` (bool) - Destroy the virtual machine after the build completes.
  Defaults to `false`.

<!-- End of code generated from the comments of the CreateConfig struct in builder/vsphere/iso/step_create.go; -->


### vApp Options Configuration

**Optional:**

- `properties` (map[string]string) - The values for the available vApp properties. These are used to supply
  configuration parameters to the virtual machine during installation.

  Omitting the `vapp` block entirely disables vApp support. Including at least one property enables vApp
  options for scratch-built virtual machines and creates any listed keys that do not already exist.

  HCL Example:

  ```hcl
  vapp {
    properties = {
      hostname      = var.hostname
      public-keys = ""
    }
  }
  ```

  JSON Example:

  ```json
  "vapp": {
    "properties": {
      "hostname": "{{ user `hostname`}}",
      "public-keys": ""
    }
  }
  ```

  A `user-data` field requires the content of a YAML file to be encoded with base64. This can be done
  using an environment variable:

  ```console
  export USERDATA=$(gzip -c9 <userdata.yaml | { base64 -w0 2>/dev/null || base64; })
  ```


### ISO Configuration

<!-- Code generated from the comments of the ISOConfig struct in multistep/commonsteps/iso_config.go; DO NOT EDIT MANUALLY -->

By default, Packer will symlink, download or copy image files to the Packer
cache into a "`hash($iso_url+$iso_checksum).$iso_target_extension`" file.
Packer uses [hashicorp/go-getter](https://github.com/hashicorp/go-getter) in
file mode in order to perform a download.

go-getter supports the following protocols:

* Local files
* Git
* Mercurial
* HTTP
* Amazon S3

Examples:
go-getter can guess the checksum type based on `iso_checksum` length, and it is
also possible to specify the checksum type.

In JSON:

```json

	"iso_checksum": "946a6077af6f5f95a51f82fdc44051c7aa19f9cfc5f737954845a6050543d7c2",
	"iso_url": "ubuntu.org/.../ubuntu-14.04.1-server-amd64.iso"

```

```json

	"iso_checksum": "file:ubuntu.org/..../ubuntu-14.04.1-server-amd64.iso.sum",
	"iso_url": "ubuntu.org/.../ubuntu-14.04.1-server-amd64.iso"

```

```json

	"iso_checksum": "file://./shasums.txt",
	"iso_url": "ubuntu.org/.../ubuntu-14.04.1-server-amd64.iso"

```

```json

	"iso_checksum": "file:./shasums.txt",
	"iso_url": "ubuntu.org/.../ubuntu-14.04.1-server-amd64.iso"

```

In HCL2:

```hcl

	iso_checksum = "946a6077af6f5f95a51f82fdc44051c7aa19f9cfc5f737954845a6050543d7c2"
	iso_url = "ubuntu.org/.../ubuntu-14.04.1-server-amd64.iso"

```

```hcl

	iso_checksum = "file:ubuntu.org/..../ubuntu-14.04.1-server-amd64.iso.sum"
	iso_url = "ubuntu.org/.../ubuntu-14.04.1-server-amd64.iso"

```

```hcl

	iso_checksum = "file://./shasums.txt"
	iso_url = "ubuntu.org/.../ubuntu-14.04.1-server-amd64.iso"

```

```hcl

	iso_checksum = "file:./shasums.txt",
	iso_url = "ubuntu.org/.../ubuntu-14.04.1-server-amd64.iso"

```

<!-- End of code generated from the comments of the ISOConfig struct in multistep/commonsteps/iso_config.go; -->


**Required**:

<!-- Code generated from the comments of the ISOConfig struct in multistep/commonsteps/iso_config.go; DO NOT EDIT MANUALLY -->

- `iso_checksum` (string) - The checksum for the ISO file or virtual hard drive file. The type of
  the checksum is specified within the checksum field as a prefix, ex:
  "md5:{$checksum}". The type of the checksum can also be omitted and
  Packer will try to infer it based on string length. Valid values are
  "none", "{$checksum}", "md5:{$checksum}", "sha1:{$checksum}",
  "sha256:{$checksum}", "sha512:{$checksum}" or "file:{$path}". Here is a
  list of valid checksum values:
   * md5:090992ba9fd140077b0661cb75f7ce13
   * 090992ba9fd140077b0661cb75f7ce13
   * sha1:ebfb681885ddf1234c18094a45bbeafd91467911
   * ebfb681885ddf1234c18094a45bbeafd91467911
   * sha256:ed363350696a726b7932db864dda019bd2017365c9e299627830f06954643f93
   * ed363350696a726b7932db864dda019bd2017365c9e299627830f06954643f93
   * file:http://releases.ubuntu.com/20.04/SHA256SUMS
   * file:file://./local/path/file.sum
   * file:./local/path/file.sum
   * none
  Although the checksum will not be verified when it is set to "none",
  this is not recommended since these files can be very large and
  corruption does happen from time to time.

- `iso_url` (string) - A URL to the ISO containing the installation image or virtual hard drive
  (VHD or VHDX) file to clone.

<!-- End of code generated from the comments of the ISOConfig struct in multistep/commonsteps/iso_config.go; -->


**Optional**:

<!-- Code generated from the comments of the ISOConfig struct in multistep/commonsteps/iso_config.go; DO NOT EDIT MANUALLY -->

- `iso_urls` ([]string) - Multiple URLs for the ISO to download. Packer will try these in order.
  If anything goes wrong attempting to download or while downloading a
  single URL, it will move on to the next. All URLs must point to the same
  file (same checksum). By default this is empty and `iso_url` is used.
  Only one of `iso_url` or `iso_urls` can be specified.

- `iso_target_path` (string) - The path where the iso should be saved after download. By default will
  go in the packer cache, with a hash of the original filename and
  checksum as its name.

- `iso_target_extension` (string) - The extension of the iso file after download. This defaults to `iso`.

<!-- End of code generated from the comments of the ISOConfig struct in multistep/commonsteps/iso_config.go; -->


### CD-ROM Configuration

For each ISO defined in the CD-ROM configuration, a CD-ROM device is added.

If the `iso_url` is defined in addition to the `iso_paths`, the `iso_url` is added to the virtual
machine first. This keeps the `iso_url` first in the boot order by default, allowing the boot ISO to
be defined by the `iso_url` and the VMware Tools ISO added from ESX host.

HCL Example:

```hcl
source "vsphere-iso" "example" {
  # ... other configuration ...
  iso_urls = [
    "windows-server.iso",
    "https://example.com/isos/windows-server.iso"
  ]

  iso_paths = [
    "[] /usr/lib/vmware/isoimages/windows.iso"
  ]
  # ... other configuration ...
}
```

JSON Example:

```json
  "iso_urls": [
    "windows-server.iso",
    "https://example.com/isos/windows-server.iso"
  ],
  "iso_paths": [
      "[] /usr/lib/vmware/isoimages/windows.iso"
  ],
```

<!-- Code generated from the comments of the CDConfig struct in multistep/commonsteps/extra_iso_config.go; DO NOT EDIT MANUALLY -->

An iso (CD) containing custom files can be made available for your build.

By default, no extra CD will be attached. All files listed in this setting
get placed into the root directory of the CD and the CD is attached as the
second CD device.

This config exists to work around modern operating systems that have no
way to mount floppy disks, which was our previous go-to for adding files at
boot time.

<!-- End of code generated from the comments of the CDConfig struct in multistep/commonsteps/extra_iso_config.go; -->


**Optional**:

<!-- Code generated from the comments of the CDConfig struct in multistep/commonsteps/extra_iso_config.go; DO NOT EDIT MANUALLY -->

- `cd_files` ([]string) - A list of files to place onto a CD that is attached when the VM is
  booted. This can include either files or directories; any directories
  will be copied onto the CD recursively, preserving directory structure
  hierarchy. Symlinks will have the link's target copied into the directory
  tree on the CD where the symlink was. File globbing is allowed.
  
  Usage example (JSON):
  
  ```json
  "cd_files": ["./somedirectory/meta-data", "./somedirectory/user-data"],
  "cd_label": "cidata",
  ```
  
  Usage example (HCL):
  
  ```hcl
  cd_files = ["./somedirectory/meta-data", "./somedirectory/user-data"]
  cd_label = "cidata"
  ```
  
  The above will create a CD with two files, user-data and meta-data in the
  CD root. This specific example is how you would create a CD that can be
  used for an Ubuntu 20.04 autoinstall.
  
  Since globbing is also supported,
  
  ```hcl
  cd_files = ["./somedirectory/*"]
  cd_label = "cidata"
  ```
  
  Would also be an acceptable way to define the above cd. The difference
  between providing the directory with or without the glob is whether the
  directory itself or its contents will be at the CD root.
  
  Use of this option assumes that you have a command line tool installed
  that can handle the iso creation. Packer will use one of the following
  tools:
  
    * xorriso
    * mkisofs
    * hdiutil (normally found in macOS)
    * oscdimg (normally found in Windows as part of the Windows ADK)

- `cd_content` (map[string]string) - Key/Values to add to the CD. The keys represent the paths, and the values
  contents. It can be used alongside `cd_files`, which is useful to add large
  files without loading them into memory. If any paths are specified by both,
  the contents in `cd_content` will take precedence.
  
  Usage example (HCL):
  
  ```hcl
  cd_files = ["vendor-data"]
  cd_content = {
    "meta-data" = jsonencode(local.instance_data)
    "user-data" = templatefile("user-data", { packages = ["nginx"] })
  }
  cd_label = "cidata"
  ```

- `cd_label` (string) - CD Label

<!-- End of code generated from the comments of the CDConfig struct in multistep/commonsteps/extra_iso_config.go; -->


<!-- Code generated from the comments of the CDRomConfig struct in builder/vsphere/common/step_add_cdrom.go; DO NOT EDIT MANUALLY -->

- `cdrom_type` (string) - The type of controller to use for the CD-ROM device. Defaults to `ide`.
  
  The available options for this setting are: `ide` and `sata`.

- `iso_paths` ([]string) - A list of paths to ISO files in either a datastore or a content library
  that will be attached to the virtual machine.
  
  HCL Example:
  
  ```hcl
  iso_paths = [
    "[nfs] iso/ubuntu-server-amd64.iso",
    "Example Content Library/ubuntu-server-amd64/ubuntu-server-amd64.iso"
  ]
  ```
  
  JSON Example:
  
  ```json
  "iso_paths": [
    "[nfs] iso/ubuntu-server-amd64.iso",
    "Example Content Library/ubuntu-server-amd64/ubuntu-server-amd64.iso"
  ]
  ```
  
  Two ISOs are referenced:
  
  1. An ISO in the "_iso_" folder of the "_nfs_" datastore with the file
    name of "_ubuntu-server-amd64.iso_". "_ubuntu-server-amd64.iso_".
  2. An ISO in the "_Example Content Library_" content library with the
    item name of "_ubuntu-server-amd64_".
  
  -> **Note:** All files in a content library have an associated item name.
  To determine the file name, view the datastore backing the content
  library or use the `govc` vSphere CLI.

<!-- End of code generated from the comments of the CDRomConfig struct in builder/vsphere/common/step_add_cdrom.go; -->


<!-- Code generated from the comments of the RemoveCDRomConfig struct in builder/vsphere/common/step_remove_cdrom.go; DO NOT EDIT MANUALLY -->

- `remove_cdrom` (bool) - Remove all CD-ROM devices from the virtual machine when the build is
  complete. Defaults to `false`.

<!-- End of code generated from the comments of the RemoveCDRomConfig struct in builder/vsphere/common/step_remove_cdrom.go; -->


<!-- Code generated from the comments of the ReattachCDRomConfig struct in builder/vsphere/common/step_reattach_cdrom.go; DO NOT EDIT MANUALLY -->

- `reattach_cdroms` (int) - The number of CD-ROM devices to reattach to the final build artifact.
  Range: 0 - 4. Defaults to 0.
  
  -> **Note:** If set to 0, the step is skipped. When set to a value in the
  range, `remove_cdrom` is ignored and the CD-ROM devices are kept without
  any attached media.

<!-- End of code generated from the comments of the ReattachCDRomConfig struct in builder/vsphere/common/step_reattach_cdrom.go; -->


### Floppy Configuration

**Optional**:

<!-- Code generated from the comments of the FloppyConfig struct in builder/vsphere/common/step_add_floppy.go; DO NOT EDIT MANUALLY -->

- `floppy_img_path` (string) - Datastore path to a floppy image that will be mounted to the virtual
  machine. Example: `[datastore] iso/foo.flp`.

- `floppy_files` ([]string) - A list of local files to be mounted to the virtual machine's floppy
  drive.

- `floppy_dirs` ([]string) - A list of directories to copy files from.

- `floppy_content` (map[string]string) - Key/Values to add to the floppy disk. The keys represent the paths, and
  the value contents. It can be used alongside `floppy_files` or
  `floppy_dirs`, which is useful to add large files without loading them
  into memory. If both specify a path, the contents in `floppy_content` will take
  precedence.
  
  HCL Example:
  
  ```hcl
  floppy_content = {
    "meta-data" = jsonencode(local.instance_data)
    "user-data" = templatefile("user-data", { packages = ["nginx"] })
  }
  ```

- `floppy_label` (string) - The label to use for the floppy disk that is attached when the virtual
  machine is booted. This is most useful for cloud-init, Kickstart, or other
  early initialization tools, which can benefit from labeled floppy disks.
  By default, the floppy label will be 'packer'.

<!-- End of code generated from the comments of the FloppyConfig struct in builder/vsphere/common/step_add_floppy.go; -->


### Network Adapter Configuration

<!-- Code generated from the comments of the NIC struct in builder/vsphere/iso/step_create.go; DO NOT EDIT MANUALLY -->

If no adapter is defined, network tasks (communicators, most provisioners)
will not work, so it's advised to define one.

Example configuration with two network adapters:

HCL Example:

```hcl

	network_adapters {
	    network = "VM Network"
	    network_card = "vmxnet3"
	}
	network_adapters {
	    network = "OtherNetwork"
	    network_card = "vmxnet3"
	}

```

JSON Example:

```json

	"network_adapters": [
	  {
	    "network": "VM Network",
	    "network_card": "vmxnet3"
	  },
	  {
	    "network": "OtherNetwork",
	    "network_card": "vmxnet3"
	  }
	],

```

<!-- End of code generated from the comments of the NIC struct in builder/vsphere/iso/step_create.go; -->


**Required**:

<!-- Code generated from the comments of the NIC struct in builder/vsphere/iso/step_create.go; DO NOT EDIT MANUALLY -->

- `network_card` (string) - The virtual machine network card type. For example `vmxnet3`.

<!-- End of code generated from the comments of the NIC struct in builder/vsphere/iso/step_create.go; -->


**Optional**:

<!-- Code generated from the comments of the NIC struct in builder/vsphere/iso/step_create.go; DO NOT EDIT MANUALLY -->

- `network` (string) - The network to which the virtual machine will connect.
  
  For example:
  
  - Name: `<NetworkName>`
  - Inventory Path: `/<DatacenterName>/<FolderName>/<NetworkName>`
  - Managed Object ID (Port Group): `Network:network-<xxxxx>`
  - Managed Object ID (Distributed Port Group): `DistributedVirtualPortgroup::dvportgroup-<xxxxx>`
  - Logical Switch UUID: `<uuid>`
  - Segment ID: `/infra/segments/<SegmentID>`
  
  ~> **Note:** If more than one network resolves to the same name, either
  the inventory path to network or an ID must be provided.
  
  ~> **Note:** If no network is specified, provide `host` to allow the
  plugin to search for an available network.

- `mac_address` (string) - The network card MAC address. For example `00:50:56:00:00:00`.

- `passthrough` (\*bool) - Enable DirectPath I/O passthrough for the network device.
  Defaults to `false`.

<!-- End of code generated from the comments of the NIC struct in builder/vsphere/iso/step_create.go; -->


<!-- Code generated from the comments of the RemoveNetworkConfig struct in builder/vsphere/common/step_remove_network.go; DO NOT EDIT MANUALLY -->

- `remove_network_adapter` (bool) - Remove all network adapters from template. Defaults to `false`.

<!-- End of code generated from the comments of the RemoveNetworkConfig struct in builder/vsphere/common/step_remove_network.go; -->


### Storage Configuration

<!-- Code generated from the comments of the DiskConfig struct in builder/vsphere/common/storage_config.go; DO NOT EDIT MANUALLY -->

The following example that will create a 15GB and a 20GB disk on the virtual
machine. The second disk will be thin provisioned:

HCL Example:

```hcl

	storage {
	    disk_size = 15000
	}
	storage {
	    disk_size = 20000
	    disk_thin_provisioned = true
	}

```

JSON Example:

```json

	"storage": [
	  {
	    "disk_size": 15000
	  },
	  {
	    "disk_size": 20000,
	    "disk_thin_provisioned": true
	  }
	],

```

The following example will use two PVSCSI controllers and two disks on each
controller.

HCL Example:

```hcl

	 disk_controller_type = ["pvscsi", "pvscsi"]
		storage {
		   disk_size = 15000
		   disk_controller_index = 0
		}
		storage {
		   disk_size = 15000
		   disk_controller_index = 0
		}
		storage {
		   disk_size = 15000
		   disk_controller_index = 1
		}
		storage {
		   disk_size = 15000
		   disk_controller_index = 1
		}

```

JSON Example:

```json

	"disk_controller_type": ["pvscsi", "pvscsi"],
	"storage": [
	  {
	    "disk_size": 15000,
	    "disk_controller_index": 0
	  },
	  {
	    "disk_size": 15000,
	    "disk_controller_index": 0
	  },
	  {
	    "disk_size": 15000,
	    "disk_controller_index": 1
	  },
	  {
	    "disk_size": 15000,
	    "disk_controller_index": 1
	  }
	],

```

<!-- End of code generated from the comments of the DiskConfig struct in builder/vsphere/common/storage_config.go; -->


**Required**:

<!-- Code generated from the comments of the DiskConfig struct in builder/vsphere/common/storage_config.go; DO NOT EDIT MANUALLY -->

- `disk_size` (int64) - The size of the disk in MiB.

<!-- End of code generated from the comments of the DiskConfig struct in builder/vsphere/common/storage_config.go; -->


**Optional**:

<!-- Code generated from the comments of the DiskConfig struct in builder/vsphere/common/storage_config.go; DO NOT EDIT MANUALLY -->

- `disk_thin_provisioned` (bool) - Enable thin provisioning for the disk.
  Defaults to `false`.

- `disk_eagerly_scrub` (bool) - Enable eager scrubbing for the disk.
  Defaults to `false`.

- `disk_controller_index` (int) - The assigned disk controller for the disk.
  Defaults to the first controller, `(0)`.

<!-- End of code generated from the comments of the DiskConfig struct in builder/vsphere/common/storage_config.go; -->


<!-- Code generated from the comments of the StorageConfig struct in builder/vsphere/common/storage_config.go; DO NOT EDIT MANUALLY -->

- `disk_controller_type` ([]string) - The disk controller type. One of `lsilogic`, `lsilogic-sas`, `pvscsi`,
  `nvme`, `scsi`, or `sata`. Defaults to `lsilogic`. Use a list to define
  additional controllers. Refer to [SCSI, SATA, and NVMe Storage Controller
  Conditions, Limitations, and Compatibility](https://techdocs.broadcom.com/us/en/vmware-cis/vsphere/vsphere/8-0/vsphere-virtual-machine-administration-guide-8-0/configuring-virtual-machine-hardwarevsphere-vm-admin/scsi-controller-configurationvsphere-vm-admin.html)
  for additional information.

- `storage` ([]DiskConfig) - A collection of one or more disks to be provisioned.
  Refer to the [Storage Configuration](#storage-configuration) section for additional information.

<!-- End of code generated from the comments of the StorageConfig struct in builder/vsphere/common/storage_config.go; -->


### Flag Configuration

**Optional**:

<!-- Code generated from the comments of the FlagConfig struct in builder/vsphere/common/step_add_flag.go; DO NOT EDIT MANUALLY -->

- `vbs_enabled` (bool) - Enable Virtualization Based Security option for virtual machine. Defaults to `false`.
  Requires `vvtd_enabled` and `NestedHV` to be set to `true`.
  Requires `firmware` to be set to `efi-secure`.

- `vvtd_enabled` (bool) - Enable IO/MMU option for virtual machine. Defaults to `false`.

<!-- End of code generated from the comments of the FlagConfig struct in builder/vsphere/common/step_add_flag.go; -->


### Boot Configuration

<!-- Code generated from the comments of the BootConfig struct in bootcommand/config.go; DO NOT EDIT MANUALLY -->

The boot configuration is very important: `boot_command` specifies the keys
to type when the virtual machine is first booted in order to start the OS
installer. This command is typed after boot_wait, which gives the virtual
machine some time to actually load.

The boot_command is an array of strings. The strings are all typed in
sequence. It is an array only to improve readability within the template.

There are a set of special keys available. If these are in your boot
command, they will be replaced by the proper key:

-   `<bs>` - Backspace

-   `<del>` - Delete

-   `<enter> <return>` - Simulates an actual "enter" or "return" keypress.

-   `<esc>` - Simulates pressing the escape key.

-   `<tab>` - Simulates pressing the tab key.

-   `<f1> - <f12>` - Simulates pressing a function key.

-   `<up> <down> <left> <right>` - Simulates pressing an arrow key.

-   `<spacebar>` - Simulates pressing the spacebar.

-   `<insert>` - Simulates pressing the insert key.

-   `<home> <end>` - Simulates pressing the home and end keys.

  - `<pageUp> <pageDown>` - Simulates pressing the page up and page down
    keys.

-   `<menu>` - Simulates pressing the Menu key.

-   `<leftAlt> <rightAlt>` - Simulates pressing the alt key.

-   `<leftCtrl> <rightCtrl>` - Simulates pressing the ctrl key.

-   `<leftShift> <rightShift>` - Simulates pressing the shift key.

-   `<leftSuper> <rightSuper>` - Simulates pressing the ⌘ or Windows key.

  - `<wait> <wait5> <wait10>` - Adds a 1, 5 or 10 second pause before
    sending any additional keys. This is useful if you have to generally
    wait for the UI to update before typing more.

  - `<waitXX>` - Add an arbitrary pause before sending any additional keys.
    The format of `XX` is a sequence of positive decimal numbers, each with
    optional fraction and a unit suffix, such as `300ms`, `1.5h` or `2h45m`.
    Valid time units are `ns`, `us` (or `µs`), `ms`, `s`, `m`, `h`. For
    example `<wait10m>` or `<wait1m20s>`.

  - `<XXXOn> <XXXOff>` - Any printable keyboard character, and of these
    "special" expressions, with the exception of the `<wait>` types, can
    also be toggled on or off. For example, to simulate ctrl+c, use
    `<leftCtrlOn>c<leftCtrlOff>`. Be sure to release them, otherwise they
    will be held down until the machine reboots. To hold the `c` key down,
    you would use `<cOn>`. Likewise, `<cOff>` to release.

  - `{{ .HTTPIP }} {{ .HTTPPort }}` - The IP and port, respectively of an
    HTTP server that is started serving the directory specified by the
    `http_directory` configuration parameter. If `http_directory` isn't
    specified, these will be blank!

-   `{{ .Name }}` - The name of the VM.

Example boot command. This is actually a working boot command used to start an
CentOS 6.4 installer:

In JSON:

```json
"boot_command": [

	   "<tab><wait>",
	   " ks=http://{{ .HTTPIP }}:{{ .HTTPPort }}/centos6-ks.cfg<enter>"
	]

```

In HCL2:

```hcl
boot_command = [

	   "<tab><wait>",
	   " ks=http://{{ .HTTPIP }}:{{ .HTTPPort }}/centos6-ks.cfg<enter>"
	]

```

The example shown below is a working boot command used to start an Ubuntu
12.04 installer:

In JSON:

```json
"boot_command": [

	"<esc><esc><enter><wait>",
	"/install/vmlinuz noapic ",
	"preseed/url=http://{{ .HTTPIP }}:{{ .HTTPPort }}/preseed.cfg ",
	"debian-installer=en_US auto locale=en_US kbd-chooser/method=us ",
	"hostname={{ .Name }} ",
	"fb=false debconf/frontend=noninteractive ",
	"keyboard-configuration/modelcode=SKIP keyboard-configuration/layout=USA ",
	"keyboard-configuration/variant=USA console-setup/ask_detect=false ",
	"initrd=/install/initrd.gz -- <enter>"

]
```

In HCL2:

```hcl
boot_command = [

	"<esc><esc><enter><wait>",
	"/install/vmlinuz noapic ",
	"preseed/url=http://{{ .HTTPIP }}:{{ .HTTPPort }}/preseed.cfg ",
	"debian-installer=en_US auto locale=en_US kbd-chooser/method=us ",
	"hostname={{ .Name }} ",
	"fb=false debconf/frontend=noninteractive ",
	"keyboard-configuration/modelcode=SKIP keyboard-configuration/layout=USA ",
	"keyboard-configuration/variant=USA console-setup/ask_detect=false ",
	"initrd=/install/initrd.gz -- <enter>"

]
```

For more examples of various boot commands, see the sample projects from our
[community templates page](https://packer.io/community-tools#templates).

<!-- End of code generated from the comments of the BootConfig struct in bootcommand/config.go; -->


Packer sends each character to the virtual machine with a default delay of 100ms between groups. The
delay alleviates possible issues with latency and CPU contention. If you notice missing keys, you
can tune this delay by specifying `boot_keygroup_interval` in your template.

HCL Example:

```hcl
source "vsphere-iso" "example" {
  # ... other configuration ...
  boot_keygroup_interval = "500ms"
  # ... other configuration ...
}
```

JSON Example:

```json
{
  "builders": [
    {
      "type": "vsphere-iso",
      "boot_keygroup_interval": "500ms"
    }
  ]
}
```

**Optional**:

<!-- Code generated from the comments of the RunConfig struct in builder/vsphere/common/step_run.go; DO NOT EDIT MANUALLY -->

- `boot_order` (string) - The priority of boot devices. Defaults to `disk,cdrom`.
  
  The available boot devices are: `floppy`, `cdrom`, `ethernet`, and
  `disk`.
  
  -> **Note:** If not set, the boot order is temporarily set to
  `disk,cdrom` for the duration of the build and then cleared upon
  build completion.

<!-- End of code generated from the comments of the RunConfig struct in builder/vsphere/common/step_run.go; -->


<!-- Code generated from the comments of the BootConfig struct in bootcommand/config.go; DO NOT EDIT MANUALLY -->

- `boot_keygroup_interval` (duration string | ex: "1h5m2s") - Time to wait after sending a group of key pressses. The value of this
  should be a duration. Examples are `5s` and `1m30s` which will cause
  Packer to wait five seconds and one minute 30 seconds, respectively. If
  this isn't specified, a sensible default value is picked depending on
  the builder type.

- `boot_wait` (duration string | ex: "1h5m2s") - The time to wait after booting the initial virtual machine before typing
  the `boot_command`. The value of this should be a duration. Examples are
  `5s` and `1m30s` which will cause Packer to wait five seconds and one
  minute 30 seconds, respectively. If this isn't specified, the default is
  `10s` or 10 seconds. To set boot_wait to 0s, use a negative number, such
  as "-1s"

- `boot_command` ([]string) - This is an array of commands to type when the virtual machine is first
  booted. The goal of these commands should be to type just enough to
  initialize the operating system installer. Special keys can be typed as
  well, and are covered in the section below on the boot command. If this
  is not specified, it is assumed the installer will start itself.

<!-- End of code generated from the comments of the BootConfig struct in bootcommand/config.go; -->


### Wait Configuration

**Optional**:

<!-- Code generated from the comments of the WaitIpConfig struct in builder/vsphere/common/step_wait_for_ip.go; DO NOT EDIT MANUALLY -->

- `ip_wait_timeout` (duration string | ex: "1h5m2s") - Amount of time to wait for VM's IP, similar to 'ssh_timeout'.
  Defaults to `30m` (30 minutes). Refer to the Golang
  [ParseDuration](https://golang.org/pkg/time/#ParseDuration)
  documentation for full details.

- `ip_settle_timeout` (duration string | ex: "1h5m2s") - Amount of time to wait for VM's IP to settle down, sometimes VM may
  report incorrect IP initially, then it is recommended to set that
  parameter to apx. 2 minutes. Examples `45s` and `10m`.
  Defaults to `5s` (5 seconds). Refer to the Golang
  [ParseDuration](https://golang.org/pkg/time/#ParseDuration)
  documentation for full details.

- `ip_wait_address` (\*string) - Set this to a CIDR address to cause the service to wait for an address that is contained in
  this network range. Defaults to `0.0.0.0/0` for any IPv4 address. Examples include:
  
  * empty string ("") - remove all filters
  * `0:0:0:0:0:0:0:0/0` - allow only ipv6 addresses
  * `192.168.1.0/24` - only allow ipv4 addresses from 192.168.1.1 to 192.168.1.254

<!-- End of code generated from the comments of the WaitIpConfig struct in builder/vsphere/common/step_wait_for_ip.go; -->


### Communicator Configuration

**Optional**:

##### Common

<!-- Code generated from the comments of the Config struct in communicator/config.go; DO NOT EDIT MANUALLY -->

- `communicator` (string) - Packer currently supports three kinds of communicators:
  
  -   `none` - No communicator will be used. If this is set, most
      provisioners also can't be used.
  
  -   `ssh` - An SSH connection will be established to the machine. This
      is usually the default.
  
  -   `winrm` - A WinRM connection will be established.
  
  In addition to the above, some builders have custom communicators they
  can use. For example, the Docker builder has a "docker" communicator
  that uses `docker exec` and `docker cp` to execute scripts and copy
  files.

- `pause_before_connecting` (duration string | ex: "1h5m2s") - We recommend that you enable SSH or WinRM as the very last step in your
  guest's bootstrap script, but sometimes you may have a race condition
  where you need Packer to wait before attempting to connect to your
  guest.
  
  If you end up in this situation, you can use the template option
  `pause_before_connecting`. By default, there is no pause. For example if
  you set `pause_before_connecting` to `10m` Packer will check whether it
  can connect, as normal. But once a connection attempt is successful, it
  will disconnect and then wait 10 minutes before connecting to the guest
  and beginning provisioning.

<!-- End of code generated from the comments of the Config struct in communicator/config.go; -->


##### SSH

<!-- Code generated from the comments of the SSH struct in communicator/config.go; DO NOT EDIT MANUALLY -->

- `ssh_host` (string) - The address to SSH to. This usually is automatically configured by the
  builder.

- `ssh_port` (int) - The port to connect to SSH. This defaults to `22`.

- `ssh_username` (string) - The username to connect to SSH with. Required if using SSH.

- `ssh_password` (string) - A plaintext password to use to authenticate with SSH.

- `ssh_ciphers` ([]string) - This overrides the value of ciphers supported by default by Golang.
  The default value is [
    "aes128-gcm@openssh.com",
    "chacha20-poly1305@openssh.com",
    "aes128-ctr", "aes192-ctr", "aes256-ctr",
  ]
  
  Valid options for ciphers include:
  "aes128-ctr", "aes192-ctr", "aes256-ctr", "aes128-gcm@openssh.com",
  "chacha20-poly1305@openssh.com",
  "arcfour256", "arcfour128", "arcfour", "aes128-cbc", "3des-cbc",

- `ssh_clear_authorized_keys` (bool) - If true, Packer will attempt to remove its temporary key from
  `~/.ssh/authorized_keys` and `/root/.ssh/authorized_keys`. This is a
  mostly cosmetic option, since Packer will delete the temporary private
  key from the host system regardless of whether this is set to true
  (unless the user has set the `-debug` flag). Defaults to "false";
  currently only works on guests with `sed` installed.

- `ssh_key_exchange_algorithms` ([]string) - If set, Packer will override the value of key exchange (kex) algorithms
  supported by default by Golang. Acceptable values include:
  "curve25519-sha256@libssh.org", "ecdh-sha2-nistp256",
  "ecdh-sha2-nistp384", "ecdh-sha2-nistp521",
  "diffie-hellman-group14-sha1", and "diffie-hellman-group1-sha1".

- `ssh_certificate_file` (string) - Path to user certificate used to authenticate with SSH.
  The `~` can be used in path and will be expanded to the
  home directory of current user.

- `ssh_pty` (bool) - If `true`, a PTY will be requested for the SSH connection. This defaults
  to `false`.

- `ssh_timeout` (duration string | ex: "1h5m2s") - The time to wait for SSH to become available. Packer uses this to
  determine when the machine has booted so this is usually quite long.
  Example value: `10m`.
  This defaults to `5m`, unless `ssh_handshake_attempts` is set.

- `ssh_disable_agent_forwarding` (bool) - If true, SSH agent forwarding will be disabled. Defaults to `false`.

- `ssh_handshake_attempts` (int) - The number of handshakes to attempt with SSH once it can connect.
  This defaults to `10`, unless a `ssh_timeout` is set.

- `ssh_bastion_host` (string) - A bastion host to use for the actual SSH connection.

- `ssh_bastion_port` (int) - The port of the bastion host. Defaults to `22`.

- `ssh_bastion_agent_auth` (bool) - If `true`, the local SSH agent will be used to authenticate with the
  bastion host. Defaults to `false`.

- `ssh_bastion_username` (string) - The username to connect to the bastion host.

- `ssh_bastion_password` (string) - The password to use to authenticate with the bastion host.

- `ssh_bastion_interactive` (bool) - If `true`, the keyboard-interactive used to authenticate with bastion host.

- `ssh_bastion_private_key_file` (string) - Path to a PEM encoded private key file to use to authenticate with the
  bastion host. The `~` can be used in path and will be expanded to the
  home directory of current user.

- `ssh_bastion_certificate_file` (string) - Path to user certificate used to authenticate with bastion host.
  The `~` can be used in path and will be expanded to the
  home directory of current user.

- `ssh_file_transfer_method` (string) - `scp` or `sftp` - How to transfer files, Secure copy (default) or SSH
  File Transfer Protocol.
  
  **NOTE**: Guests using Windows with Win32-OpenSSH v9.1.0.0p1-Beta, scp
  (the default protocol for copying data) returns a a non-zero error code since the MOTW
  cannot be set, which cause any file transfer to fail. As a workaround you can override the transfer protocol
  with SFTP instead `ssh_file_transfer_method = "sftp"`.

- `ssh_proxy_host` (string) - A SOCKS proxy host to use for SSH connection

- `ssh_proxy_port` (int) - A port of the SOCKS proxy. Defaults to `1080`.

- `ssh_proxy_username` (string) - The optional username to authenticate with the proxy server.

- `ssh_proxy_password` (string) - The optional password to use to authenticate with the proxy server.

- `ssh_keep_alive_interval` (duration string | ex: "1h5m2s") - How often to send "keep alive" messages to the server. Set to a negative
  value (`-1s`) to disable. Example value: `10s`. Defaults to `5s`.

- `ssh_read_write_timeout` (duration string | ex: "1h5m2s") - The amount of time to wait for a remote command to end. This might be
  useful if, for example, packer hangs on a connection after a reboot.
  Example: `5m`. Disabled by default.

- `ssh_remote_tunnels` ([]string) - Remote tunnels forward a port from your local machine to the instance.
  Format: ["REMOTE_PORT:LOCAL_HOST:LOCAL_PORT"]
  Example: "9090:localhost:80" forwards localhost:9090 on your machine to port 80 on the instance.

- `ssh_local_tunnels` ([]string) - Local tunnels forward a port from the instance to your local machine.
  Format: ["LOCAL_PORT:REMOTE_HOST:REMOTE_PORT"]
  Example: "8080:localhost:3000" allows the instance to access your local machine’s port 3000 via localhost:8080.

<!-- End of code generated from the comments of the SSH struct in communicator/config.go; -->


- `ssh_private_key_file` (string) - Path to a PEM encoded private key file to use to authenticate with SSH.
  The `~` can be used in path and will be expanded to the home directory
  of current user.


-> **NOTE:** The builder uses vApp Options to inject SSH public keys to the virtual machine. The `temporary_key_pair_name`
will only work if `vapp.properties` includes `public-keys`. If using `ssh_private_key_file`,
provide the public key using the `configuration_parameters` or [vApp Options Configuration](#vapp-options-configuration) whenever the `guestinfo.userdata` is available.

##### Windows Remote Management (WinRM)

<!-- Code generated from the comments of the WinRM struct in communicator/config.go; DO NOT EDIT MANUALLY -->

- `winrm_username` (string) - The username to use to connect to WinRM.

- `winrm_password` (string) - The password to use to connect to WinRM.

- `winrm_host` (string) - The address for WinRM to connect to.
  
  NOTE: If using an Amazon EBS builder, you can specify the interface
  WinRM connects to via
  [`ssh_interface`](/packer/integrations/hashicorp/amazon/latest/components/builder/ebs#ssh_interface)

- `winrm_no_proxy` (bool) - Setting this to `true` adds the remote
  `host:port` to the `NO_PROXY` environment variable. This has the effect of
  bypassing any configured proxies when connecting to the remote host.
  Default to `false`.

- `winrm_port` (int) - The WinRM port to connect to. This defaults to `5985` for plain
  unencrypted connection and `5986` for SSL when `winrm_use_ssl` is set to
  true.

- `winrm_timeout` (duration string | ex: "1h5m2s") - The amount of time to wait for WinRM to become available. This defaults
  to `30m` since setting up a Windows machine generally takes a long time.

- `winrm_use_ssl` (bool) - If `true`, use HTTPS for WinRM.

- `winrm_insecure` (bool) - If `true`, do not check server certificate chain and host name.

- `winrm_use_ntlm` (boolean) - If `true`, NTLMv2 authentication (with session security) will be used
  for WinRM, rather than default (basic authentication), removing the
  requirement for basic authentication to be enabled within the target
  guest. Further reading for remote connection authentication can be found
  [here](https://msdn.microsoft.com/en-us/library/aa384295(v=vs.85).aspx).

<!-- End of code generated from the comments of the WinRM struct in communicator/config.go; -->


### Shutdown Configuration

**Optional:**

<!-- Code generated from the comments of the ShutdownConfig struct in builder/vsphere/common/step_shutdown.go; DO NOT EDIT MANUALLY -->

- `shutdown_command` (string) - Specify a virtual machine guest shutdown command. This command will be run using
  the `communicator`. Otherwise, the VMware Tools are used to gracefully shut down
  the virtual machine.

- `shutdown_timeout` (duration string | ex: "1h5m2s") - Amount of time to wait for graceful shut down of the virtual machine.
  Defaults to `5m` (5 minutes).
  This will likely need to be modified if the `communicator` is 'none'.

- `disable_shutdown` (bool) - Packer normally halts the virtual machine after all provisioners have
  run when no `shutdown_command` is defined. If this is set to `true`, Packer
  *will not* halt the virtual machine but will assume that you will send the stop
  signal yourself through a `preseed.cfg`, a script or the final provisioner.
  Packer will wait for a default of 5 minutes until the virtual machine is shutdown.
  The timeout can be changed using `shutdown_timeout` option.

<!-- End of code generated from the comments of the ShutdownConfig struct in builder/vsphere/common/step_shutdown.go; -->


## Export Configuration

<!-- Code generated from the comments of the ExportConfig struct in builder/vsphere/common/step_export.go; DO NOT EDIT MANUALLY -->

You can export an image in Open Virtualization Format (OVF) to the Packer
host.

HCL Example:

```hcl

	# ...
	vm_name = "example-ubuntu"
	# ...
	export {
	  force = true
	  output_directory = "./output-artifacts"
	}

```

JSON Example:

```json
...

	"vm_name": "example-ubuntu",

...

	"export": {
	  "force": true,
	  "output_directory": "./output-artifacts"
	},

```

The above configuration would create the following files:

```text
./output-artifacts/example-ubuntu-disk-0.vmdk
./output-artifacts/example-ubuntu.mf
./output-artifacts/example-ubuntu.ovf
```

<!-- End of code generated from the comments of the ExportConfig struct in builder/vsphere/common/step_export.go; -->


**Optional**:

<!-- Code generated from the comments of the ExportConfig struct in builder/vsphere/common/step_export.go; DO NOT EDIT MANUALLY -->

- `name` (string) - The name of the exported image in Open Virtualization Format (OVF).
  
  -> **Note:** The name of the virtual machine with the `.ovf` extension is
  used if this option is not specified.

- `force` (bool) - Forces the export to overwrite existing files. Defaults to `false`.
  If set to `false`, an error is returned if the file(s) already exists.

- `image_files` (bool) - Include additional image files that are  associated with the virtual
  machine. Defaults to `false`. For example, `.nvram` and `.log` files.

- `manifest` (string) - The hash algorithm to use when generating a manifest file. Defaults to
  `sha256`.
  
  The available options for this setting are: 'none', 'sha1', 'sha256', and
  'sha512'.
  
  --> **Tip:** Use `none` to disable the creation of a manifest file.

- `options` ([]string) - Advanced image export options. Available options include:
  * `mac` - MAC address is exported for each Ethernet device.
  * `uuid` - UUID is exported for the virtual machine.
  * `extraconfig` - Extra configuration options are exported for the
    virtual machine.
  * `nodevicesubtypes` - Resource subtypes for CD/DVD drives, floppy
    drives, and SCSI controllers are not exported.
  
  For example, adding the following export configuration option outputs the
  MAC addresses for each Ethernet device in the OVF descriptor:
  
  HCL Example:
  
  ```hcl
  ...
    export {
      options = ["mac"]
    }
  ```
  
  JSON: Example:
  
  ```json
  ...
    "export": {
      "options": ["mac"]
    },
  ```

- `output_format` (string) - The output format for the exported virtual machine image.
  Defaults to `ovf`. Available options include `ovf` and `ova`.
  
  When set to `ova`, the image is first exported using Open Virtualization
  Format (`.ovf`) and then converted to an Open Virtualization Archive
  (`.ova`) using the VMware [Open Virtualization Format Tool](https://developer.broadcom.com/tools/open-virtualization-format-ovf-tool/latest)
  (ovftool). The intermediate files are removed after the conversion.
  
  ~> **Note:** To use the `ova` format option, VMware ovftool must be
  installed on the Packer host and accessible in either the system `PATH`
  or the user's `PATH`.

<!-- End of code generated from the comments of the ExportConfig struct in builder/vsphere/common/step_export.go; -->


### Output Configuration

**Optional**:

<!-- Code generated from the comments of the OutputConfig struct in builder/vsphere/common/output_config.go; DO NOT EDIT MANUALLY -->

- `output_directory` (string) - The directory where artifacts from the build, such as the virtual machine
  files and disks, will be output to. The path to the directory may be
  relative or absolute. If relative, the path is relative to the working
  directory Packer is run from. This directory must not exist or, if
  created, must be empty before running the builder. By default, this is
  "output-<buildName>" where "buildName" is the name of the build.

- `directory_permission` (\*os.FileMode) - The permissions to apply to the "output_directory", and to any parent
  directories that get created for output_directory. By default, this is
  "0750". You should express the permission as quoted string with a
  leading zero such as "0755" in JSON file, because JSON does not support
  octal value. In Unix-like OS, the actual permission may differ from
  this value because of umask.

<!-- End of code generated from the comments of the OutputConfig struct in builder/vsphere/common/output_config.go; -->


### Content Library Configuration

<!-- Code generated from the comments of the ContentLibraryDestinationConfig struct in builder/vsphere/common/step_import_to_content_library.go; DO NOT EDIT MANUALLY -->

Create a content library item in a content library whose content is a VM
template or an OVF template created from the virtual machine image after
the build is complete.

The template is stored in an existing or newly created library item.

<!-- End of code generated from the comments of the ContentLibraryDestinationConfig struct in builder/vsphere/common/step_import_to_content_library.go; -->


**Optional**:

<!-- Code generated from the comments of the ContentLibraryDestinationConfig struct in builder/vsphere/common/step_import_to_content_library.go; DO NOT EDIT MANUALLY -->

- `library` (string) - The name of the content library in which the new content library item
  containing the template will be created or updated. The content library
  must be of type Local to allow deploying virtual machines.

- `name` (string) - The name of the content library item that will be created or updated.
  
  For VM templates, the default is [vm_name](#vm_name) + timestamp when not set.
  VM templates are always imported as new library items. If an item with the
  specified name already exists, the import will fail unless [overwrite](#overwrite)
  is set to `true`.
  
  For OVF templates, the name defaults to [vm_name](#vm_name) when not set.
  If an item with the same name already exists, it will be updated with the
  new OVF template, otherwise a new item will be created.
  
  ~> **Note:** VM templates cannot update existing content library items in-place
  like OVF templates. When [overwrite](#overwrite) is enabled, the existing item
  will be deleted before creating the new one.

- `description` (string) - A description for the content library item that will be created.
  Defaults to "Packer imported [vm_name](#vm_name) VM template".

- `cluster` (string) - The cluster where the VM template will be placed.
  If `cluster` and `resource_pool` are both specified, `resource_pool` must
  belong to cluster. If `cluster` and `host` are both specified, the ESX
  host must be a member of the cluster. This option is not used when
  importing OVF templates. Defaults to [`cluster`](#cluster).

- `folder` (string) - The virtual machine folder where the VM template will be placed.
  This option is not used when importing OVF templates. Defaults to
  the same folder as the source virtual machine.

- `host` (string) - The ESX host where the virtual machine template will be placed.
  If `host` and `resource_pool` are both specified, `resource_pool` must
  belong to host. If `host` and `cluster` are both specified, `host` must
  be a member of the cluster. This option is not used when importing OVF
  templates. Defaults to [`host`](#host).

- `resource_pool` (string) - The resource pool where the virtual machine template will be placed.
  Defaults to [`resource_pool`](#resource_pool). If [`resource_pool`](#resource_pool)
  is unset, the system will attempt to choose a suitable resource pool
  for the VM template.

- `datastore` (string) - The datastore for the virtual machine template's configuration and log
  files. This option is not used when importing OVF templates.
  Defaults to the storage backing associated with the content library.

- `destroy` (bool) - Destroy the virtual machine after the import to the content library.
  Defaults to `false`.

- `ovf` (bool) - Import an OVF template to the content library item. Defaults to `false`.

- `skip_import` (bool) - Skip the import to the content library item. Useful during a build test
  stage. Defaults to `false`.

- `ovf_flags` ([]string) - Flags to use for OVF package creation. The supported flags can be
  obtained using ExportFlag.list. If unset, no flags will be used.
  Known values: `EXTRA_CONFIG`, `PRESERVE_MAC`.

- `overwrite` (bool) - Overwrite the content library item if it already exists. This only applies
  to VM templates. For OVF templates, existing items are always updated.
  When enabled for VM templates, the existing item will be deleted before
  creating the new one. Defaults to `false`.

<!-- End of code generated from the comments of the ContentLibraryDestinationConfig struct in builder/vsphere/common/step_import_to_content_library.go; -->


**VM Template**

HCL Example:

```hcl
source "vsphere-iso" "example" {
  # ... other configuration ...
  content_library_destination {
    library = "Example Content Library"
  }
  # ... other configuration ...
}
```

JSON Example:

```json
  "content_library_destination" : {
    "library": "Example Content Library"
    }
```

**OVF Template**

HCL Example:

```hcl
source "vsphere-iso" "example" {
  # ... other configuration ...
  content_library_destination {
    library = "Example Content Library"
    ovf = true
  }
  # ... other configuration ...
}
```

JSON Example:

```json
  "content_library_destination" : {
    "library": "Example Content Library",
    "ovf": true
  }
```

### Extra Configuration

**Optional**:

<!-- Code generated from the comments of the ConfigParamsConfig struct in builder/vsphere/common/step_config_params.go; DO NOT EDIT MANUALLY -->

- `configuration_parameters` (map[string]string) - A map of key-value pairs to send to the [`extraConfig`](https://dp-downloads.broadcom.com/api-content/apis/API_VWSA_001/8.0U3/html/ReferenceGuides/vim.vm.ConfigSpec.html#extraConfig).
  in the vSphere API's `VirtualMachineConfigSpec`.
  
  HCL Example:
  
  ```hcl
    configuration_parameters = {
      "disk.EnableUUID" = "TRUE"
      "svga.autodetect" = "TRUE"
      "log.keepOld"     = "15"
    }
  ```
  
  JSON Example:
  
  ```json
    "configuration_parameters": {
      "disk.EnableUUID": "TRUE",
      "svga.autodetect": "TRUE",
      "log.keepOld": "15"
    }
  ```
  
  ~> **Note:** Configuration keys that would conflict with parameters that
  are explicitly configurable through other fields in the `ConfigSpec` object
  are silently ignored. Refer to the [`VirtualMachineConfigSpec`](https://dp-downloads.broadcom.com/api-content/apis/API_VWSA_001/8.0U3/html/ReferenceGuides/vim.vm.ConfigSpec.html)
  in the vSphere API documentation.

- `tools_sync_time` (\*bool) - Enable or disable time synchronization between the guest operating system and the
  ESX host at startup and after VM operations that may introduce time drift (such
  as resume from suspend, vMotion, or snapshot restore). If set to `true`, time
  synchronization is explicitly enabled. If set to `false`, time synchronization is
  explicitly disabled. If omitted, the builder does not modify the virtual
  machine's time synchronization settings:
    - `vsphere-iso` builder uses the vSphere default for new virtual machines
       (`true`).
    - `vsphere-clone` builder inherits the setting from the source virtual machine.

- `tools_sync_time_periodically` (\*bool) - Enable or disable periodic time synchronization between the guest operating
  system and the ESX host. Use this setting only if the guest operating system does
  not have native time synchronization.
    - `vsphere-iso` builder uses the vSphere default for new virtual machines
       (`false`).
    - `vsphere-clone` builder inherits the setting from the source virtual machine.

- `tools_upgrade_policy` (bool) - Automatically check for and upgrade VMware Tools after a virtual machine
  power cycle. Defaults to `false`.

<!-- End of code generated from the comments of the ConfigParamsConfig struct in builder/vsphere/common/step_config_params.go; -->


### Tags Configuration

**Optional**:

Tags provide a flexible metadata system that allows you to attach key-value information to virtual machines
and templates.

This plugin supports the following configuration formats:

<!-- Code generated from the comments of the TagsConfig struct in builder/vsphere/common/tags_config.go; DO NOT EDIT MANUALLY -->

- `tags` ([]string) - List of tag IDs to attach.

- `tag` ([]TagConfig) - List of tag configuration blocks.

<!-- End of code generated from the comments of the TagsConfig struct in builder/vsphere/common/tags_config.go; -->


Tags consist of the following parts::

<!-- Code generated from the comments of the TagConfig struct in builder/vsphere/common/tags_config.go; DO NOT EDIT MANUALLY -->

- `category` (string) - The tag category name. Mutually exclusive with `id`.

- `name` (string) - The tag name within the category. Mutually exclusive with `id`.

- `id` (string) - The tag ID (URN). Mutually exclusive with `category` and `name`.

<!-- End of code generated from the comments of the TagConfig struct in builder/vsphere/common/tags_config.go; -->


Both formats can be used together, and all tags will be merged and applied to the virtual machine.

~> **Important:** When using `tag` blocks with `category` and `name`, the tag `category` must already exist
in vSphere and be associable with virtual machines. The plugin will create tags within existing
categories if they do not exist and the account context used to run the build has the appropriate
privileges.

HCL Example:

```hcl
source "vsphere-iso" "example" {
  # ... other configuration ...
  tags = [
    "urn:vmomi:InventoryServiceTag:abcdefgh-1234-5678-90ab-cdef12345678:GLOBAL",
    "urn:vmomi:InventoryServiceTag:bcdefghj-2345-6789-01bc-def123456789:GLOBAL"
  ]
  tag {
    id = "urn:vmomi:InventoryServiceTag:cdef1234-5678-90ab-cdef-1234567890ab:GLOBAL"
  }
  tag {
    category = "os-distribution"
    name     = "ubuntu"
  }
  tag {
    category = "os-version"
    name     = "26.04"
  }
  # ... other configuration ...
}
```

JSON Example:

```json
  "tags": [
    "urn:vmomi:InventoryServiceTag:abcdefgh-1234-5678-90ab-cdef12345678:GLOBAL",
    "urn:vmomi:InventoryServiceTag:bcdefghj-2345-6789-01bc-def123456789:GLOBAL"
  ],
  "tag": [
    {
      "id": "urn:vmomi:InventoryServiceTag:cdef1234-5678-90ab-cdef-1234567890ab:GLOBAL"
    },
    {
      "category": "build-date",
      "name": "{{ timestamp }}"
    },
    {
      "category": "packer-version",
      "name": "{{ packer_version }}"
    }
  ]
```

## Working with Clusters and Hosts

### Standalone ESX Hosts

Only use the `host` option. Optionally, specify a `resource_pool`:

HCL Example:

```hcl
source "vsphere-iso" "example" {
  # ... other configuration ...
  host = "esx01.example.com"
  resource_pool = "example_resource_pool"
  # ... other configuration ...
}
```

JSON Example:

```json
  "host": "esx01.example.com",
  "resource_pool": "example_resource_pool",
```

### Clusters with Distributed Resource Scheduler Enabled

Only use the `cluster` option. Optionally, specify a `resource_pool`:

HCL Example:

```hcl
source "vsphere-iso" "example" {
  # ... other configuration ...
  cluster = "cluster-01"
  resource_pool = "example_resource_pool"
  # ... other configuration ...
}
```

JSON Example:

```json
  "cluster": "cluster-01",
  "resource_pool": "example_resource_pool",
```

### Clusters without Distributed Resource Scheduler Enabled

Use the `cluster` and `host`parameters:

HCL Example:

```hcl
source "vsphere-iso" "example" {
  # ... other configuration ...
  cluster = "cluster-01"
  host = "esx01.example.com"
  # ... other configuration ...
}
```

JSON Example:

```json
  "cluster": "cluster-01",
  "host": "esx01.example.com",
```

## Privileges

It is recommended to create a custom vSphere role with the required privileges to integrate Packer
with vSphere. Accounts or groups can be added to the role to ensure that Packer has least privilege
access to the infrastructure.

For example, a named service account (_e.g._ `svc-packer-vsphere@example.com`).

Clone the default **Read-Only** vSphere role and add the following privileges:

| Category                 | Privilege                                           | Reference                                          |
| ------------------------ | --------------------------------------------------- | -------------------------------------------------- |
| Content Library          | Add library item                                    | `ContentLibrary.AddLibraryItem`                    |
| ...                      | Update library Item                                 | `ContentLibrary.UpdateLibraryItem`                 |
| Cryptographic Operations | Direct access                                       | `Cryptographer.Access`                             |
| ...                      | Encrypt                                             | `Cryptographer.Encrypt`                            |
| Datastore                | Allocate space                                      | `Datastore.AllocateSpace`                          |
| ...                      | Browse datastore                                    | `Datastore.Browse`                                 |
| ...                      | Low level file operations                           | `Datastore.FileManagement`                         |
| Network                  | Assign network                                      | `Network.Assign`                                   |
| Resource                 | Assign virtual machine to resource pool             | `Resource.AssignVMToPool`                          |
| vApp                     | Export                                              | `vApp.Export`                                      |
| Virtual Machine          | Configuration > Add new disk                        | `VirtualMachine.Config.AddNewDisk`                 |
| ...                      | Configuration > Add or remove device                | `VirtualMachine.Config.AddRemoveDevice`            |
| ...                      | Configuration > Advanced configuration              | `VirtualMachine.Config.AdvancedConfig`             |
| ...                      | Configuration > Change CPU count                    | `VirtualMachine.Config.CPUCount`                   |
| ...                      | Configuration > Change memory                       | `VirtualMachine.Config.Memory`                     |
| ...                      | Configuration > Change settings                     | `VirtualMachine.Config.Settings`                   |
| ...                      | Configuration > Change Resource                     | `VirtualMachine.Config.Resource`                   |
| ...                      | Configuration > Set annotation                      | `VirtualMachine.Config.Annotation`                 |
| ...                      | Edit Inventory > Create from existing               | `VirtualMachine.Inventory.CreateFromExisting`      |
| ...                      | Edit Inventory > Create new                         | `VirtualMachine.Inventory.Create`                  |
| ...                      | Edit Inventory > Remove                             | `VirtualMachine.Inventory.Delete`                  |
| ...                      | Interaction > Configure CD media                    | `VirtualMachine.Interact.SetCDMedia`               |
| ...                      | Interaction > Configure floppy media                | `VirtualMachine.Interact.SetFloppyMedia`           |
| ...                      | Interaction > Connect devices                       | `VirtualMachine.Interact.DeviceConnection`         |
| ...                      | Interaction > Inject USB HID scan codes             | `VirtualMachine.Interact.PutUsbScanCodes`          |
| ...                      | Interaction > Power off                             | `VirtualMachine.Interact.PowerOff`                 |
| ...                      | Interaction > Power on                              | `VirtualMachine.Interact.PowerOn`                  |
| ...                      | Provisioning > Create template from virtual machine | `VirtualMachine.Provisioning.CreateTemplateFromVM` |
| ...                      | Provisioning > Mark as template                     | `VirtualMachine.Provisioning.MarkAsTemplate`       |
| ...                      | Provisioning > Mark as virtual machine              | `VirtualMachine.Provisioning.MarkAsVM`             |
| ...                      | Snapshot Management > Create snapshot               | `VirtualMachine.State.CreateSnapshot`              |

Global permissions **[are required](https://techdocs.broadcom.com/us/en/vmware-cis/vsphere/vsphere/8-0/vsphere-security-8-0/vsphere-permissions-and-user-management-tasks/understanding-authorization-in-vsphere.html)** for the content library based on the hierarchical inheritance of permissions. Once the custom vSphere role is created, assign **Global Permissions** in vSphere to the accounts or groups used for the Packer to vSphere integration, if using the content library.

For example:

1. Log in to the vCenter at _https://<vcenter_fqdn>/ui_ as `administrator@vsphere.local`.
2. Select **Menu** > **Administration**.
3. In the left pane, select **Access control** > **Global permissions** and click the
   **Add permissions** icon.
4. In the **Add permissions** dialog box, enter the service account
   (_e.g._ `svc-packer-vsphere@example.com`), select the custom role
   (_e.g._ Packer to vSphere Integration Role) and the **Propagate to children** check box,
   and click **OK**.

In an environment with many vCenter instances, such as management and workload, in enhanced
linked-mode, you may wish to further reduce the scope of access across the vSphere infrastructure if
you do not want Packer to have access to the management vCenter instance, but only allow
access to workload vCenter instances.

For example:

1. From the **Hosts and clusters** inventory, select management vCenter to restrict scope,
   and click the **Permissions** tab.
2. Select the service account with the custom role assigned and click the **Change role** icon.
3. In the **Change role** dialog box, from the **Role** drop-down menu, select **No Access**, select
   the **Propagate to children** check box, and click **OK**.
