// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"log"

	"github.com/hashicorp/packer-plugin-sdk/plugin"
	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/clone"
	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/iso"
	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/supervisor"
	"github.com/vmware/packer-plugin-vsphere/datasource/datastore"
	"github.com/vmware/packer-plugin-vsphere/datasource/virtualmachine"
	"github.com/vmware/packer-plugin-vsphere/post-processor/vsphere"
	vsphereTemplate "github.com/vmware/packer-plugin-vsphere/post-processor/vsphere-template"
	"github.com/vmware/packer-plugin-vsphere/version"
)

func main() {
	pps := plugin.NewSet()
	pps.RegisterBuilder("iso", new(iso.Builder))
	pps.RegisterBuilder("clone", new(clone.Builder))
	pps.RegisterBuilder("supervisor", new(supervisor.Builder))
	pps.RegisterDatasource("virtualmachine", new(virtualmachine.Datasource))
	pps.RegisterDatasource("datastore", new(datastore.Datasource))
	pps.RegisterPostProcessor(plugin.DEFAULT_NAME, new(vsphere.PostProcessor))
	pps.RegisterPostProcessor("template", new(vsphereTemplate.PostProcessor))
	pps.SetVersion(version.PluginVersion)
	err := pps.Run()
	if err != nil {
		log.Fatal(err)
	}
}
