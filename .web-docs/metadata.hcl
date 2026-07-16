# © Broadcom. All Rights Reserved.
# The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
# SPDX-License-Identifier: MPL-2.0

# For full specification on the configuration of this file visit:
# https://github.com/hashicorp/integration-template#metadata-configuration
integration {
  name = "VMware vSphere"
  description = "A plugin for creating virtual machine images for VMware vSphere."
  identifier = "packer/vmware/vsphere"
  flags = ["hcp-ready"]
  component {
    type = "builder"
    name = "vSphere ISO"
    slug = "vsphere-iso"
  }
  component {
    type = "builder"
    name = "vSphere Clone"
    slug = "vsphere-clone"
  }
  component {
    type = "builder"
    name = "vSphere Supervisor"
    slug = "vsphere-supervisor"
  }
  component {
    type = "post-processor"
    name = "vSphere"
    slug = "vsphere"
  }
  component {
    type = "post-processor"
    name = "vSphere Template"
    slug = "vsphere-template"
  }
  component {
    type = "data-source"
    name = "vSphere Virtual Machine"
    slug = "vsphere-virtualmachine"
  }
  component {
    type = "data-source"
    name = "vSphere Datastore"
    slug = "vsphere-datastore"
  }
  component {
    type = "data-source"
    name = "vSphere Datastore Cluster"
    slug = "vsphere-datastore-cluster"
  }
  component {
    type = "data-source"
    name = "vSphere Host"
    slug = "vsphere-host"
  }
  component {
    type = "data-source"
    name = "vSphere Compute Cluster"
    slug = "vsphere-compute-cluster"
  }
  component {
    type = "data-source"
    name = "vSphere Resource Pool"
    slug = "vsphere-resource-pool"
  }
  component {
    type = "data-source"
    name = "vSphere Network"
    slug = "vsphere-network"
  }
  component {
    type = "data-source"
    name = "vSphere Content Library"
    slug = "vsphere-content-library"
  }
  component {
    type = "data-source"
    name = "vSphere Content Library Item"
    slug = "vsphere-content-library-item"
  }
}
