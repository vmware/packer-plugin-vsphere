// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

//go:generate packer-sdc struct-markdown
//go:generate packer-sdc mapstructure-to-hcl2 -type OutputConfig

package common

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/hashicorp/packer-plugin-sdk/common"
	"github.com/hashicorp/packer-plugin-sdk/template/interpolate"
)

type OutputConfig struct {
	// The directory where artifacts from the build, such as the virtual machine
	// files and disks, will be output to. The path to the directory may be
	// relative or absolute. If relative, the path is relative to the working
	// directory Packer is run from. This directory must not exist or, if
	// created, must be empty prior to running the builder. By default, this is
	// "output-<buildName>" where "buildName" is the name of the build.
	OutputDir string `mapstructure:"output_directory" required:"false"`
	// The permissions to apply to the "output_directory", and to any parent
	// directories that get created for output_directory.  By default, this is
	// "0750". You should express the permission as quoted string with a
	// leading zero such as "0755" in JSON file, because JSON does not support
	// octal value. In Unix-like OS, the actual permission may differ from
	// this value because of umask.
	DirPerm *os.FileMode `mapstructure:"directory_permission" required:"false"`
}

func (c *OutputConfig) Prepare(ctx *interpolate.Context, pc *common.PackerConfig) []error {
	if c.OutputDir == "" {
		c.OutputDir = fmt.Sprintf("output-%s", pc.PackerBuildName)
	}

	if runtime.GOOS != "windows" && (c.DirPerm == nil || *c.DirPerm == 0) {
		perm := os.FileMode(0750)
		c.DirPerm = &perm
	}

	return nil
}

// ListFiles retrieves a list of all non-directory file paths within the configured output directory.
func (c *OutputConfig) ListFiles() ([]string, error) {
	files := make([]string, 0, 10)

	visit := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			files = append(files, path)
		}
		return nil
	}

	return files, filepath.Walk(c.OutputDir, visit)
}
