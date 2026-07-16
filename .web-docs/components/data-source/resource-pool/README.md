Type: `vsphere-resource-pool`
Artifact BuilderId: `vsphere.resource-pool`

This data source retrieves information about existing resource pools from
vSphere and returns the name, managed object ID, and builder-ready nested path
for a pool that matches all specified filters. The result can be used with the
builders and post-processors to set `resource_pool`.

~> **Note:** When more than one resource pool matches the filters, the data
source returns an error. Narrow the filters (for example with `cluster` or
`host`) until exactly one pool matches.

## Configuration Reference

### Filters Configuration

**Optional:**

<!-- Code generated from the comments of the Config struct in datasource/resourcepool/data.go; DO NOT EDIT MANUALLY -->

- `name` (string) - Basic filter with glob support on the resource pool leaf name
  (e.g. `rp-production*` or `rp-development`). Paths containing `/` are
  passed to the inventory finder (e.g. `*/Resources/rp-production`).
  Defaults to `*`.

- `name_regex` (string) - Extended name filter with regular expression support matched against the
  pool name (e.g. `^rp-production[0-9]*$`). Default is empty. The match is
  checked by substring. Use `^` and `$` to define a full string. The
  expression must use [Go Regex Syntax](https://pkg.go.dev/regexp/syntax).

- `cluster` (string) - Filter to return only resource pools under the specified compute cluster.

- `host` (string) - Filter to return only resource pools available to the specified ESX host
  (pools owned by the host's parent compute resource / cluster).

- `tag` ([]Tag) - Filter to return only resource pools that have all specified tags
  attached.

<!-- End of code generated from the comments of the Config struct in datasource/resourcepool/data.go; -->


### Tags Filter Configuration

<!-- Code generated from the comments of the Tag struct in datasource/common/tag.go; DO NOT EDIT MANUALLY -->

Tag identifies a vSphere tag by name and category for datasource filters.
Specify one or more `tag` blocks; every listed tag must be attached.

HCL Example:

```hcl

	tag {
	  category = "environment"
	  name     = "production"
	}

```

<!-- End of code generated from the comments of the Tag struct in datasource/common/tag.go; -->


**Required:**

<!-- Code generated from the comments of the Tag struct in datasource/common/tag.go; DO NOT EDIT MANUALLY -->

- `name` (string) - Name of the tag that must be attached to the object.

- `category` (string) - Name of the tag category that contains the tag.
  
  -> **Note:** Both `name` and `category` must be specified in the `tag`
  filter.

<!-- End of code generated from the comments of the Tag struct in datasource/common/tag.go; -->


### Connection Configuration

**Optional:**

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


## Output

<!-- Code generated from the comments of the DatasourceOutput struct in datasource/resourcepool/data.go; DO NOT EDIT MANUALLY -->

- `name` (string) - Name of the found resource pool.

- `id` (string) - Managed object ID of the found resource pool.

- `path` (string) - Builder-ready nested path under the compute cluster root `Resources` pool
  (e.g. `rp-production` or `rp-parent/rp-child`). Empty for the
  cluster/host root pool.

<!-- End of code generated from the comments of the DatasourceOutput struct in datasource/resourcepool/data.go; -->


## Example Usage

### Select Resource Pool by Name and Cluster

```hcl
data "vsphere-resource-pool" "build" {
  vcenter_server = "vc01.example.com"
  username       = "administrator@vsphere.local"
  password       = "VMware1!"
  datacenter     = "dc-01"
  cluster        = "w01-cl01"
  name           = "rp-packer*"
}

source "vsphere-clone" "example" {
  vcenter_server = "vc01.example.com"
  username       = "administrator@vsphere.local"
  password       = "VMware1!"
  cluster        = "w01-cl01"
  resource_pool  = data.vsphere-resource-pool.build.path
  datastore      = "w01-cl01-vsan01"
  template       = "alpine-template"
  vm_name        = "alpine-from-rp"
  communicator   = "none"
}

build {
  sources = ["source.vsphere-clone.example"]
}
```

### Filter by Tags

```hcl
data "vsphere-resource-pool" "build" {
  vcenter_server = "vc01.example.com"
  username       = "administrator@vsphere.local"
  password       = "VMware1!"
  datacenter     = "dc-01"
  cluster        = "w01-cl01"

  tag {
    category = "environment"
    name     = "production"
  }
}

locals {
  pool_name = data.vsphere-resource-pool.build.name
  pool_path = data.vsphere-resource-pool.build.path
}
```
