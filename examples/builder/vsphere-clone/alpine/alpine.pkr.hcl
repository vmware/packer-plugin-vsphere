# © Broadcom. All Rights Reserved.
# The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
# SPDX-License-Identifier: MPL-2.0

packer {
  required_plugins {
    vsphere = {
      version = "~> 1"
      source  = "github.com/vmware/vsphere"
    }
  }
}

variable "vcenter_server" {
  type        = string
  default     = "vc01.example.com"
  description = "The vCenter instance used for managing the ESX host."
}

variable "username" {
  type        = string
  default     = "administrator@vsphere.local"
  sensitive   = true
  description = "The username for authenticating with the vCenter instance."
}

variable "password" {
  type        = string
  default     = "VMw@re1!"
  sensitive   = true
  description = "The password for authenticating with the vCenter instance."
}

variable "insecure_connection" {
  type        = bool
  default     = true
  description = "Set to true to allow insecure connections to the vCenter instance."
}

variable "host" {
  type        = string
  default     = "esx01.example.com"
  description = "The ESX host where the virtual machine will be built."
}

variable "datastore" {
  type        = string
  default     = "local-ssd01-esx01"
  description = "The datastore host where the virtual machine will be built."
}

variable "vm_name_prefix" {
  type        = string
  default     = "alpine"
  description = "Prefix for naming the virtual machine."
}

variable "template" {
  type        = string
  default     = "alpine-template"
  description = "The name of the template to clone for building the virtual machine."
}

variable "communicator" {
  type    = string
  default = "none"
}

locals { timestamp = regex_replace(timestamp(), "[- TZ:]", "") }

source "vsphere-clone" "example" {
  vcenter_server      = var.vcenter_server
  username            = var.username
  password            = var.password
  insecure_connection = var.insecure_connection
  host                = var.host
  datastore           = var.datastore
  vm_name             = "${var.vm_name_prefix}-${local.timestamp}"
  template            = var.template
  communicator        = var.communicator
}

build {
  sources = ["source.vsphere-clone.example"]
}
