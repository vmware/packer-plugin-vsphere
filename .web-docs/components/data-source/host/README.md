Type: `vsphere-host`
Artifact BuilderId: `vsphere.host`

This data source retrieves information about existing ESX hosts from vSphere
and returns the name, managed object ID, parent compute cluster, and summary
memory capacity fields for a host that matches all specified filters. The
result can be used with the builders to set `host`.

## Configuration Reference

### Filters Configuration

**Optional:**

<!-- Code generated from the comments of the Config struct in datasource/host/data.go; DO NOT EDIT MANUALLY -->

- `name` (string) - Basic filter with glob support (e.g. `w01-cl01-esx01` or `*-esx*`).
  Defaults to `*`. Using stricter globs does not reduce execution time
  because the vSphere API returns the full inventory, but can improve
  readability over regular expressions.

- `name_regex` (string) - Extended name filter with regular expression support
  (e.g. `^w01-cl[0-9]+-esx[0-9]+$`). Default is empty. The match is checked
  by substring. Use `^` and `$` to define a full string. The expression
  must use [Go Regex Syntax](https://pkg.go.dev/regexp/syntax).

- `cluster` (string) - Filter to return only hosts that belong to the specified compute cluster.

- `tag` ([]Tag) - Filter to return only hosts that have all specified tags attached.

- `most_free_memory` (bool) - When more than one host matches the filters, select the host with the
  most free memory (`summary.memory_free`). By default, multiple matches
  result in an error.

<!-- End of code generated from the comments of the Config struct in datasource/host/data.go; -->


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

<!-- Code generated from the comments of the DatasourceOutput struct in datasource/host/data.go; DO NOT EDIT MANUALLY -->

- `name` (string) - Name of the found host.

- `id` (string) - Managed object ID of the found host.

- `cluster` (string) - Name of the compute cluster that contains the host, if any.

- `summary` (Summary) - Memory capacity fields from the host summary.

- `tags` ([]Tag) - Tags attached to the found host.

<!-- End of code generated from the comments of the DatasourceOutput struct in datasource/host/data.go; -->


### Summary

<!-- Code generated from the comments of the Summary struct in datasource/host/data.go; DO NOT EDIT MANUALLY -->

- `memory_capacity` (int64) - Total memory capacity of the host, in bytes.

- `memory_free` (int64) - Free memory available on the host, in bytes.

<!-- End of code generated from the comments of the Summary struct in datasource/host/data.go; -->


## Example Usage

### Select Host with Most Free Memory

This example finds the host with the most free memory among those matching a
name glob and uses it with the vSphere Clone builder.

```hcl
data "vsphere-host" "build" {
  vcenter_server   = "vc01.example.com"
  username         = "administrator@vsphere.local"
  password         = "VMware1!"
  datacenter       = "dc-01"
  name             = "*-esx*"
  most_free_memory = true
}

source "vsphere-clone" "example" {
  vcenter_server = "vc01.example.com"
  username       = "administrator@vsphere.local"
  password       = "VMware1!"
  host           = data.vsphere-host.build.name
  datastore      = "w01-cl01-vsan01"
  template       = "linux-debian"
  vm_name        = "linux-debian-from-host"
  communicator   = "none"
}

build {
  sources = ["source.vsphere-clone.example"]
}
```

### Filter by Tags

```hcl
data "vsphere-host" "build" {
  vcenter_server   = "vc01.example.com"
  username         = "administrator@vsphere.local"
  password         = "VMware1!"
  datacenter       = "dc-01"
  most_free_memory = true

  tag {
    category = "environment"
    name     = "production"
  }
}

locals {
  host_name    = data.vsphere-host.build.name
  cluster_name = data.vsphere-host.build.cluster
  free_bytes   = data.vsphere-host.build.summary.memory_free
}
```

### Filter by Compute Cluster

```hcl
data "vsphere-host" "on_cluster" {
  vcenter_server   = "vc01.example.com"
  username         = "administrator@vsphere.local"
  password         = "VMware1!"
  datacenter       = "dc-01"
  cluster          = "cluster-01"
  most_free_memory = true
}
```
