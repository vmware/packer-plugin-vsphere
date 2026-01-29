<!-- markdownlint-disable first-line-h1 no-inline-html -->

The Packer Plugin for VMware vSphere is a plugin for creating virtual machine images for use with
VMware vSphere®.

### Installation

To install this plugin add this code into your Packer configuration and run
[packer init](/packer/docs/commands/init)

```hcl
packer {
  required_plugins {
    vsphere = {
      version = "~> 1"
      source  = "github.com/vmware/vsphere"
    }
  }
}
```

Alternatively, you can use `packer plugins install` to manage installation of this plugin.

```sh
packer plugins install github.com/vmware/vsphere
```

### Components

The plugin includes builders and post-processors for creating virtual machine images, depending on
your desired strategy:

#### Builders

- [vsphere-iso](/packer/integrations/hashicorp/vsphere/latest/components/builder/vsphere-iso) - 
  This builder starts from an ISO file and uses the vSphere API to build a virtual machine image on
  an ESX host.

- [vsphere-clone](/packer/integrations/hashicorp/vsphere/latest/components/builder/vsphere-clone) -
  This builder clones a virtual machine from an existing template using the uses the vSphere API and
  then modifies and saves it as a new template.

- [vsphere-supervisor](/packer/integrations/hashicorp/vsphere/latest/components/builder/vsphere-supervisor) -
  This builder deploys and publishes new virtual machine to a vSphere Supervisor cluster using VM
  Service.

#### Post-Processors

- [vsphere](/packer/integrations/hashicorp/vsphere/latest/components/post-processor/vsphere) -
  This post-processor uploads an artifact to a vSphere endpoint. The artifact must be a VMX, OVA,
  or OVF file.

- [vsphere-template](/packer/integrations/hashicorp/vsphere/latest/components/post-processor/vsphere-template) - 
  This post-processor uses an artifact from the vSphere post-processor. It then marks the virtual
  machine as a template and moves it to your specified path.
