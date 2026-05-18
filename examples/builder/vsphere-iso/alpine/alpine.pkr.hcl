# © Broadcom. All Rights Reserved.
# The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
# SPDX-License-Identifier: MPL-2.0

// Minimum Version: Alpine Linux 3.23.4
// alpine-standard-<version>-x86_64.iso

packer {
  required_plugins {
    vsphere = {
      version = "~> 1"
      source  = "github.com/vmware/vsphere"
    }
  }
}

variable "alpine_version" {
  type        = string
  default     = "3.23.4"
  description = "The version of Alpine Linux to be used."
}

variable "vm_name_prefix" {
  type        = string
  default     = "alpine"
  description = "Prefix for naming the virtual machine."
}

variable "guest_os_type" {
  type        = string
  default     = "other6xLinux64Guest"
  description = "The type of guest OS to configure for the virtual machine."
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

variable "vcenter_server" {
  type        = string
  default     = "vc01.example.com"
  description = "The vCenter instance used for managing the ESX host."
}

variable "host" {
  type        = string
  default     = "esx01.example.com"
  description = "The ESX host where the virtual machine will be built."
}

variable "insecure_connection" {
  type        = bool
  default     = true
  description = "Set to true to allow insecure connections to the vCenter instance."
}

variable "vm_version" {
  type        = string
  default     = "21"
  description = "The virtual hardware version to use for the virtual machine."
}

variable "root_password" {
  type        = string
  default     = "VMw@re1!"
  sensitive   = true
  description = "The root password for the virtual machine."
}

variable "ssh_username" {
  type        = string
  default     = "root"
  description = "The SSH username for the virtual machine."
}

variable "datastore" {
  type        = string
  default     = "local-ssd01-esx01"
  description = "The ESX datastore where the ISO and virtual machine will be stored."
}

variable "datastore_path" {
  type        = string
  default     = "iso"
  description = "The path within the datastore that contains the ISO file."
}

variable "cpus" {
  type        = number
  default     = 1
  description = "Number of CPUs to assign to the virtual machine."
}

variable "ram" {
  type        = number
  default     = 512
  description = "Amount of RAM (in MB) to assign to the virtual machine."
}

variable "network_name" {
  type        = string
  default     = "VM Network"
  description = "The network name to attach the virtual machine to."
}

variable "disk_size" {
  type        = number
  default     = 1024
  description = "Size of the disk (in MB) to allocate for the virtual machine."
}

variable "disk_controller_type" {
  type        = list(string)
  default     = ["pvscsi"]
  description = "The type of storage controller to use for virtual machine disks."
}

variable "network_card" {
  type        = string
  default     = "vmxnet3"
  description = "The type of network card to use for the virtual machine."
}

locals {
  iso_path  = "[${var.datastore}] ${var.datastore_path}/linux/alpine/3/amd64/alpine-standard-${var.alpine_version}-x86_64.iso"
  timestamp = regex_replace(timestamp(), "[- TZ:]", "")
}

source "vsphere-iso" "example" {
  username            = var.username
  password            = var.password
  vcenter_server      = var.vcenter_server
  host                = var.host
  insecure_connection = var.insecure_connection
  vm_version          = var.vm_version
  vm_name             = "${var.vm_name_prefix}-${local.timestamp}"
  ssh_password        = var.root_password
  ssh_username        = var.ssh_username
  iso_paths           = [local.iso_path]
  floppy_files        = ["${path.root}/answerfile", "${path.root}/setup.sh"]
  guest_os_type       = var.guest_os_type
  CPUs                = var.cpus
  RAM                 = var.ram
  boot_command = [
    "<wait30>",
    "root<enter><wait>",
    "mount -t vfat /dev/fd0 /media/floppy<enter><wait>",
    "setup-alpine -f /media/floppy/answerfile<enter>",
    "<wait10>",
    "${var.root_password}<enter>",
    "${var.root_password}<enter>",
    "<wait5>",
    "y<enter>",
    "<wait20>",
    "reboot<enter>",
    "<wait20>",
    "root<enter>",
    "${var.root_password}<enter><wait>",
    "mount -t vfat /dev/fd0 /media/floppy<enter><wait>",
    "/media/floppy/SETUP.SH<enter>"
  ]
  boot_wait = "15s"
  network_adapters {
    network_card = var.network_card
    network      = var.network_name
  }
  storage {
    disk_size             = var.disk_size
    disk_thin_provisioned = true
  }
  disk_controller_type = var.disk_controller_type
  datastore            = var.datastore
}

build {
  sources = ["source.vsphere-iso.example"]

  provisioner "shell" {
    inline = ["ls /"]
  }
}