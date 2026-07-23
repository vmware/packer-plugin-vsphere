# Acceptance Tests

The plugin acceptance tests require an online vCenter instance and a small inventory that matches
the defaults below, or the environment overrides.

Unit tests (`make test / task test`) do **not** require this environment.

## Running Acceptance Tests

```bash
# All ACC Packages
make testacc                                  # All acceptance tests.

task testacc                                  # All acceptance tests.

# Datasources (read-only; no plugin build required)
make testacc-datasource                       # All datasource acceptance tests.
task testacc-datasource                       # All datasource acceptance tests.

# Builders
make testacc-builder                          # All builder acceptance tests.
make testacc-builder-iso                      # All vsphere-iso builder acceptance tests.
make testacc-builder-clone                    # All vsphere-clone builder acceptance tests.

task testacc-builder                          # All builder acceptance tests.
task testacc-builder-iso                      # All vsphere-iso builder acceptance tests.
task testacc-builder-clone                    # All vsphere-clone builder acceptance tests.

# Post-processors
make testacc-post-processor                   # All post-processor acceptance tests.
make testacc-post-processor-vsphere           # Only vsphere post-processor acceptance tests.
make testacc-post-processor-vsphere-template  # Only vsphere post-processor acceptance tests.

task testacc-post-processor                   # All post-processor acceptance tests.
task testacc-post-processor-vsphere           # Only vsphere post-processor acceptance tests.
task testacc-post-processor-vsphere-template  # Only vsphere post-processor acceptance tests.
```

Optional filters:

```bash
# Target an Acceptance Test
make testacc-builder-iso TESTACC_BUILDER_ISO_RUN='TestAccISOBuilder_MatrixA'

task testacc-builder-iso TESTACC_BUILDER_ISO_RUN=TestAccISOBuilder_MatrixA

make testacc-datasource TESTACC_DATASOURCE_RUN='TestAccDatasourceNetwork'

task testacc-datasource TESTACC_DATASOURCE_RUN=TestAccDatasourceNetwork
```

## Environment Variables

| Variable                      | Default                                                            | Purpose                                               |
| ----------------------------- | ------------------------------------------------------------------ | ----------------------------------------------------- |
| `PACKER_ACC`                  | _(unset)_                                                          | Set to `1` to Enable Acceptance Tests                 |
| `VSPHERE_VCENTER_SERVER`      | `vc01.example.com`                                                 | vCenter Hostname or IPv4 Address                      |
| `VSPHERE_USERNAME`            | `administrator@vsphere.local`                                      | vCenter Username                                      |
| `VSPHERE_PASSWORD`            | `VMw@re1!`                                                         | vCenter Password                                      |
| `VSPHERE_DATACENTER`          | `dc01`                                                             | Datacenter (Required, if more than one)               |
| `VSPHERE_HOST`                | `esx01.example.com`                                                | Primary ESX Host                                      |
| `VSPHERE_HOST_SECONDARY`      | `esx01.example.com`                                                | Secondary ESX Host (Optional)                         |
| `VSPHERE_DATASTORE`           | `local-ssd01-esx01`                                                | Datastore                                             |
| `VSPHERE_DATASTORE_CLUSTER`   | `nfs-dsc01`                                                        | Datastore Cluster                                     |
| `VSPHERE_CLUSTER`             | `cl01`                                                             | Compute Cluster                                       |
| `VSPHERE_CLUSTER_DRS`         | `cl01`                                                             | Compute Cluster with DRS Enabled (Optional)           |
| `VSPHERE_FOLDER`              | `acc-test-fd01`                                                    | Virtual Machine Folder                                |
| `VSPHERE_RESOURCE_POOL`       | `acc-test-rp01`                                                    | Resource Pool                                         |
| `VSPHERE_NETWORK`             | `VM Network`                                                       | vSphere Standard or Distrbuted Port Group             |
| `VSPHERE_TEMPLATE`            | `alpine-vm-template`                                               | Inventory VM Template                                 |
| `VSPHERE_ISO_PATH`            | `[iso] iso/linux/alpine/3/amd64/alpine-standard-3.23.4-x86_64.iso` | Datastore ISO path for Alpine ISO                     |
| `VSPHERE_CONTENT_LIBRARY`     | `lib01`                                                            | Content Library Name                                  |
| `VSPHERE_CL_VMTX_ITEM`        | `alpine-vm-template`                                               | VM Template Content Library Item                      |
| `VSPHERE_CL_OVF_ITEM`         | `alpine-ovf-template`                                              | OVF Template Content Library Item                     |
| `VSPHERE_OVF_URL`             | _(unset)_                                                          | HTTPS `.ovf` for Remote OVF Clone Source              |
| `VSPHERE_OVA_URL`             | _(unset)_                                                          | HTTPS `.ova` for Remote OVA Clone Source              |
| `VSPHERE_OVF_USERNAME`        | _(unset)_                                                          | HTTPS Basic Auth for Remote OVF/OVA URLs (Optional)   |
| `VSPHERE_OVF_PASSWORD`        | _(unset)_                                                          | HTTPS Basic Auth for Remove OVF/OVA URLs (Optional)   |
| `VSPHERE_OVF_SKIP_TLS_VERIFY` | _(unset)_                                                          | Set `1`/`true` to Remote OVF/OVA URL TLS Verification |
| `VSPHERE_TAG_CATEGORY`        | `color`                                                            | Tag Category                                          |
| `VSPHERE_TAG_A`               | `blue`                                                             | Tag Name within Tag Category                          |
| `VSPHERE_TAG_B`               | `red`                                                              | Tag Name within Tag Category                          |
| `VSPHERE_NOTES`               | `Built by Acceptance Tests`                                        | Notes                                                 |

Defaults are defined in [`testing/env`](../env/env.go). Load them in tests with
`env.AccFromEnv()` or `acceptance.AccFromEnv()`.
