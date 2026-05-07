Type: `vsphere`

Artifact BuilderId: `packer.post-processor.vsphere`

This post-processor uploads an artifact to a vSphere endpoint.

The artifact must be a VMX, OVA, or OVF file.

-> **Note:** This post-processor is developed to maintain compatibility with VMware vSphere versions until
their respective End of General Support dates. For detailed information, refer to the
[Broadcom Product Lifecycle](https://support.broadcom.com/group/ecx/productlifecycle).

## Examples

Examples are available in the [examples](https://github.com/vmware/packer-plugin-vsphere/tree/main/examples/)
directory of the GitHub repository.

## Configuration Reference

The following configuration options are available for the post-processor.

**Required:**

<!-- Code generated from the comments of the Config struct in post-processor/vsphere/post-processor.go; DO NOT EDIT MANUALLY -->

- `cluster` (string) - The cluster or ESX host to upload the virtual machine. This can be either the
  name of the vSphere cluster or the fully qualified domain name (FQDN) or IP
  address of the ESX host.

- `datacenter` (string) - The name of the vSphere datacenter object to place the virtual machine.
  This is _not required_ if `resource_pool` is specified.

- `host` (string) - The fully qualified domain name or IP address of the vCenter instance or ESX host.

- `password` (string) - The password to use to authenticate to the vSphere endpoint.

- `username` (string) - The username to use to authenticate to the vSphere endpoint.

<!-- End of code generated from the comments of the Config struct in post-processor/vsphere/post-processor.go; -->


**Optional:**

<!-- Code generated from the comments of the Config struct in post-processor/vsphere/post-processor.go; DO NOT EDIT MANUALLY -->

- `datastore` (string) - The name of the vSphere datastore to place the virtual machine.
  Mutually exclusive with `datastore_cluster`.

- `datastore_cluster` (string) - The name of the vSphere datastore cluster to place the virtual machine.
  When specified, Storage DRS will automatically select the optimal datastore.
  Mutually exclusive with `datastore`.

- `disk_mode` (string) - The disk format of the target virtual machine. One of `thin` or `thick`.

- `esxi_host` (string) - The fully qualified domain name or IP address of the ESX host to upload the
  virtual machine. This is _not required_ if `host` is a vCenter instance.

- `insecure` (bool) - Skip the verification of the server certificate. Defaults to `false`.

- `options` ([]string) - Options to send to `ovftool` when uploading the virtual machine.
  Use `ovftool --help` to list all the options available.

- `overwrite` (bool) - Overwrite existing files.
  If `true`, forces overwrites of existing files. Defaults to `false`.

- `resource_pool` (string) - The name of the resource pool to place the virtual machine.

- `vm_folder` (string) - The name of the virtual machine folder path where the virtual machine will be
  placed.

- `vm_name` (string) - The name of the virtual machine to be created on the vSphere endpoint.

- `vm_network` (string) - The name of the network in which to place the virtual machine.

- `hardware_version` (string) - The maximum virtual hardware version for the deployed virtual machine.
  
  It does not upgrade the virtual hardware version of the source VM. Instead, it
  limits the virtual hardware version of the deployed virtual machine to the
  specified version. If the source virtual machine's hardware version is higher
  than the specified version, the deployed virtual machine's hardware version will
  be downgraded to the specified version.
  
  If the source virtual machine's hardware version is lower than or equal to the
  specified version, the deployed virtual machine's hardware version will be the
  same as the source virtual machine's.
  
  This option is useful when deploying to vCenter instance or an ESX host whose
  version is different from the one used to create the artifact.
  
  Refer to [KB 315655](https://knowledge.broadcom.com/external/article?articleNumber=315655)
  for more information on supported virtual hardware versions.

- `max_retries` (int) - The maximum number of times to retry the upload operation if it fails.
  Defaults to `5`.

<!-- End of code generated from the comments of the Config struct in post-processor/vsphere/post-processor.go; -->


- `keep_input_artifact` (boolean) - Preserve the local virtual machines files, even after importing
  them to the vSphere endpoint. Defaults to `false`.

## Example Usage

The following is an example of the post-processor used in conjunction with the `null` builder to
upload a VMX to a vSphere cluster. You can also use this post-processor with the VMX artifact from a
build.

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
      post-processor "vsphere"{
          vm_name             = "foo"
          host                = "vc01.example.com"
          username            = "administrator@vsphere.local"
          password            = "VMw@re1!"
          datacenter          = "dc-01"
          cluster             = "cluster-01"
          datastore           = "datastore-01"
          vm_network          = "VM Network"
          keep_input_artifact = true
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
        "vm_name": "foo",
        "host": "vc01.example.com",
        "username": "administrator@vsphere.local",
        "password": "VMw@re1!",
        "datacenter": "dc-01",
        "cluster": "cluster-01",
        "datastore": "datastore-01",
        "vm_network": "VM Network",
        "keep_input_artifact": true
      }
    ]
  ]
}
```

## Privileges

The post-processor uses `ovftool` and needs several privileges to be able to run `ovftool`.

Rather than giving Administrator access, you can create a role to give the post-processor the
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

The role must be authorized on the:

- Cluster of the host.
- The destination folder.
- The destination datastore.
- The network to be assigned.
