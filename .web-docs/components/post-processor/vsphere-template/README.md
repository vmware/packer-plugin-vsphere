Type: `vsphere-template`

Artifact BuilderId: `packer.post-processor.vsphere`

This post-processor uses an artifact from the `vmware-iso` builder with an ESX host or an artifact
from the [vSphere](/packer/integrations/hashicorp/vsphere/latest/components/post-processor/vsphere) post-processor. It then marks
the virtual machine as a template and moves it to your specified path.

-> **Note:** This post-processor is developed to maintain compatibility with VMware vSphere versions until
their respective End of General Support dates. For detailed information, refer to the
[Broadcom Product Lifecycle](https://support.broadcom.com/group/ecx/productlifecycle).

## Examples

- Examples are available in the [examples](https://github.com/vmware/packer-plugin-vsphere/tree/main/builder/vsphere/examples/)
  directory of the GitHub repository.

## Configuration Reference

The following configuration options are available for the post-processor.

**Required:**

<!-- Code generated from the comments of the Config struct in post-processor/vsphere-template/post-processor.go; DO NOT EDIT MANUALLY -->

- `host` (string) - The fully qualified domain name or IP address of the vSphere endpoint.

- `username` (string) - The username to use to authenticate to the vSphere endpoint.

- `password` (string) - The password to use to authenticate to the vSphere endpoint.

<!-- End of code generated from the comments of the Config struct in post-processor/vsphere-template/post-processor.go; -->


**Optional:**

<!-- Code generated from the comments of the Config struct in post-processor/vsphere-template/post-processor.go; DO NOT EDIT MANUALLY -->

- `insecure` (bool) - Skip the verification of the server certificate. Defaults to `false`.

- `datacenter` (string) - The name of the datacenter to use.
  Required when the vCenter instance has more than one datacenter.

- `template_name` (string) - The name of the template.
  If not specified, the name of the virtual machine will be used.

- `folder` (string) - The name of the virtual machine folder path where the template will be created.

- `snapshot_enable` (bool) - Create a snapshot before marking as a template. Defaults to `false`.

- `snapshot_name` (string) - The name of the snapshot. Required when `snapshot_enable` is `true`.

- `snapshot_description` (string) - A description for the snapshot. Required when `snapshot_enable` is `true`.

- `reregister_vm` (boolean) - Keep the virtual machine registered after marking as a template.

- `override` (bool) - Overwrite existing template. Defaults to `false`.

<!-- End of code generated from the comments of the Config struct in post-processor/vsphere-template/post-processor.go; -->


- `keep_input_artifact` (boolean) - This option is not applicable to `vsphere-template`. For a
  template to function, the original virtual machine from which it was generated cannot be deleted.
  Therefore, the vSphere Template post-processor always preserves the original virtual machine.

  ~> **Note**: If you are getting permission denied errors when trying to mark as a template, but it
  works in the vSphere UI, set this to `false`. Default is `true`.

**Optional:**

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

## Example Usage

An example is shown below, showing only the post-processor configuration:

HCL Example:

```hcl
source "null" "example" {
    communicator = "none"
}

build {
    sources = [
        "source.null.example"
    ]

    post-processors {
      post-processor "vsphere-template"{
        host                = "vc01.example.com"
        insecure            = false
        username            = "administrator@vsphere.local"
        password            = "VMw@re1!"
        datacenter          = "dc-01"
        folder              = "/templates/os/distro"
        tags                = [
          "urn:vmomi:InventoryServiceTag:abcdefgh-1234-5678-90ab-cdef12345678:GLOBAL",
          "urn:vmomi:InventoryServiceTag:bcdefghj-2345-6789-01bc-def123456789:GLOBAL"
        ]
        tag {
          id = "urn:vmomi:InventoryServiceTag:cdef1234-5678-90ab-cdef-1234567890ab:GLOBAL"
        }
        tag {
          category = "build-date"
          name     = "{{ timestamp }}"
        }
        tag {
          category = "packer-version"
          name     = "{{ packer_version }}"
        }
      }
    }
}
```

JSON Example:

```json
{
  "builders": [
    {
      "type": "null",
      "communicator": "none"
    }
  ],
  "post-processors": [
    [
      {
        "type": "vsphere-template",
        "host": "vc01.example.com",
        "insecure": true,
        "username": "administrator@vsphere.local",
        "password": "VMw@re1!",
        "datacenter": "dc-01",
        "folder": "/templates/os/distro",
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
      }
    ]
  ]
}
```

## Using the vSphere Template with Local Builders

Once the [vSphere](/packer/integrations/hashicorp/vsphere/latest/components/post-processor/vsphere) post-processor takes an artifact
from the builder and uploads it to a vSphere endpoint, you may want the virtual machine to be marked
as a template.

HCL Example:

```hcl
build {
    sources = [
        "source.null.example"
    ]

  post-processors {
    post-processor "vsphere" {
      # ...
    }

    post-processor "vsphere-template" {
      # ...
    }
  }
}
```

JSON Example:

```json
{
  "builders": [
    {
      "type": "null",
      "communicator": "none"
    }
  ],
  "post-processors": [
    [
      {
        "type": "vsphere",
         ...
      },
      {
        "type": "vsphere-template",
         ...
      }
    ],
    {
      "type": "...",
      ...
    }
  ]
}
```

In the example above, the result of each builder is passed through the defined sequence of
post-processors starting with the `vsphere` post-processor which will upload the artifact to a
vSphere endpoint. The resulting artifact is then passed on to the `vsphere-template` post-processor
which handles marking a virtual machine as a template.

In JSON, note that the `vsphere` and `vsphere-template` post-processors can be paired together in
their own array.

## Privileges

The post processor needs several privileges to be able to mark the virtual as a template.

Rather than giving full administrator access, you can create a role to give the post-processor the
privileges necessary to run.

Below is an example role that will work. Please note that this is a user-supplied list so there may
be a few extraneous privileges that are not strictly required.

For vSphere, the role needs the following privileges:

- `Datastore.AllocateSpace`
- `Host.Config.AdvancedConfig`
- `Host.Config.NetService`
- `Host.Config.Network`
- `Network.Assign`
- `System.Anonymous`
- `System.Read`
- `System.View`
- `VApp.Import`
- `VirtualMachine.Config.AddNewDisk`
- `VirtualMachine.Config.AdvancedConfig`
- `VirtualMachine.Inventory.Delete`

  and either (if `reregister_vm` is `false`):

  - `VirtualMachine.Provisioning.MarkAsTemplate`

  or (if `reregister_vm` is `true` or unset):

  - `VirtualMachine.Inventory.Register`
  - `VirtualMachine.Inventory.Unregister`

The role must be authorized on the:

- Cluster of the host.
- The destination folder.
- The destination datastore.
- The network to be assigned.

### Additional Privileges for Tags

If using the tags configuration, the following additional privileges are required:

| Category        | Privilege                                           | Reference                                          |
| --------------- | --------------------------------------------------- | -------------------------------------------------- |
| Tagging         | Assign or unassign tag                              | `com.vmware.cis.tagging.TagAssociation.Attach`.    |
| ...             | Create tag                                          | `com.vmware.cis.tagging.Tag.Create`                |
| ...             | Read tag                                            | `com.vmware.cis.tagging.Tag.Read`                  |
| ...             | Read tag category                                   | `com.vmware.cis.tagging.Category.Read`             |

~> **Note:** Tag privileges are part of the vSphere REST API and are managed separately from
traditional vSphere privileges. These privileges must be assigned at the **Global** level to work
correctly with the tagging service.

# Troubleshooting

Some users have reported that vSphere templates created from local vSphere builds get their boot
order reset to CD-ROM only instead of the original boot order defined by the template. If this issue
affects you, the solution is to set `"bios.hddOrder": "scsi0:0"` in your builder's `vmx_data`.
