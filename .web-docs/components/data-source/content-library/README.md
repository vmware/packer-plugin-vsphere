Type: `vsphere-content-library`
Artifact BuilderId: `vsphere.content-library`

This data source retrieves information about an existing content library from
vSphere and returns the name and unique identifier for a library that matches
all specified filters.

~> **Note:** When more than one content library matches the filters, the data
source returns an error. Narrow the filters (for example with `name` or `tag`)
until exactly one library matches.

## Configuration Reference

### Filters Configuration

**Optional:**

<!-- Code generated from the comments of the Config struct in datasource/contentlibrary/data.go; DO NOT EDIT MANUALLY -->

- `name` (string) - Basic filter with glob support on the content library name (e.g.
  `lib*` or `lib01`). Defaults to `*`.

- `name_regex` (string) - Extended name filter with regular expression support matched against the
  content library name (e.g. `^lib0[0-9]+$`). Default is empty. The
  match is checked by substring. Use `^` and `$` to define a full string.
  The expression must use [Go Regex Syntax](https://pkg.go.dev/regexp/syntax).

- `tag` ([]Tag) - Filter to return only content libraries that have all specified tags
  attached.

<!-- End of code generated from the comments of the Config struct in datasource/contentlibrary/data.go; -->


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

<!-- Code generated from the comments of the DatasourceOutput struct in datasource/contentlibrary/data.go; DO NOT EDIT MANUALLY -->

- `name` (string) - Name of the found content library.

- `id` (string) - Unique identifier of the found content library.

- `tags` ([]Tag) - Tags attached to the found content library.

<!-- End of code generated from the comments of the DatasourceOutput struct in datasource/contentlibrary/data.go; -->


## Example Usage

### Select Content Library by Name

```hcl
data "vsphere-content-library" "build" {
  vcenter_server = "vc01.example.com"
  username       = "administrator@vsphere.local"
  password       = "VMware1!"
  datacenter     = "dc-01"
  name           = "lib01"
}

locals {
  content_library    = data.vsphere-content-library.build.name
  content_library_id = data.vsphere-content-library.build.id
}
```

### Filter by Tags

```hcl
data "vsphere-content-library" "build" {
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
  content_library = data.vsphere-content-library.build.name
}
```
