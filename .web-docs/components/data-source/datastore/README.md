Type: `vsphere-datastore`
Artifact BuilderId: `vsphere.datastore`

This data source retrieves information about existing datastores from vSphere
and returns the name, managed object ID, and summary capacity fields for a
datastore that matches all specified filters. The result can be used with the
builders to set `datastore`.

## Configuration Reference

### Filters Configuration

**Optional:**

<!-- Code generated from the comments of the Config struct in datasource/datastore/data.go; DO NOT EDIT MANUALLY -->

- `name` (string) - Basic filter with glob support (e.g. `w01-cl01-vsan` or `*-vsan`).
  Defaults to `*`. Using stricter globs does not reduce execution time
  because the vSphere API returns the full inventory, but can improve
  readability over regular expressions.

- `name_regex` (string) - Extended name filter with regular expression support
  (e.g. `^w01-cl[0-9]+-vsan$`). Default is empty. The match is checked
  by substring. Use `^` and `$` to define a full string. The expression
  must use [Go Regex Syntax](https://pkg.go.dev/regexp/syntax).

- `host` (string) - Filter to return only datastores mounted on the specified ESX host.

- `cluster` (string) - Filter to return only datastores available to the specified compute
  cluster.

- `tag` ([]Tag) - Filter to return only datastores that have all specified tags attached.

- `most_free_space` (bool) - When more than one datastore matches the filters, select the datastore
  with the most free space (`summary.free`). By default, multiple matches
  result in an error.

<!-- End of code generated from the comments of the Config struct in datasource/datastore/data.go; -->


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

<!-- Code generated from the comments of the DatasourceOutput struct in datasource/datastore/data.go; DO NOT EDIT MANUALLY -->

- `name` (string) - Name of the found datastore.

- `id` (string) - Managed object ID of the found datastore.

- `summary` (Summary) - Capacity fields from the datastore summary.

<!-- End of code generated from the comments of the DatasourceOutput struct in datasource/datastore/data.go; -->


### Summary

<!-- Code generated from the comments of the Summary struct in datasource/datastore/data.go; DO NOT EDIT MANUALLY -->

- `capacity` (int64) - Total capacity of the datastore, in bytes.

- `free` (int64) - Free space available on the datastore, in bytes.

<!-- End of code generated from the comments of the Summary struct in datasource/datastore/data.go; -->


## Example Usage

### Select Datastore with Most Free Space

This example finds the datastore with the most free space among those matching
a name glob and uses it with the vSphere Clone builder.

```hcl
data "vsphere-datastore" "build" {
  vcenter_server  = "vc01.example.com"
  username        = "administrator@vsphere.local"
  password        = "VMware1!"
  datacenter      = "dc-01"
  name            = "*-vsan"
  most_free_space = true
}

source "vsphere-clone" "example" {
  vcenter_server = "vc01.example.com"
  username       = "administrator@vsphere.local"
  password       = "VMware1!"
  host           = "esx01.example.com"
  datastore      = data.vsphere-datastore.build.name
  template       = "alpine-template"
  vm_name        = "alpine-from-ds"
  communicator   = "none"
}

build {
  sources = ["source.vsphere-clone.example"]
}
```

### Filter by Tags

```hcl
data "vsphere-datastore" "build" {
  vcenter_server  = "vc01.example.com"
  username        = "administrator@vsphere.local"
  password        = "VMware1!"
  datacenter      = "dc-01"
  most_free_space = true

  tag {
    category = "datastore-type"
    name     = "vsan"
  }
}

locals {
  datastore_name = data.vsphere-datastore.build.name
  free_bytes     = data.vsphere-datastore.build.summary.free
}
```

### Filter by Host or Compute Cluster

```hcl
data "vsphere-datastore" "on_host" {
  vcenter_server  = "vc01.example.com"
  username        = "administrator@vsphere.local"
  password        = "VMware1!"
  datacenter      = "dc-01"
  host            = "esx01.example.com"
  most_free_space = true
}

data "vsphere-datastore" "on_cluster" {
  vcenter_server  = "vc01.example.com"
  username        = "administrator@vsphere.local"
  password        = "VMware1!"
  datacenter      = "dc-01"
  cluster         = "cluster-01"
  most_free_space = true
}
```
