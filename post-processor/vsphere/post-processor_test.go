// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package vsphere

import (
	"fmt"
	"net/url"
	"os/exec"
	"strings"
	"testing"
)

func getTestConfig() Config {
	return Config{
		Username:   "me",
		Password:   "notpassword",
		Host:       "myhost",
		Datacenter: "mydc",
		Cluster:    "mycluster",
		VMName:     "my vm",
		Datastore:  "my datastore",
		Insecure:   true,
		DiskMode:   "thin",
		VMFolder:   "my folder",
	}
}

func TestArgs(t *testing.T) {
	var p PostProcessor

	p.config = getTestConfig()

	source := "something.vmx"
	ovftoolURI := fmt.Sprintf("vi://%s:%s@%s/%s/host/%s",
		url.QueryEscape(p.config.Username),
		url.QueryEscape(p.config.Password),
		p.config.Host,
		p.config.Datacenter,
		p.config.Cluster)

	if p.config.ResourcePool != "" {
		ovftoolURI += "/Resources/" + p.config.ResourcePool
	}

	args, err := p.BuildArgs(source, ovftoolURI)
	if err != nil {
		t.Errorf("Error: %s", err)
	}

	t.Logf("ovftool %s", strings.Join(args, " "))
}

func TestGenerateURI_Basic(t *testing.T) {
	var p PostProcessor

	p.config = getTestConfig()

	uri, err := p.generateURI()
	if err != nil {
		t.Fatalf("unexpected error: '%s'", err)
	}
	expectedURI := "vi://me:notpassword@myhost/mydc/host/mycluster"
	if uri.String() != expectedURI {
		t.Fatalf("unexpected result: expected '%s', but returned '%s'", expectedURI, uri)
	}
}

func TestGenerateURI_PasswordEscapes(t *testing.T) {
	type escapeCases struct {
		Input    string
		Expected string
	}

	cases := []escapeCases{
		{`this has spaces`, `this%20has%20spaces`},
		{`exclamation_!`, `exclamation_%21`},
		{`hash_#_dollar_$`, `hash_%23_dollar_$`},
		{`ampersand_&awesome`, `ampersand_&awesome`},
		{`single_quote_'_and_another_'`, `single_quote_%27_and_another_%27`},
		{`open_paren_(_close_paren_)`, `open_paren_%28_close_paren_%29`},
		{`asterisk_*_plus_+`, `asterisk_%2A_plus_+`},
		{`comma_,slash_/`, `comma_,slash_%2F`},
		{`colon_:semicolon_;`, `colon_%3Asemicolon_;`},
		{`equal_=question_?`, `equal_=question_%3F`},
		{`at_@`, `at_%40`},
		{`open_bracket_[closed_bracket]`, `open_bracket_%5Bclosed_bracket%5D`},
		{`user:password with $paces@host/name.foo`, `user%3Apassword%20with%20$paces%40host%2Fname.foo`},
	}

	for _, escapeCase := range cases {
		var p PostProcessor

		p.config = getTestConfig()
		p.config.Password = escapeCase.Input

		uri, err := p.generateURI()
		if err != nil {
			t.Fatalf("unexpected error: '%s'", err)
		}
		expectedURI := fmt.Sprintf("vi://me:%s@myhost/mydc/host/mycluster", escapeCase.Expected)

		if uri.String() != expectedURI {
			t.Fatalf("unexpected result: expected '%s', but returned '%s'", expectedURI, uri)
		}
	}
}

func TestGetEncodedPassword(t *testing.T) {

	// Password is encoded and contains a colon
	ovftoolURI := "vi://hostname/Datacenter/host/cluster"

	u, _ := url.Parse(ovftoolURI)
	u.User = url.UserPassword("us:ername", "P@ssW:rd")

	encoded, isSet := getEncodedPassword(u)
	expected := "P%40ssW%3Ard"
	if !isSet {
		t.Fatalf("unexpected result: expected 'true', but returned '%t'", isSet)
	}
	if encoded != expected {
		t.Fatalf("unexpected result: expected '%s', but returned '%s'", expected, encoded)
	}

	// There is no password
	u.User = url.UserPassword("us:ername", "")

	_, isSet = getEncodedPassword(u)
	if isSet {
		t.Fatalf("unexpected result: expected 'false', but returned '%t'", isSet)
	}

}

func TestConfigure_WithTags(t *testing.T) {
	// Skip if ovftool is not available
	if _, err := exec.LookPath(ovftool); err != nil {
		t.Skip("ovftool not found, skipping test")
	}

	var p PostProcessor

	config := map[string]any{
		"username":   "me",
		"password":   "notpassword",
		"host":       "myhost",
		"datacenter": "mydc",
		"cluster":    "mycluster",
		"vm_name":    "my vm",
		"datastore":  "my datastore",
		"tags":       []string{"urn:vmomi:InventoryServiceTag:12345678-1234-1234-1234-123456789012:GLOBAL"},
	}

	err := p.Configure(config)
	if err != nil {
		t.Fatalf("unexpected error with valid tag configuration: %s", err)
	}

	if len(p.config.Tags) != 1 {
		t.Fatalf("expected 1 tag, got %d", len(p.config.Tags))
	}
}

func TestConfigure_WithTagBlocks(t *testing.T) {
	// Skip if ovftool is not available
	if _, err := exec.LookPath(ovftool); err != nil {
		t.Skip("ovftool not found, skipping test")
	}

	var p PostProcessor

	config := map[string]any{
		"username":   "me",
		"password":   "notpassword",
		"host":       "myhost",
		"datacenter": "mydc",
		"cluster":    "mycluster",
		"vm_name":    "my vm",
		"datastore":  "my datastore",
		"tag": []map[string]any{
			{
				"category": "environment",
				"name":     "production",
			},
		},
	}

	err := p.Configure(config)
	if err != nil {
		t.Fatalf("unexpected error with valid tag block configuration: %s", err)
	}

	if len(p.config.Tag) != 1 {
		t.Fatalf("expected 1 tag block, got %d", len(p.config.Tag))
	}
}
