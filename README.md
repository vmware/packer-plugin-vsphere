<!--
© Broadcom. All Rights Reserved.
The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
SPDX-License-Identifier: MPL-2.0
-->

<!-- markdownlint-disable first-line-h1 no-inline-html -->

# Packer Plugin for VMware vSphere

The Packer Plugin for VMware vSphere is a plugin for creating virtual machine images for use with
[VMware vSphere][docs-vsphere]®.

The plugin includes builders and post-processors for creating virtual machine images, depending on
your desired strategy:

**Builders**

- `vsphere-iso` - This builder starts from an ISO file and uses the vSphere API to build a virtual
  machine image on an ESXi host.

- `vsphere-clone` -  This builder clones a virtual machine from an existing template using the
  uses the vSphere API and then modifies and saves it as a new template.

- `vsphere-supervisor` - This builder deploys and publishes new virtual machine to a vSphere
  Supervisor cluster using VM Service.

**Post-Processors**

- `vsphere` - This post-processor uploads an artifact to a vSphere endpoint. The artifact must be a
  VMX, OVA, or OVF file.

- `vsphere-template` - This post-processor uses an artifact from the vSphere post-processor. 
  It then marks the virtual machine as a template and moves it to your specified path.

## Requirements

- [VMware vSphere][docs-vsphere]

    The plugin supports versions in accordance with the [Broadcom Product Lifecycle][product-lifecycle].

- [Go 1.23.12][golang-install]

    Required if building the plugin.

## Installation

### Using Pre-built Releases

#### Automatic Installation

Packer v1.7.0 and later supports the `packer init` command which enables the automatic installation
of Packer plugins. For more information, see the [Packer documentation][docs-packer-init].

To install this plugin, copy and paste this code (HCL2) into your Packer configuration and run
`packer init`.

```hcl
packer {
  required_version = ">= 1.7.0"
  required_plugins {
    vsphere = {
      version = ">= 2.1.1"
      source  = "github.com/vmware/vsphere"
    }
  }
}
```

#### Manual Installation

You can download [pre-built binary releases][releases-vsphere-plugin] of the plugin on GitHub. Once
you have downloaded the latest release archive for your target operating system and architecture,
extract the release archive to retrieve the plugin binary file for your platform.

To install the downloaded plugin, please follow the Packer documentation on [installing a plugin][docs-packer-plugin-install].

### Using the Source

If you prefer to build the plugin from sources, clone the GitHub repository locally and run the
command `go build` from the repository root directory. Upon successful compilation, a
`packer-plugin-vsphere` plugin binary file can be found in the root directory.

To install the compiled plugin, please follow the Packer documentation on [installing a plugin][docs-packer-plugin-install].

### Documentation

For more information on how to use the plugin, please refer to the [documentation][docs-vsphere-plugin].

## Contributing

The Packer Plugin for VMware vSphere is the work of many contributors and the project team appreciates your help!

If you discover a bug or would like to suggest an enhancement, submit [an issue][issues].

If you would like to submit a pull request, please read the [contribution guidelines][contributing] to get started. In case of enhancement or feature contribution, we kindly ask you to open an issue to discuss it beforehand.

## Support

The Packer Plugin for VMware vSphere is supported by the maintainers and the plugin community.

## License

© Broadcom. All Rights Reserved.
The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.

The Packer Plugin for VMware vSphere is available under the [Mozilla Public License, version 2.0][license] license.

[license]: LICENSE
[contributing]: .github/CONTRIBUTING.md
[issues]: https://github.com/vmware/packer-plugin-vsphere/issues
[docs-packer-init]: https://developer.hashicorp.com/packer/docs/commands/init
[docs-packer-plugin-install]: https://developer.hashicorp.com/packer/docs/plugins/install-plugins
[docs-vsphere]: https://techdocs.broadcom.com/us/en/vmware-cis/vsphere.html
[docs-vsphere-clone]: https://developer.hashicorp.com/packer/integrations/vmware/vsphere/latest/components/builder/vsphere-clone
[docs-vsphere-iso]: https://developer.hashicorp.com/packer/integrations/vmware/vsphere/latest/components/builder/vsphere-iso
[docs-vsphere-supervisor]: https://developer.hashicorp.com/packer/integrations/vmware/vsphere/latest/components/builder/vsphere-supervisor
[docs-vsphere-plugin]: https://developer.hashicorp.com/packer/integrations/vmware/vsphere
[golang-install]: https://golang.org/doc/install
[packer]: https://www.packer.io
[releases-vsphere-plugin]: https://github.com/vmware/packer-plugin-vsphere/releases
[product-lifecycle]: https://support.broadcom.com/group/ecx/productlifecycle
