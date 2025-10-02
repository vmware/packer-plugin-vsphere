// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package clone

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
)

// HttpTestServer provides HTTP and HTTPS test servers for testing remote OVF/OVA functionality.
type HttpTestServer struct {
	HttpServer  *httptest.Server
	HttpsServer *httptest.Server
	ovfContent  string
	ovaContent  string
}

// NewHttpTestServer creates HTTP and HTTPS test servers with authentication endpoints.
func NewHttpTestServer() *HttpTestServer {
	ts := &HttpTestServer{
		ovfContent: generateMinimalOvfContent(),
		ovaContent: generateMinimalOvaContent(),
	}

	handler := ts.createHandler()
	ts.HttpServer = httptest.NewServer(handler)
	ts.HttpsServer = httptest.NewTLSServer(handler)

	return ts
}

// Close shuts down both HTTP and HTTPS test servers.
func (ts *HttpTestServer) Close() {
	if ts.HttpServer != nil {
		ts.HttpServer.Close()
	}
	if ts.HttpsServer != nil {
		ts.HttpsServer.Close()
	}
}

// URL generation methods

// GetHttpUrl returns the HTTP server URL for the specified file type.
func (ts *HttpTestServer) GetHttpUrl(fileType string) string {
	return fmt.Sprintf("%s/%s", ts.HttpServer.URL, fileType)
}

// GetHttpsUrl returns the HTTPS server URL for the specified file type.
func (ts *HttpTestServer) GetHttpsUrl(fileType string) string {
	return fmt.Sprintf("%s/%s", ts.HttpsServer.URL, fileType)
}

// GetHttpAuthUrl returns the HTTP server URL with basic authentication endpoint for the specified file type.
func (ts *HttpTestServer) GetHttpAuthUrl(fileType string) string {
	return fmt.Sprintf("%s/auth/%s", ts.HttpServer.URL, fileType)
}

// GetHttpsAuthUrl returns the HTTPS server URL with basic authentication endpoint for the specified file type.
func (ts *HttpTestServer) GetHttpsAuthUrl(fileType string) string {
	return fmt.Sprintf("%s/auth/%s", ts.HttpsServer.URL, fileType)
}

// GetInsecureHttpsClient returns an HTTP client that skips TLS verification.
func (ts *HttpTestServer) GetInsecureHttpsClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
}

// HTTP handler setup

// createHandler creates the HTTP handler for both servers.
func (ts *HttpTestServer) createHandler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/example.ovf", ts.handleOvf)
	mux.HandleFunc("/example.ova", ts.handleOva)

	mux.HandleFunc("/auth/", ts.handleBasicAuth)
	mux.HandleFunc("/auth/special/", ts.handleSpecialAuth)

	mux.HandleFunc("/error/404", ts.handleError404)
	mux.HandleFunc("/error/500", ts.handleError500)
	mux.HandleFunc("/error/timeout", ts.handleErrorTimeout)

	return mux
}

// HTTP endpoint handlers

// handleOvf serves OVF content with appropriate headers.
func (ts *HttpTestServer) handleOvf(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/xml")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(ts.ovfContent))
}

// handleOva serves OVA content with appropriate headers.
func (ts *HttpTestServer) handleOva(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/octet-stream")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(ts.ovaContent))
}

// handleBasicAuth handles basic authentication endpoints.
func (ts *HttpTestServer) handleBasicAuth(w http.ResponseWriter, r *http.Request) {
	username, password, ok := r.BasicAuth()
	if !ok {
		ts.requireAuth(w, "Test OVF Server")
		_, _ = w.Write([]byte("401 Unauthorized - Basic authentication required"))
		return
	}

	if username != "testuser" || password != "testpass" {
		ts.requireAuth(w, "Test OVF Server")
		_, _ = w.Write([]byte("401 Unauthorized - Invalid credentials"))
		return
	}

	ts.serveFileByPath(w, strings.TrimPrefix(r.URL.Path, "/auth/"))
}

// handleSpecialAuth handles authentication with special characters in credentials.
func (ts *HttpTestServer) handleSpecialAuth(w http.ResponseWriter, r *http.Request) {
	username, password, ok := r.BasicAuth()
	if !ok {
		ts.requireAuth(w, "Special Chars Test")
		return
	}

	if username != "testuser@packages.example.com" || password != "VMw@re1!" {
		ts.requireAuth(w, "Special Chars Test")
		_, _ = w.Write([]byte("401 Unauthorized - Invalid special character credentials"))
		return
	}

	ts.serveFileByPath(w, strings.TrimPrefix(r.URL.Path, "/auth/special/"))
}

// handleError404 simulates a 404 Not Found error.
func (ts *HttpTestServer) handleError404(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotFound)
	_, _ = w.Write([]byte("404 Not Found - File does not exist"))
}

// handleError500 simulates a 500 Internal Server Error.
func (ts *HttpTestServer) handleError500(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusInternalServerError)
	_, _ = w.Write([]byte("500 Internal Server Error - Server error"))
}

// handleErrorTimeout simulates a slow response for timeout testing.
func (ts *HttpTestServer) handleErrorTimeout(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("This endpoint simulates slow responses"))
}

// requireAuth sets WWW-Authenticate header and returns 401 status.
func (ts *HttpTestServer) requireAuth(w http.ResponseWriter, realm string) {
	w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Basic realm="%s"`, realm))
	w.WriteHeader(http.StatusUnauthorized)
}

// serveFileByPath serves OVF or OVA content based on file extension.
func (ts *HttpTestServer) serveFileByPath(w http.ResponseWriter, path string) {
	switch {
	case strings.HasSuffix(path, ".ovf"):
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(ts.ovfContent))
	case strings.HasSuffix(path, ".ova"):
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(ts.ovaContent))
	default:
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("404 Not Found"))
	}
}

// Content generation functions

// generateMinimalOvfContent creates minimal valid OVF XML content for testing.
func generateMinimalOvfContent() string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<Envelope xmlns="http://schemas.dmtf.org/ovf/envelope/1"
          xmlns:ovf="http://schemas.dmtf.org/ovf/envelope/1"
          xmlns:rasd="http://schemas.dmtf.org/wbem/wscim/1/cim-schema/2/CIM_ResourceAllocationSettingData"
          xmlns:vssd="http://schemas.dmtf.org/wbem/wscim/1/cim-schema/2/CIM_VirtualSystemSettingData"
          xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
  <References>
    <File ovf:href="test-disk1.vmdk" ovf:id="file1" ovf:size="1024"/>
  </References>
  <DiskSection>
    <Info>Virtual disk information</Info>
    <Disk ovf:capacity="1" ovf:diskId="vmdisk1" ovf:fileRef="file1" ovf:format="http://www.vmware.com/interfaces/specifications/vmdk.html#streamOptimized"/>
  </DiskSection>
  <NetworkSection>
    <Info>Logical networks</Info>
    <Network ovf:name="VM Network">
      <Description>The VM Network network</Description>
    </Network>
  </NetworkSection>
  <VirtualSystem ovf:id="test-vm">
    <Info>A test virtual machine</Info>
    <Name>Test VM</Name>
    <OperatingSystemSection ovf:id="36">
      <Info>The kind of installed guest operating system</Info>
      <Description>Ubuntu Linux (64-bit)</Description>
    </OperatingSystemSection>
    <VirtualHardwareSection>
      <Info>Virtual hardware requirements</Info>
      <System>
        <vssd:ElementName>Virtual Hardware Family</vssd:ElementName>
        <vssd:InstanceID>0</vssd:InstanceID>
        <vssd:VirtualSystemIdentifier>test-vm</vssd:VirtualSystemIdentifier>
        <vssd:VirtualSystemType>vmx-13</vssd:VirtualSystemType>
      </System>
      <Item>
        <rasd:AllocationUnits>hertz * 10^6</rasd:AllocationUnits>
        <rasd:Description>Number of Virtual CPUs</rasd:Description>
        <rasd:ElementName>1 virtual CPU(s)</rasd:ElementName>
        <rasd:InstanceID>1</rasd:InstanceID>
        <rasd:ResourceType>3</rasd:ResourceType>
        <rasd:VirtualQuantity>1</rasd:VirtualQuantity>
      </Item>
      <Item>
        <rasd:AllocationUnits>byte * 2^20</rasd:AllocationUnits>
        <rasd:Description>Memory Size</rasd:Description>
        <rasd:ElementName>1024MB of memory</rasd:ElementName>
        <rasd:InstanceID>2</rasd:InstanceID>
        <rasd:ResourceType>4</rasd:ResourceType>
        <rasd:VirtualQuantity>1024</rasd:VirtualQuantity>
      </Item>
      <Item>
        <rasd:Address>0</rasd:Address>
        <rasd:Description>SCSI Controller</rasd:Description>
        <rasd:ElementName>SCSI controller 0</rasd:ElementName>
        <rasd:InstanceID>3</rasd:InstanceID>
        <rasd:ResourceSubType>lsilogic</rasd:ResourceSubType>
        <rasd:ResourceType>6</rasd:ResourceType>
      </Item>
      <Item>
        <rasd:AddressOnParent>0</rasd:AddressOnParent>
        <rasd:ElementName>Hard disk 1</rasd:ElementName>
        <rasd:HostResource>ovf:/disk/vmdisk1</rasd:HostResource>
        <rasd:InstanceID>4</rasd:InstanceID>
        <rasd:Parent>3</rasd:Parent>
        <rasd:ResourceType>17</rasd:ResourceType>
      </Item>
      <Item>
        <rasd:AddressOnParent>7</rasd:AddressOnParent>
        <rasd:AutomaticAllocation>true</rasd:AutomaticAllocation>
        <rasd:Connection>VM Network</rasd:Connection>
        <rasd:Description>VmxNet3 ethernet adapter on "VM Network"</rasd:Description>
        <rasd:ElementName>Network adapter 1</rasd:ElementName>
        <rasd:InstanceID>5</rasd:InstanceID>
        <rasd:ResourceSubType>VmxNet3</rasd:ResourceSubType>
        <rasd:ResourceType>10</rasd:ResourceType>
      </Item>
    </VirtualHardwareSection>
    <ProductSection>
      <Info>Product information</Info>
      <Product>Test VM Product</Product>
      <Vendor>Test Vendor</Vendor>
      <Version>1.0</Version>
      <Property ovf:key="hostname" ovf:type="string" ovf:userConfigurable="true">
        <Label>Hostname</Label>
        <Description>The hostname for the virtual machine</Description>
      </Property>
      <Property ovf:key="user-data" ovf:type="string" ovf:userConfigurable="true">
        <Label>User Data</Label>
        <Description>Cloud-init user data</Description>
      </Property>
    </ProductSection>
  </VirtualSystem>
</Envelope>`
}

// generateMinimalOvaContent creates minimal OVA content for testing.
func generateMinimalOvaContent() string {
	return "MINIMAL_OVA_CONTENT_FOR_TESTING_PURPOSES_BINARY_DATA_SIMULATION"
}
