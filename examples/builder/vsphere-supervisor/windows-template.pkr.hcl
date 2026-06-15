# © Broadcom. All Rights Reserved.
# The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
# SPDX-License-Identifier: MPL-2.0

# VM-Service source VM configs.
variable "image_name" {
  type = string
}
variable "class_name" {
  type = string
}
variable "storage_class" {
  type = string
}
variable "source_name" {
  type    = string
  default = null
}
variable "bootstrap_provider" {
  type    = string
  default = "Sysprep"
}
variable "bootstrap_data_file" {
  type    = string
  default = null
}

# Supervisor cluster configs.
variable "kubeconfig_path" {
  type    = string
  default = null
}
variable "supervisor_namespace" {
  type    = string
  default = null
}

# SSH connection configs.
variable "communicator" {
  type    = string
  default = "ssh"
}
variable "ssh_username" {
  type    = string
}
variable "ssh_password" {
  type      = string
  sensitive = true
}
variable "ssh_bastion_host" {
  type    = string
  default = null
}
variable "ssh_bastion_username" {
  type    = string
  default = null
}
variable "ssh_bastion_password" {
  type      = string
  default   = null
  sensitive = true
}

# Whether to keep the created source VM after the build.
variable "keep_input_artifact" {
  type    = bool
  default = false
}

source "vsphere-supervisor" "vm" {
  kubeconfig_path      = var.kubeconfig_path
  supervisor_namespace = var.supervisor_namespace
  class_name           = var.class_name
  image_name           = var.image_name
  source_name          = var.source_name
  storage_class        = var.storage_class
  bootstrap_provider   = var.bootstrap_provider
  bootstrap_data_file  = var.bootstrap_data_file
  communicator         = var.communicator
  ssh_username         = var.ssh_username
  ssh_password         = var.ssh_password
  ssh_bastion_host     = var.ssh_bastion_host
  ssh_bastion_username = var.ssh_bastion_username
  ssh_bastion_password = var.ssh_bastion_password
  keep_input_artifact  = var.keep_input_artifact
}

build {
  sources = ["source.vsphere-supervisor.vm"]
  provisioner "powershell" {
    inline = [
      "Set-Location $env:TEMP",
      "echo 'Hello from Packer!' | Out-File -FilePath ./hello-packer.txt",
    ]
  }
}
