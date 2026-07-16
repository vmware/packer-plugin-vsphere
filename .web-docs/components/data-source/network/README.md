Type: `vsphere-network`
Artifact BuilderId: `vsphere.network`

This data source retrieves information about existing networks from vSphere
and returns the name, managed object ID, and API managed-object type for a
network that matches all specified filters. The result can be used with the
builders to set `network`.

Looks up standard port groups, distributed port groups, and NSX segments (plus
other opaque networks).

~> **Note:** Distributed virtual switches are not returned.

~> **Note:** When more than one network matches the filters, the data source
returns an error. Narrow the filters (for example with `type`, `cluster`, `host`,
or `tag`) until exactly one network matches.

## Configuration Reference

### Filters Configuration

**Optional:**

<!-- Code generated from the comments of the Config struct in datasource/network/data.go; DO NOT EDIT MANUALLY -->

- `name` (string) - Basic filter with glob support on the network name (e.g. `pg-prod*` or
  `VM Network`). Defaults to `*`.

- `name_regex` (string) - Extended name filter with regular expression support matched against the
  network name (e.g. `^pg-prod[0-9]*$`). Default is empty. The match is
  checked by substring. Use `^` and `$` to define a full string. The
  expression must use [Go Regex Syntax](https://pkg.go.dev/regexp/syntax).

- `cluster` (string) - Filter to return only networks available to the specified compute
  cluster.

- `host` (string) - Filter to return only networks available to the specified ESX host.

- `type` (string) - Optional filter by type. Use one of: `standard-port-group`,
  `distributed-port-group`, or `nsx-segment`. Default is empty (any
  supported type).
  
  -> **Note:** The corresponding API managed-object types are also
  accepted: `standard-port-group` = `Network`, `distributed-port-group` =
  `DistributedVirtualPortgroup`, and `nsx-segment` = `OpaqueNetwork`.
  
  -> **Note:** `nsx-segment` matches all opaque networks, not only NSX.

- `tag` ([]Tag) - Filter to return only networks that have all specified tags attached.

<!-- End of code generated from the comments of the Config struct in datasource/network/data.go; -->


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

<!-- Code generated from the comments of the DatasourceOutput struct in datasource/network/data.go; DO NOT EDIT MANUALLY -->

- `name` (string) - Name of the found network.

- `id` (string) - Managed object ID of the found network.
  
  ~> **Note:** When using the data source result in a builder's `network` option,
  use `data.vsphere-network.build.id` instead of `data.vsphere-network.build.name`
  if the inventory contains more than one network with the same display name.

- `type` (string) - API managed-object type of the found network (`Network`,
  `DistributedVirtualPortgroup`, or `OpaqueNetwork`).

<!-- End of code generated from the comments of the DatasourceOutput struct in datasource/network/data.go; -->


## Example Usage

### Select Distributed Port Group by Name and Type

```hcl
data "vsphere-network" "build" {
  vcenter_server = "vc01.example.com"
  username       = "administrator@vsphere.local"
  password       = "VMware1!"
  datacenter     = "dc-01"
  cluster        = "w01-cl01"
  name           = "pg-prod*"
  type           = "distributed-port-group"
}

source "vsphere-iso" "example" {
  vcenter_server = "vc01.example.com"
  username       = "administrator@vsphere.local"
  password       = "VMware1!"
  cluster        = "w01-cl01"
  datastore      = "w01-cl01-vsan01"
  network        = data.vsphere-network.build.id
  vm_name        = "linux-debian-from-network"
  guest_os_type  = "otherGuest64"
  communicator   = "none"
}

build {
  sources = ["source.vsphere-iso.example"]
}
```

### Filter by Tags

```hcl
data "vsphere-network" "build" {
  vcenter_server = "vc01.example.com"
  username       = "administrator@vsphere.local"
  password       = "VMware1!"
  datacenter     = "dc-01"
  type           = "distributed-port-group"

  tag {
    category = "environment"
    name     = "production"
  }
}

locals {
  network_name = data.vsphere-network.build.name
  network_type = data.vsphere-network.build.type
}
```
