Type: `vsphere-supervisor`

Artifact BuilderId: `vsphere.supervisor`

This builder creates a virtual machine on a vSphere Supervisor cluster using the VM-Operator API.

Refer to [Deploying and Managing Virtual Machines in vSphere Supervisor](https://techdocs.broadcom.com/us/en/vmware-cis/vsphere/vsphere-supervisor/8-0/vsphere-supervisor-services-and-workloads-8-0/deploying-and-managing-virtual-machines-in-vsphere-iaas-control-plane.html)
for more information on the VM Service functionality in vSphere Supervisor.

- It uses a `kubeconfig` file to connect to the vSphere Supervisor cluster.
- It uses the [VM-Operator API](https://vm-operator.readthedocs.io/en/latest/concepts/) to deploy
  and configure the source virtual machine.
- It can use Packer provisioners to customize the virtual machine after establishing a successful
  connection.
- It publishes the customized virtual machine as a new virtual machine image to the designated
  content library in vSphere.

-> **Note:** This builder is developed to maintain compatibility with VMware vSphere versions until
their respective End of General Support dates. For detailed information, refer to the
[Broadcom Product Lifecycle](https://support.broadcom.com/group/ecx/productlifecycle).

## Examples

Examples are available in the [examples](https://github.com/vmware/packer-plugin-vsphere/tree/main/examples/)
directory of the GitHub repository.

HCL Example:

```hcl
source "vsphere-supervisor" "example-vm" {
  image_name = "<Image name of the source VM, e.g. 'ubuntu-impish-21.10-cloudimg'>"
  class_name = "<VM class that describes the virtual hardware settings, e.g. 'best-effort-large'>"
  storage_class = "<Storage class that provides the backing storage for volume, e.g. 'wcplocal-storage-profile'>"
  bootstrap_provider = "<CloudInit, Sysprep, or vAppConfig to customize the guest OS>"
  bootstrap_data_file = "<Path to the file containing the bootstrap data for guest OS customization>"
  publish_location_name = "<target location / content library for the published image, optional, e.g. 'cl-6066c61f7931c5ef9'>"
}

build {
  sources = ["source.vsphere-supervisor.example-vm"]
}
```

HCL Example with image import:

```hcl
source "vsphere-supervisor" "example-vm" {
  import_source_url = "<Remote URL to import image from, optional, e.g. 'https://example.com/example.ovf'>"
  import_source_ssl_certificate = "<SSL certificate of the remote HTTPS server, optional, e.g. '-----BEGIN CERTIFICATE-----xxxxx-----END CERTIFICATE-----'>"
  import_target_location_name = "<Target location / content library for the imported image, optional, e.g. 'cl-6066c61f7931c5ef9'>"
  import_target_image_type = "<Target image type of the imported image, optional, e.g. 'ovf'>"
  import_target_image_name = "<Target image name of the imported image for the source VM, e.g. 'ubuntu-impish-21.10-cloudimg'>"
  class_name = "<VM class that describes the virtual hardware settings, e.g. 'best-effort-large'>"
  storage_class = "<Storage class that provides the backing storage for volume, e.g. 'wcplocal-storage-profile'>"
  bootstrap_provider = "<CloudInit, Sysprep, or vAppConfig to customize the guest OS>"
  bootstrap_data_file = "<Path to the file containing the bootstrap data for guest OS customization>"
  publish_location_name = "<target location / content library for the published image, optional, e.g. 'cl-6066c61f7931c5ef9'>"
}

build {
  sources = ["source.vsphere-supervisor.example-vm"]
}
```

JSON Example:

```json
{
  "builders": [
    {
      "type": "vsphere-supervisor",
      "image_name": "<Image name of the source VM, e.g. 'ubuntu-impish-21.10-cloudimg'>",
      "class_name": "<VM class that describes the virtual hardware settings, e.g. 'best-effort-large'>",
      "storage_class": "<Storage class that provides the backing storage for volume, e.g. 'wcplocal-storage-profile'>",
      "bootstrap_provider": "<CloudInit, Sysprep, or vAppConfig to customize the guest OS>",
      "bootstrap_data_file": "<Path to the file containing the bootstrap data for guest OS customization>",
      "publish_location_name": "<target location / content library for the published image, optional, e.g. 'cl-6066c61f7931c5ef9'>"
    }
  ]
}
```

JSON Example with image import:

```json
{
  "builders": [
    {
      "type": "vsphere-supervisor",
      "import_source_url": "<Remote URL to import image from, optional, e.g. 'https://example.com/example.ovf'>",
      "import_source_ssl_certificate": "<SSL certificate of the remote HTTPS server, optional, e.g. '-----BEGIN CERTIFICATE-----xxxxx-----END CERTIFICATE-----'>",
      "import_target_location_name": "<Target location / content library for the import image, optional, e.g. 'cl-6066c61f7931c5ef9'>",
      "import_target_image_type": "<Target image type of the imported image, optional, e.g. 'ovf'>",
      "import_target_image_name": "<Target image name of the imported image for the source VM, e.g. 'ubuntu-impish-21.10-cloudimg'>",
      "class_name": "<VM class that describes the virtual hardware settings, e.g. 'best-effort-large'>",
      "storage_class": "<Storage class that provides the backing storage for volume, e.g. 'wcplocal-storage-profile'>",
      "bootstrap_provider": "<CloudInit, Sysprep, or vAppConfig to customize the guest OS>",
      "bootstrap_data_file": "<Path to the file containing the bootstrap data for guest OS customization>",
      "publish_location_name": "<target location / content library for the published image, optional, e.g. 'cl-6066c61f7931c5ef9'>"
    }
  ]
}
```

## Configuration Reference

There are various configuration options available for each step in this builder. The _required_
items are listed below as well as the _optional_ configurations.

**Required**:

<!-- Code generated from the comments of the CreateSourceConfig struct in builder/vsphere/supervisor/step_create_source.go; DO NOT EDIT MANUALLY -->

- `class_name` (string) - Name of the VM class that describes virtual hardware settings.

- `storage_class` (string) - Name of the storage class that configures storage-related attributes.

<!-- End of code generated from the comments of the CreateSourceConfig struct in builder/vsphere/supervisor/step_create_source.go; -->


### Supervisor Connection

**Optional**:

<!-- Code generated from the comments of the ConnectSupervisorConfig struct in builder/vsphere/supervisor/step_connect_supervisor.go; DO NOT EDIT MANUALLY -->

- `kubeconfig_path` (string) - The path to kubeconfig file for accessing to the vSphere Supervisor cluster. Defaults to the value of `KUBECONFIG` envvar or `$HOME/.kube/config` if the envvar is not set.

- `supervisor_namespace` (string) - The Supervisor namespace to deploy the source VM. Defaults to the current context's namespace in kubeconfig.

<!-- End of code generated from the comments of the ConnectSupervisorConfig struct in builder/vsphere/supervisor/step_connect_supervisor.go; -->


#### Source VM Image Importing

**Optional:**

<!-- Code generated from the comments of the ImportImageConfig struct in builder/vsphere/supervisor/step_import_image.go; DO NOT EDIT MANUALLY -->

- `import_source_url` (string) - The remote URL where the to-be-imported image is hosted.

- `import_source_ssl_certificate` (string) - The SSL certificate of the remote HTTP server that hosts the to-be-imported image.

- `import_target_location_name` (string) - Name of a writable and import-allowed ContentLibrary resource in the namespace where the image will be imported.

- `import_target_image_type` (string) - The type of imported image.
  Defaults to suffix of the source URL. Available options include `ovf` and `iso`.

- `import_target_image_name` (string) - Name of the imported image.
  Defaults to the file name of the image referenced in the source URL.

- `import_request_name` (string) - The name of the image import request.
  Defaults to `packer-vsphere-supervisor-import-req-<random-suffix>`.

- `watch_import_timeout_sec` (int) - The timeout in seconds to wait for the image to be imported.
  Defaults to `600`.

- `keep_import_request` (bool) - Preserve the import request in the Supervisor cluster after the build finishes.
  Defaults to `false`.

- `clean_imported_image` (bool) - Clean the imported image after the build finishes. If set to `true`, the imported image will be deleted.
  Defaults to `false`.

<!-- End of code generated from the comments of the ImportImageConfig struct in builder/vsphere/supervisor/step_import_image.go; -->


#### Source Virtual Machine Creation

<!-- Code generated from the comments of the CreateSourceConfig struct in builder/vsphere/supervisor/step_create_source.go; DO NOT EDIT MANUALLY -->

- `image_name` (string) - Name of the source virtual machine (VM) image. If it is specified, the image with the name will be used for the
  source VM, otherwise the image name from imported image will be used.

- `source_name` (string) - Name of the source VM. Limited to 15 characters. Defaults to `source-<random-5-digit-suffix>`.

- `keep_input_artifact` (bool) - Preserve all the created objects in Supervisor cluster after the build finishes. Defaults to `false`.

- `bootstrap_provider` (string) - Name of the bootstrap provider to use for configuring the source VM.
  Supported values are `CloudInit`, `Sysprep`, and `vAppConfig`. Defaults to `CloudInit`.

- `bootstrap_data_file` (string) - Path to a file with bootstrap configuration data. Required if `bootstrap_provider` is not set to `CloudInit`.
  Defaults to a basic cloud config that sets up the user account from the SSH communicator config.

- `guest_os_type` (string) - The guest operating system identifier for the VM.
  Defaults to `otherGuest`.

- `iso_boot_disk_size` (string) - Size of the PVC that will be used as the boot disk when deploying an ISO VM.
  Supported units are `Gi`, `Mi`, `Ki`, `G`, `M`, `K`, etc.
  Defaults to `20Gi`.

<!-- End of code generated from the comments of the CreateSourceConfig struct in builder/vsphere/supervisor/step_create_source.go; -->


### Source Virtual Machine Watching

**Optional**:

<!-- Code generated from the comments of the WatchSourceConfig struct in builder/vsphere/supervisor/step_watch_source.go; DO NOT EDIT MANUALLY -->

- `watch_source_timeout_sec` (int) - The timeout in seconds to wait for the source VM to be ready. Defaults to `3600`.

<!-- End of code generated from the comments of the WatchSourceConfig struct in builder/vsphere/supervisor/step_watch_source.go; -->


### Source Virtual Machine Publishing

**Optional**:

<!-- Code generated from the comments of the PublishSourceConfig struct in builder/vsphere/supervisor/step_publish_source.go; DO NOT EDIT MANUALLY -->

- `publish_image_name` (string) - The name of the published VM image. If not specified, the vm-operator API will set a default name.

- `watch_publish_timeout_sec` (int) - The timeout in seconds to wait for the VM to be published. Defaults to `600`.

<!-- End of code generated from the comments of the PublishSourceConfig struct in builder/vsphere/supervisor/step_publish_source.go; -->


### Communicator Configuration

**Optional**:

<!-- Code generated from the comments of the SSH struct in communicator/config.go; DO NOT EDIT MANUALLY -->

- `ssh_host` (string) - The address to SSH to. This usually is automatically configured by the
  builder.

- `ssh_port` (int) - The port to connect to SSH. This defaults to `22`.

- `ssh_username` (string) - The username to connect to SSH with. Required if using SSH.

- `ssh_password` (string) - A plaintext password to use to authenticate with SSH.

- `ssh_ciphers` ([]string) - This overrides the value of ciphers supported by default by Golang.
  The default value is [
    "aes128-gcm@openssh.com",
    "chacha20-poly1305@openssh.com",
    "aes128-ctr", "aes192-ctr", "aes256-ctr",
  ]
  
  Valid options for ciphers include:
  "aes128-ctr", "aes192-ctr", "aes256-ctr", "aes128-gcm@openssh.com",
  "chacha20-poly1305@openssh.com",
  "arcfour256", "arcfour128", "arcfour", "aes128-cbc", "3des-cbc",

- `ssh_clear_authorized_keys` (bool) - If true, Packer will attempt to remove its temporary key from
  `~/.ssh/authorized_keys` and `/root/.ssh/authorized_keys`. This is a
  mostly cosmetic option, since Packer will delete the temporary private
  key from the host system regardless of whether this is set to true
  (unless the user has set the `-debug` flag). Defaults to "false";
  currently only works on guests with `sed` installed.

- `ssh_key_exchange_algorithms` ([]string) - If set, Packer will override the value of key exchange (kex) algorithms
  supported by default by Golang. Acceptable values include:
  "curve25519-sha256@libssh.org", "ecdh-sha2-nistp256",
  "ecdh-sha2-nistp384", "ecdh-sha2-nistp521",
  "diffie-hellman-group14-sha1", and "diffie-hellman-group1-sha1".

- `ssh_certificate_file` (string) - Path to user certificate used to authenticate with SSH.
  The `~` can be used in path and will be expanded to the
  home directory of current user.

- `ssh_pty` (bool) - If `true`, a PTY will be requested for the SSH connection. This defaults
  to `false`.

- `ssh_timeout` (duration string | ex: "1h5m2s") - The time to wait for SSH to become available. Packer uses this to
  determine when the machine has booted so this is usually quite long.
  Example value: `10m`.
  This defaults to `5m`, unless `ssh_handshake_attempts` is set.

- `ssh_disable_agent_forwarding` (bool) - If true, SSH agent forwarding will be disabled. Defaults to `false`.

- `ssh_handshake_attempts` (int) - The number of handshakes to attempt with SSH once it can connect.
  This defaults to `10`, unless a `ssh_timeout` is set.

- `ssh_bastion_host` (string) - A bastion host to use for the actual SSH connection.

- `ssh_bastion_port` (int) - The port of the bastion host. Defaults to `22`.

- `ssh_bastion_agent_auth` (bool) - If `true`, the local SSH agent will be used to authenticate with the
  bastion host. Defaults to `false`.

- `ssh_bastion_username` (string) - The username to connect to the bastion host.

- `ssh_bastion_password` (string) - The password to use to authenticate with the bastion host.

- `ssh_bastion_interactive` (bool) - If `true`, the keyboard-interactive used to authenticate with bastion host.

- `ssh_bastion_private_key_file` (string) - Path to a PEM encoded private key file to use to authenticate with the
  bastion host. The `~` can be used in path and will be expanded to the
  home directory of current user.

- `ssh_bastion_certificate_file` (string) - Path to user certificate used to authenticate with bastion host.
  The `~` can be used in path and will be expanded to the
  home directory of current user.

- `ssh_file_transfer_method` (string) - `scp` or `sftp` - How to transfer files, Secure copy (default) or SSH
  File Transfer Protocol.
  
  **NOTE**: Guests using Windows with Win32-OpenSSH v9.1.0.0p1-Beta, scp
  (the default protocol for copying data) returns a a non-zero error code since the MOTW
  cannot be set, which cause any file transfer to fail. As a workaround you can override the transfer protocol
  with SFTP instead `ssh_file_transfer_method = "sftp"`.

- `ssh_proxy_host` (string) - A SOCKS proxy host to use for SSH connection

- `ssh_proxy_port` (int) - A port of the SOCKS proxy. Defaults to `1080`.

- `ssh_proxy_username` (string) - The optional username to authenticate with the proxy server.

- `ssh_proxy_password` (string) - The optional password to use to authenticate with the proxy server.

- `ssh_keep_alive_interval` (duration string | ex: "1h5m2s") - How often to send "keep alive" messages to the server. Set to a negative
  value (`-1s`) to disable. Example value: `10s`. Defaults to `5s`.

- `ssh_read_write_timeout` (duration string | ex: "1h5m2s") - The amount of time to wait for a remote command to end. This might be
  useful if, for example, packer hangs on a connection after a reboot.
  Example: `5m`. Disabled by default.

- `ssh_remote_tunnels` ([]string) - Remote tunnels forward a port from your local machine to the instance.
  Format: ["REMOTE_PORT:LOCAL_HOST:LOCAL_PORT"]
  Example: "9090:localhost:80" forwards localhost:9090 on your machine to port 80 on the instance.

- `ssh_local_tunnels` ([]string) - Local tunnels forward a port from the instance to your local machine.
  Format: ["LOCAL_PORT:REMOTE_HOST:REMOTE_PORT"]
  Example: "8080:localhost:3000" allows the instance to access your local machine’s port 3000 via localhost:8080.

<!-- End of code generated from the comments of the SSH struct in communicator/config.go; -->


<!-- Code generated from the comments of the SSHTemporaryKeyPair struct in communicator/config.go; DO NOT EDIT MANUALLY -->

- `temporary_key_pair_type` (string) - `dsa` | `ecdsa` | `ed25519` | `rsa` ( the default )
  
  Specifies the type of key to create. The possible values are 'dsa',
  'ecdsa', 'ed25519', or 'rsa'.
  
  NOTE: DSA is deprecated and no longer recognized as secure, please
  consider other alternatives like RSA or ED25519.

- `temporary_key_pair_bits` (int) - Specifies the number of bits in the key to create. For RSA keys, the
  minimum size is 1024 bits and the default is 4096 bits. Generally, 3072
  bits is considered sufficient. DSA keys must be exactly 1024 bits as
  specified by FIPS 186-2. For ECDSA keys, bits determines the key length
  by selecting from one of three elliptic curve sizes: 256, 384 or 521
  bits. Attempting to use bit lengths other than these three values for
  ECDSA keys will fail. Ed25519 keys have a fixed length and bits will be
  ignored.
  
  NOTE: DSA is deprecated and no longer recognized as secure as specified
  by FIPS 186-5, please consider other alternatives like RSA or ED25519.

<!-- End of code generated from the comments of the SSHTemporaryKeyPair struct in communicator/config.go; -->


<!-- Code generated from the comments of the WinRM struct in communicator/config.go; DO NOT EDIT MANUALLY -->

- `winrm_username` (string) - The username to use to connect to WinRM.

- `winrm_password` (string) - The password to use to connect to WinRM.

- `winrm_host` (string) - The address for WinRM to connect to.
  
  NOTE: If using an Amazon EBS builder, you can specify the interface
  WinRM connects to via
  [`ssh_interface`](/packer/integrations/hashicorp/amazon/latest/components/builder/ebs#ssh_interface)

- `winrm_no_proxy` (bool) - Setting this to `true` adds the remote
  `host:port` to the `NO_PROXY` environment variable. This has the effect of
  bypassing any configured proxies when connecting to the remote host.
  Default to `false`.

- `winrm_port` (int) - The WinRM port to connect to. This defaults to `5985` for plain
  unencrypted connection and `5986` for SSL when `winrm_use_ssl` is set to
  true.

- `winrm_timeout` (duration string | ex: "1h5m2s") - The amount of time to wait for WinRM to become available. This defaults
  to `30m` since setting up a Windows machine generally takes a long time.

- `winrm_use_ssl` (bool) - If `true`, use HTTPS for WinRM.

- `winrm_insecure` (bool) - If `true`, do not check server certificate chain and host name.

- `winrm_use_ntlm` (bool) - If `true`, NTLMv2 authentication (with session security) will be used
  for WinRM, rather than default (basic authentication), removing the
  requirement for basic authentication to be enabled within the target
  guest. Further reading for remote connection authentication can be found
  [here](https://msdn.microsoft.com/en-us/library/aa384295(v=vs.85).aspx).

<!-- End of code generated from the comments of the WinRM struct in communicator/config.go; -->


## Deprovisioning Tasks

If you would like to clean up the virtual machine after the build is complete, you can use the
[Ansible provisioner](https://developer.hashicorp.com/packer/integrations/hashicorp/ansible/latest/components/provisioner/ansible)
to run the following tasks to delete machine-specific files and data.

HCL Example:

```hcl
build {
  sources = ["source.vsphere-supervisor.vm"]

  provisioner "ansible" {
    playbook_file = "cleanup-playbook.yml"
  }
}
```

JSON Example:

```json
{
  "builders": [
    {
      "type": "vsphere-supervisor"
    }
  ],
  "provisioners": [
    {
      "type": "ansible",
      "playbook_file": "./cleanup-playbook.yml"
    }
  ]
}
```

Content of `cleanup-playbook.yml`:

```yaml
---
# cleanup-playbook.yml
- name: Clean up source virtual machine
  hosts: default
  become: true
  tasks:
    - name: Truncate machine id
      file:
        state: "{{ item.state }}"
        path: "{{ item.path }}"
        owner: root
        group: root
        mode: "{{ item.mode }}"
      loop:
        - { path: /etc/machine-id, state: absent, mode: "0644" }
        - { path: /etc/machine-id, state: touch, mode: "0644" }

    - name: Truncate audit logs
      file:
        state: "{{ item.state }}"
        path: "{{ item.path }}"
        owner: root
        group: utmp
        mode: "{{ item.mode }}"
      loop:
        - { path: /var/log/wtmp, state: absent, mode: "0664" }
        - { path: /var/log/lastlog, state: absent, mode: "0644" }
        - { path: /var/log/wtmp, state: touch, mode: "0664" }
        - { path: /var/log/lastlog, state: touch, mode: "0644" }

    - name: Remove cloud-init lib dir and logs
      file:
        state: absent
        path: "{{ item }}"
      loop:
        - /var/lib/cloud
        - /var/log/cloud-init.log
        - /var/log/cloud-init-output.log
        - /var/run/cloud-init

    - name: Truncate all remaining log files in /var/log
      shell:
        cmd: |
          find /var/log -type f -iname '*.log' | xargs truncate -s 0

    - name: Delete all logrotated log zips
      shell:
        cmd: |
          find /var/log -type f -name '*.gz' -exec rm {} +

    - name: Find temp files
      find:
        depth: 1
        file_type: any
        paths:
          - /tmp
          - /var/tmp
        pattern: "*"
      register: temp_files

    - name: Reset temp space
      file:
        state: absent
        path: "{{ item.path }}"
      loop: "{{ temp_files.files }}"

    - name: Truncate shell history
      file:
        state: absent
        path: "{{ item.path }}"
      loop:
        - { path: /root/.bash_history }
        - {
            path: "/home/{{ ansible_env.SUDO_USER | default(ansible_user_id) }}/.bash_history",
          }
```
