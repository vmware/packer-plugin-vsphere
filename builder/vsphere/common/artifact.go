// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package common

import (
	"fmt"
	"log"
	"os"

	registryimage "github.com/hashicorp/packer-plugin-sdk/packer/registry/image"
	"github.com/vmware/govmomi/object"
	"github.com/vmware/packer-plugin-vsphere/builder/vsphere/driver"
)

const BuilderId = "vmware.vsphere"

type Artifact struct {
	Outconfig            *OutputConfig
	Name                 string
	Location             LocationConfig
	Datacenter           *object.Datacenter
	VM                   *driver.VirtualMachineDriver
	ContentLibraryConfig *ContentLibraryDestinationConfig
	// StateData should store data such as GeneratedData
	// to be shared with post-processors
	StateData map[string]interface{}
}

func (a *Artifact) BuilderId() string {
	return BuilderId
}

func (a *Artifact) Files() []string {
	if a.Outconfig != nil {
		files, _ := a.Outconfig.ListFiles()
		return files
	}
	return []string{}
}

func (a *Artifact) Id() string {
	return a.Name
}

func (a *Artifact) String() string {
	return a.Name
}

func (a *Artifact) State(name string) interface{} {
	if name == registryimage.ArtifactStateURI {
		return a.stateHCPPackerRegistryMetadata()
	}
	return a.StateData[name]
}

// stateHCPPackerRegistryMetadata will write the metadata as an hcpRegistryImage
func (a *Artifact) stateHCPPackerRegistryMetadata() interface{} {
	labels := make(map[string]interface{})

	floppyPath, ok := a.StateData["uploaded_floppy_path"].(string)
	if ok && floppyPath != "" {
		labels["uploaded_floppy_path"] = floppyPath
	}
	metadata, ok := a.StateData["metadata"].(map[string]string)
	if ok {
		for label, data := range metadata {
			labels[label] = data
		}
	}
	if a.Location.Cluster != "" {
		labels["cluster"] = a.Location.Cluster
	}
	if a.Location.Host != "" {
		labels["host"] = a.Location.Host
	}
	if a.ContentLibraryConfig != nil {
		labels["content_library_destination"] = fmt.Sprintf("%s/%s", a.ContentLibraryConfig.Library, a.ContentLibraryConfig.Name)
	}
	// this is where the iso was downloaded from
	sourceURL, ok := a.StateData["SourceImageURL"].(string)
	if ok {
		labels["source_image_url"] = sourceURL
	}
	// This is where the iso was uploaded to on the remote vsphere datastore
	var sourceID string
	isoPath, ok := a.StateData["iso_path"].(string)
	if ok {
		sourceID = isoPath
	}

	// If a clone, the source comes from a different place.
	templatePath, ok := a.StateData["source_template"].(string)
	if ok && templatePath != "" {
		sourceID = templatePath
	}

	ovfURL, ok := a.StateData["source_ovf_url"].(string)
	if ok && ovfURL != "" {
		sourceID = ovfURL
		labels["source_ovf_url"] = ovfURL
	}

	img, _ := registryimage.FromArtifact(a,
		registryimage.WithID(a.Name),
		registryimage.WithRegion(a.Datacenter.Name()),
		registryimage.WithProvider("vsphere"),
		registryimage.WithSourceID(sourceID),
		registryimage.SetLabels(labels),
	)

	return img
}

func (a *Artifact) Destroy() error {
	if a.Outconfig != nil {
		if err := os.RemoveAll(a.Outconfig.OutputDir); err != nil {
			log.Printf("[WARN] Failed to remove output directory: %v", err)
		}
	}
	return a.VM.Destroy()
}
