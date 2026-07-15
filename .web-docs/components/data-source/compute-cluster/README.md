Type: `vsphere-compute-cluster`
Artifact BuilderId: `vsphere.compute-cluster`

This data source retrieves information about existing compute clusters from
vSphere and returns the name, managed object ID, and root resource pool path
for a cluster that matches all specified filters. The result can be used with
the builders to set `cluster`.

~> **Note:** When more than one compute cluster matches the filters, the data
source returns an error. Narrow the filters until exactly one cluster matches.

## Configuration Reference

### Filters Configuration

**Optional:**

<!-- Code generated from the comments of the Config struct in datasource/computecluster/data.go; DO NOT EDIT MANUALLY -->

- `name` (string) - Basic filter with glob support (e.g. `w01-cl01` or `*-cl*`).
  Defaults to `*`. Using stricter globs does not reduce execution time
  because the vSphere API returns the full inventory, but can improve
  readability over regular expressions.

- `name_regex` (string) - Extended name filter with regular expression support
  (e.g. `^w01-cl[0-9]+$`). Default is empty. The match is checked by
  substring. Use `^` and `$` to define a full string. The expression must
  use [Go Regex Syntax](https://pkg.go.dev/regexp/syntax).

- `tag` ([]Tag) - Filter to return only compute clusters that have all specified tags
  attached.

<!-- End of code generated from the comments of the Config struct in datasource/computecluster/data.go; -->


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

<!-- Code generated from the comments of the DatasourceOutput struct in datasource/computecluster/data.go; DO NOT EDIT MANUALLY -->

- `name` (string) - Name of the found compute cluster.

- `id` (string) - Managed object ID of the found compute cluster.

- `resource_pool` (string) - Inventory path of the cluster root resource pool. Builders treat an empty
  `resource_pool` as this root; an absolute path (starting with `/`) can be
  passed through when an explicit pool is required.

<!-- End of code generated from the comments of the DatasourceOutput struct in datasource/computecluster/data.go; -->


## Example Usage

### Select Compute Cluster by Name

```hcl
data "vsphere-compute-cluster" "build" {
  vcenter_server = "vc01.example.com"
  username       = "administrator@vsphere.local"
  password       = "VMware1!"
  datacenter     = "dc-01"
  name           = "w01-cl01"
}

source "vsphere-clone" "example" {
  vcenter_server = "vc01.example.com"
  username       = "administrator@vsphere.local"
  password       = "VMware1!"
  cluster        = data.vsphere-compute-cluster.build.name
  datastore      = "w01-cl01-vsan01"
  template       = "alpine-template"
  vm_name        = "alpine-from-cluster"
  communicator   = "none"
}

build {
  sources = ["source.vsphere-clone.example"]
}
```

### Filter by Tags

```hcl
data "vsphere-compute-cluster" "build" {
  vcenter_server = "vc01.example.com"
  username       = "administrator@vsphere.local"
  password       = "VMware1!"
  datacenter     = "dc-01"

  tag {
    category = "environment"
    name     = "production"
  }
}

locals {
  cluster_name  = data.vsphere-compute-cluster.build.name
  resource_pool = data.vsphere-compute-cluster.build.resource_pool
}
```
