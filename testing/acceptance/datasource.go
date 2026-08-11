// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package acceptance

import (
	"maps"
	"testing"

	"github.com/vmware/packer-plugin-vsphere/testing/env"
	"github.com/zclconf/go-cty/cty"
)

// Datasource is the common execution interface implemented by plugin data
// sources.
type Datasource interface {
	Configure(...interface{}) error
	Execute() (cty.Value, error)
}

// DatasourceConfig returns a live vCenter connection configuration combined
// with datasource-specific filters.
func DatasourceConfig(acc env.AccConfig, filters map[string]interface{}) map[string]interface{} {
	config := map[string]interface{}{
		"vcenter_server":      acc.VCenterServer,
		"username":            acc.Username,
		"password":            acc.Password,
		"datacenter":          acc.Datacenter,
		"insecure_connection": true,
	}
	maps.Copy(config, filters)
	return config
}

// ExecuteDatasource configures and executes a datasource against the live
// acceptance-test environment.
func ExecuteDatasource(t *testing.T, datasource Datasource, config map[string]interface{}) cty.Value {
	t.Helper()
	if err := datasource.Configure(config); err != nil {
		t.Fatalf("configure datasource: %v", err)
	}
	output, err := datasource.Execute()
	if err != nil {
		t.Fatalf("execute datasource: %v", err)
	}
	return output
}
