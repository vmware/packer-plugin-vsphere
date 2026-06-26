<!--
© Broadcom. All Rights Reserved.
The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
SPDX-License-Identifier: MPL-2.0
-->

<!-- markdownlint-disable first-line-h1 no-inline-html -->

<img src="docs/images/icon-color.svg" alt="VMware vSphere" width="150">

# Packer Plugin for VMware vSphere

[![Latest Release](https://img.shields.io/github/v/tag/vmware/packer-plugin-vsphere?label=latest%20release&style=for-the-badge)][releases-vsphere-plugin] [![License](https://img.shields.io/github/license/vmware/packer-plugin-vsphere.svg?style=for-the-badge)][license]

The Packer Plugin for VMware vSphere is a plugin for creating virtual machine images for
use with [VMware vSphere][docs-vsphere]®.

The plugin includes builders and post-processors for creating virtual machine images,
depending on your desired strategy:

**Builders**

- `vsphere-iso` - This builder creates a virtual machine, installs a guest operating
  system from an ISO, provisions software within the guest operating system, and then
  saves or exports the virtual machine as an image. Use this builder to start by
  creating a new image.

- `vsphere-clone` - This builder imports an existing virtual machine, runs provisioners
  on the virtual machine, and then saves exports the virtual machine as an image. Use
  this builder to start from an existing image as the source.

- `vsphere-supervisor` - This builder deploys and publishes a virtual machine to a
  vSphere Supervisor cluster using VM Service.

**Post-Processors**

- `vsphere` - This post-processor uploads an artifact to a vSphere endpoint. The
  artifact must be a `.vmx`, `.ova`, or `.ovf` file.

- `vsphere-template` - This post-processor uses an artifact from the vSphere
  post-processor. It then marks the virtual machine as a template and moves it to your
  specified path.

## Requirements

- [VMware vSphere][docs-vsphere]

    The plugin supports versions in accordance with the [Broadcom Product Lifecycle][product-lifecycle].

- [Go 1.26.4][golang-install] is required to build the plugin from source.

## Installation

### Using the Releases

#### Automatic Installation

Include the following in your configuration to automatically install the plugin when you
run `packer init`.

```hcl
packer {
  required_version = ">= 1.7.0"
  required_plugins {
    vmware = {
      version = ">= 2.2.0"
      source  = "github.com/vmware/vsphere"
    }
  }
}
```

For more information, please refer to the Packer [documentation][docs-packer-init].

#### Manual Installation

You can install the plugin using the `packer plugins install` command.

Examples:

1. Install the latest version of the plugin:

    ```shell
    packer plugins install github.com/vmware/vsphere
    ```

2. Install a specific version of the plugin:

    ```shell
    packer plugins install github.com/vmware/vsphere@v2.2.0
    ```

### Using the Source

You can build from source by cloning the GitHub repository and running `make build` from
the repository root. After a successful build, the `packer-plugin-vsphere` binary is
created in the root directory.

To install the compiled plugin, please refer to the Packer [documentation][docs-packer-plugin-install].

## Documentation

- Please refer to the plugin [documentation][docs-vsphere-plugin] for more information on
the plugin usage.

## Contributing

Please read the [code of conduct][code-of-conduct] and [contribution guidelines][contributing]
to get started.

## Support

The Packer Plugin for VMware vSphere is supported by the maintainers and the
plugin community.

## License

© Broadcom. All Rights Reserved.</br>
The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.</br>
Licensed under the [Mozilla Public License, version 2.0][license].

[license]: LICENSE
[contributing]: .github/CONTRIBUTING.md
[code-of-conduct]: .github/CODE_OF_CONDUCT.md
[issues]: https://github.com/vmware/packer-plugin-vsphere/issues
[docs-packer-init]: https://developer.hashicorp.com/packer/docs/commands/init
[docs-packer-plugin-install]: https://developer.hashicorp.com/packer/docs/plugins/install-plugins
[docs-vsphere]: https://techdocs.broadcom.com/us/en/vmware-cis/vsphere.html
[docs-vsphere-clone]: https://vmware.github.io/packer-plugin-vsphere/latest/builders/vsphere-clone/
[docs-vsphere-iso]: https://vmware.github.io/packer-plugin-vsphere/latest/builders/vsphere-iso/
[docs-vsphere-supervisor]: https://vmware.github.io/packer-plugin-vsphere/latest/builders/vsphere-supervisor/
[docs-vsphere-plugin]: https://vmware.github.io/packer-plugin-vsphere/latest/
[golang-install]: https://golang.org/doc/install
[packer]: https://www.packer.io
[releases-vsphere-plugin]: https://github.com/vmware/packer-plugin-vsphere/releases
[product-lifecycle]: https://support.broadcom.com/group/ecx/productlifecycle
