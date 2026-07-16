Type: `vsphere-virtualmachine`
Artifact BuilderId: `vsphere.virtualmachine`

This data source retrieves information about existing virtual machines from vSphere
and returns the name of a virtual machine that matches all specified filters. This
virtual machine can be used in the vSphere Clone builder to select a template.

## Configuration Reference

### Filters Configuration

**Optional:**

<!-- Code generated from the comments of the Config struct in datasource/virtualmachine/data.go; DO NOT EDIT MANUALLY -->

- `name` (string) - Basic filter with glob support (e.g. `ubuntu_basic*`). Defaults to `*`.
  Using strict globs will not reduce execution time because vSphere API
  returns the full inventory. But can be used for better readability over
  regular expressions.

- `name_regex` (string) - Extended name filter with regular expressions support
  (e.g. `ubuntu[-_]basic[0-9]*`). Default is empty. The match of the
  regular expression is checked by substring. Use `^` and `$` to define a
  full string. For example, the `^[^_]+$` filter will search names
  without any underscores. The expression must use
  [Go Regex Syntax](https://pkg.go.dev/regexp/syntax).

- `template` (bool) - Filter to return only objects that are virtual machine templates.
  Defaults to `false` and returns all virtual machines.

- `host` (string) - Filter to search virtual machines only on the specified ESX host.

- `tag` ([]Tag) - Filter to return only the virtual machines that have all specified tags
  attached.

- `latest` (bool) - This filter determines how to handle multiple virtual machines that were matched
  with all previous filters. Virtual machine creation time is being used to find
  the latest. By default, multiple matching machines results in an error.

<!-- End of code generated from the comments of the Config struct in datasource/virtualmachine/data.go; -->


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

<!-- Code generated from the comments of the DatasourceOutput struct in datasource/virtualmachine/data.go; DO NOT EDIT MANUALLY -->

- `vm_name` (string) - Name of the found virtual machine.

<!-- End of code generated from the comments of the DatasourceOutput struct in datasource/virtualmachine/data.go; -->


## Example Usage

### Filter by Tags and Get Latest

This example demonstrates how to connect to vSphere cluster and search for the latest
virtual machine that matches the specified tag filters.

```hcl
data "vsphere-virtualmachine" "example" {
  vcenter_server = "vc01.example.com"
  username       = "administrator@vsphere.local"
  password       = "VMware1!"
  datacenter     = "dc-01"
  latest         = true

  tag {
    category = "environment"
    name     = "production"
  }
}

locals {
  vm_name = data.vsphere-virtualmachine.example.vm_name
}

source "null" "example" {
    communicator = "none"
}

build {
  sources = [
    "source.null.example"
  ]

  provisioner "shell-local" {
    inline = [
      "echo vm_name: ${local.vm_name}",
    ]
  }
}
```

### Filter by Name with Glob Pattern

This example shows how to use the `name` filter with glob patterns to find virtual
machines matching a specific naming convention.

```hcl
data "vsphere-virtualmachine" "example" {
  vcenter_server = "vc01.example.com"
  username       = "administrator@vsphere.local"
  password       = "VMware1!"
  datacenter     = "dc-01"
  name           = "linux-debian-*"
  latest         = true
}

locals {
  vm_result = data.vsphere-virtualmachine.example.vm_name
}
```

### Filter by Name with Regular Expression

This example demonstrates using the `name_regex` filter for more complex pattern
matching with regular expressions.

```hcl
data "vsphere-virtualmachine" "example" {
  vcenter_server = "vc01.example.com"
  username       = "administrator@vsphere.local"
  password       = "VMware1!"
  datacenter     = "dc-01"
  name_regex     = "^linux-debian-.+$"
  latest         = true
}

locals {
  vm_result = data.vsphere-virtualmachine.example.vm_name
}
```

### Filter for VM Templates Only

This example shows how to use the `template` filter to search only for virtual
machine templates.

```hcl
data "vsphere-virtualmachine" "example" {
  vcenter_server = "vc01.example.com"
  username       = "administrator@vsphere.local"
  password       = "VMware1!"
  datacenter     = "dc-01"
  name           = "linux-debian-*"
  template       = true
}

locals {
  result_template = data.vsphere-virtualmachine.example.vm_name
}
```

### Filter by ESX Host

This example demonstrates filtering virtual machines by a specific ESX host,
combined with other filters.

```hcl
data "vsphere-virtualmachine" "example" {
  vcenter_server = "vc01.example.com"
  username       = "administrator@vsphere.local"
  password       = "VMware1!"
  datacenter     = "dc-01"
  host           = "esx01.example.com"
  name           = "linux-debian-*"
  latest         = true
}

locals {
  vm_result = data.vsphere-virtualmachine.example.vm_name
}
```
