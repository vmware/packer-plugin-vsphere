Type: `vsphere-datastore-cluster`
Artifact BuilderId: `vsphere.datastore-cluster`

This data source retrieves information about existing datastore clusters
from vSphere and returns the name, managed object ID, member datastore names,
and aggregate summary capacity fields for a cluster that matches all specified
filters. The result can be used with the builders to set `datastore_cluster`.

## Configuration Reference

### Filters Configuration

**Optional:**

<!-- Code generated from the comments of the Config struct in datasource/datastorecluster/data.go; DO NOT EDIT MANUALLY -->

- `name` (string) - Basic filter with glob support (e.g. `w01-cl01-dsc01` or `*-dsc01`).
  Defaults to `*`. Using stricter globs does not reduce execution time
  because the vSphere API returns the full inventory, but can improve
  readability over regular expressions.

- `name_regex` (string) - Extended name filter with regular expression support
  (e.g. `^w01-cl[0-9]+-dsc$`). Default is empty. The match is checked
  by substring. Use `^` and `$` to define a full string. The expression
  must use [Go Regex Syntax](https://pkg.go.dev/regexp/syntax).

- `host` (string) - Filter to return only datastore clusters whose member datastores are
  mounted on the specified ESX host.

- `cluster` (string) - Filter to return only datastore clusters whose member datastores are
  available to the specified compute cluster.

- `tag` ([]Tag) - Filter to return only datastore clusters that have all specified tags
  attached.

- `most_free_space` (bool) - When more than one datastore cluster matches the filters, select the
  cluster with the most aggregate free space across member datastores
  (`summary.free`). By default, multiple matches result in an error.

<!-- End of code generated from the comments of the Config struct in datasource/datastorecluster/data.go; -->


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

<!-- Code generated from the comments of the DatasourceOutput struct in datasource/datastorecluster/data.go; DO NOT EDIT MANUALLY -->

- `name` (string) - Name of the found datastore cluster.

- `id` (string) - Managed object ID of the found datastore cluster.

- `datastores` ([]string) - Names of member datastores in the cluster.

- `summary` (Summary) - Aggregate capacity fields across member datastores.

<!-- End of code generated from the comments of the DatasourceOutput struct in datasource/datastorecluster/data.go; -->


### Summary

<!-- Code generated from the comments of the Summary struct in datasource/datastorecluster/data.go; DO NOT EDIT MANUALLY -->

- `capacity` (int64) - Total capacity of member datastores, in bytes.

- `free` (int64) - Aggregate free space across member datastores, in bytes.

<!-- End of code generated from the comments of the Summary struct in datasource/datastorecluster/data.go; -->


## Example Usage

### Select Datastore Cluster with Most Free Space

```hcl
data "vsphere-datastore-cluster" "build" {
  vcenter_server  = "vc01.example.com"
  username        = "administrator@vsphere.local"
  password        = "VMware1!"
  datacenter      = "dc-01"
  name            = "*-dsc"
  most_free_space = true
}

source "vsphere-clone" "example" {
  vcenter_server    = "vc01.example.com"
  username          = "administrator@vsphere.local"
  password          = "VMware1!"
  host              = "esx01.example.com"
  datastore_cluster = data.vsphere-datastore-cluster.build.name
  template          = "alpine-template"
  vm_name           = "alpine-from-dsc"
  communicator      = "none"
}

build {
  sources = ["source.vsphere-clone.example"]
}
```

### Filter by Tags

```hcl
data "vsphere-datastore-cluster" "build" {
  vcenter_server  = "vc01.example.com"
  username        = "administrator@vsphere.local"
  password        = "VMware1!"
  datacenter      = "dc-01"
  most_free_space = true

  tag {
    category = "environment"
    name     = "production"
  }
}

locals {
  cluster_name = data.vsphere-datastore-cluster.build.name
  free_bytes   = data.vsphere-datastore-cluster.build.summary.free
  members      = data.vsphere-datastore-cluster.build.datastores
}
```

### Filter by Host or Compute Cluster

```hcl
data "vsphere-datastore-cluster" "on_host" {
  vcenter_server  = "vc01.example.com"
  username        = "administrator@vsphere.local"
  password        = "VMware1!"
  datacenter      = "dc-01"
  host            = "esx01.example.com"
  most_free_space = true
}

data "vsphere-datastore-cluster" "on_cluster" {
  vcenter_server  = "vc01.example.com"
  username        = "administrator@vsphere.local"
  password        = "VMware1!"
  datacenter      = "dc-01"
  cluster         = "cluster-01"
  most_free_space = true
}
```
