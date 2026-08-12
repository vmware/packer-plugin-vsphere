Type: `vsphere-content-library-item`
Artifact BuilderId: `vsphere.content-library-item`

This data source retrieves information about an existing item in a content
library from vSphere and returns the name, unique identifier, library, type,
content library path, and attached tags for an item that matches all specified
filters.

The `path` output is returned in the `<library>/<item>/<file>` form used by the
`iso_paths` option of the `vsphere-iso` builder, so an ISO stored in a content
library can be selected dynamically and attached at build time.

~> **Note:** When more than one item matches the filters, the data source
returns an error unless `latest` is set to `true`, in which case the item with
the most recent last modified time is returned. Narrow the filters (for example
with `name`, `name_regex`, `type`, or `tag`) until exactly one item matches.

## Configuration Reference

### Filters Configuration

**Required:**

<!-- Code generated from the comments of the Config struct in datasource/contentlibraryitem/data.go; DO NOT EDIT MANUALLY -->

- `content_library` (string) - Name of the content library to search for items. This filter is required.

<!-- End of code generated from the comments of the Config struct in datasource/contentlibraryitem/data.go; -->


**Optional:**

<!-- Code generated from the comments of the Config struct in datasource/contentlibraryitem/data.go; DO NOT EDIT MANUALLY -->

- `name` (string) - Basic filter with glob support on the item name (e.g.
  `linux-debian-13*-amd64` or `linux-debian-13`). Defaults to `*`.

- `name_regex` (string) - Extended name filter with regular expression support matched against the
  item name (e.g. `^linux-debian-13\.[0-9]+\.[0-9]+-amd64$`). Default is
  empty. The match is checked by substring. Use `^` and `$` to define a
  full string. The expression must use
  [Go Regex Syntax](https://pkg.go.dev/regexp/syntax).

- `type` (string) - Filter to return only items of the specified type. One of `iso`, `ovf`,
  or `vm-template`. Default is empty and returns items of any type.

- `tag` ([]Tag) - Filter to return only items that have all specified tags attached.

- `latest` (bool) - This filter determines how to handle multiple items that were matched
  with all previous filters. The item last modified time is used to find
  the latest. By default, multiple matching items results in an error.

<!-- End of code generated from the comments of the Config struct in datasource/contentlibraryitem/data.go; -->


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

<!-- Code generated from the comments of the DatasourceOutput struct in datasource/contentlibraryitem/data.go; DO NOT EDIT MANUALLY -->

- `name` (string) - Name of the found content library item.

- `id` (string) - Unique identifier of the found content library item.

- `library` (string) - Name of the content library that contains the item.

- `type` (string) - Type of the found content library item. One of `iso`, `ovf`, or
  `vm-template`.

- `path` (string) - Content library path of the found item.
  
  - For `iso` items, the path is returned in the `<library>/<item>/<file>`
  form used by the builder `iso_paths` option.
  - For `ovf` and `vm-template` items, the path is returned in the
  `<library>/<item>` form.

- `tags` ([]Tag) - Tags attached to the found content library item.

<!-- End of code generated from the comments of the DatasourceOutput struct in datasource/contentlibraryitem/data.go; -->


## Example Usage

### Select an ISO for the `vsphere-iso` Builder

```hcl
data "vsphere-content-library-item" "iso" {
  vcenter_server  = "vc01.example.com"
  username        = "administrator@vsphere.local"
  password        = "VMware1!"
  datacenter      = "dc-01"
  content_library = "lib01"
  name            = "linux-debian-13*-amd64"
  type            = "iso"
  latest          = true
}

source "vsphere-iso" "example" {
  iso_paths = [data.vsphere-content-library-item.iso.path]
  # ...
}
```

### Filter by Tags

```hcl
data "vsphere-content-library-item" "iso" {
  vcenter_server  = "vc01.example.com"
  username        = "administrator@vsphere.local"
  password        = "VMware1!"
  datacenter      = "dc-01"
  content_library = "lib01"
  name            = "linux-debian-13*-amd64"
  type            = "iso"

  tag {
    category = "environment"
    name     = "production"
  }
}

locals {
  content_library_item = data.vsphere-content-library-item.iso.name
}
```

### Propagate Tags to a Builder

Use the `tags` output with a builder `dynamic "tag"` block to inherit tags
from the selected content library item.

```hcl
data "vsphere-content-library-item" "iso" {
  vcenter_server  = "vc01.example.com"
  username        = "administrator@vsphere.local"
  password        = "VMware1!"
  datacenter      = "dc-01"
  content_library = "lib01"
  name            = "linux-debian-13*-amd64"
  type            = "iso"
  latest          = true
}

source "vsphere-iso" "example" {
  vcenter_server = "vc01.example.com"
  username       = "administrator@vsphere.local"
  password       = "VMware1!"
  host           = "esx01.example.com"
  iso_paths      = [data.vsphere-content-library-item.iso.path]

  dynamic "tag" {
    for_each = data.vsphere-content-library-item.iso.tags
    content {
      category = tag.value.category
      name     = tag.value.name
    }
  }

  tag {
    category = "pipeline"
    name     = "downstream"
  }
}
```

### Select an Item by Regular Expression

```hcl
data "vsphere-content-library-item" "iso" {
  vcenter_server  = "vc01.example.com"
  username        = "administrator@vsphere.local"
  password        = "VMware1!"
  datacenter      = "dc-01"
  content_library = "lib01"
  name_regex      = "^linux-debian-13\\.[0-9]+\\.[0-9]+-amd64$"
  type            = "iso"
  latest          = true
}

locals {
  content_library_item = data.vsphere-content-library-item.iso.name
}
```
