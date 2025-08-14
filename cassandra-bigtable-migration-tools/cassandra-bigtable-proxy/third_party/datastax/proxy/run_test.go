// Copyright (c) DataStax, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package proxy

import (
	"context"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/bigtable"
	bt "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/bigtable"
	mockbigtable "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/mocks/bigtable"
	rh "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/responsehandler"
	schemaMapping "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/schema-mapping"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
)

func Test_parseProtocolVersion(t *testing.T) {

	a := []string{"3", "4", "5", "65", "66", "invalid"}
	for _, val := range a {
		res, boolval := parseProtocolVersion(val)
		assert.NotNilf(t, res, "should not be nil")
		if val == "invalid" {
			assert.Equalf(t, false, boolval, "should be true")
			break
		}
		assert.Equalf(t, true, boolval, "should be true")
	}

}

func Test_maybeAddPort(t *testing.T) {
	res := maybeAddPort("127.0.0.1", "7000")
	assert.Equalf(t, res, "127.0.0.1:7000", "assert equal")
}

func Test_listenAndServe(t *testing.T) {}

var testKeyPEM = []byte(`
-----BEGIN RSA PRIVATE KEY-----
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
-----END RSA PRIVATE KEY-----
`)

var testCertPEM = []byte(`
-----BEGIN CERTIFICATE-----
MIIF0DCCA7igAwIBAgICBnowDQYJKoZIhvcNAQELBQAwdzELMAkGA1UEBhMCVVMx
CzAJBgNVBAgTAkNBMRQwEgYDVQQHEwtTYW50YSBDbGFyYTEcMBoGA1UECRMTMzk3
NSBGcmVlZG9tIENpcmNsZTEOMAwGA1UEERMFOTUwNTQxFzAVBgNVBAoTDkRhdGFT
dGF4LCBpbmMuMB4XDTIyMDQyOTE4MTYyMVoXDTMyMDQyOTE4MTYyMVowdzELMAkG
A1UEBhMCVVMxCzAJBgNVBAgTAkNBMRQwEgYDVQQHEwtTYW50YSBDbGFyYTEcMBoG
A1UECRMTMzk3NSBGcmVlZG9tIENpcmNsZTEOMAwGA1UEERMFOTUwNTQxFzAVBgNV
BAoTDkRhdGFTdGF4LCBpbmMuMIICIjANBgkqhkiG9w0BAQEFAAOCAg8AMIICCgKC
AgEAsZjsUojrggVN9qJ2+uC0R8celNRl88Fw0fsvPazY7s0C9qj+c9FhlS+KJznt
w5DeC+eWm50x+Pjj+2nCzO85iUGty491beSIt/0WECwZkK/TuCrrF8p3c1c2LkZI
WLsTk6KHAKQKA6J7flrIE1JTYCeLnOnusTGX/Y9/hCTxwPQziFdCH3Slet3iDTN5
ABcG46mIEoUo0sBvEQf2X1ZHpMdgNyxWDcYIkXraSZP6/Rap1hCaCswe68Ti17Z1
Vf1pIJXSrPyeyOAyt/oFGxcO4H1PTq0iLFB+XmlpgSNPawEDDgQztt3mMrAyvY4P
rnYNg34aftxSCWxmYqKvqD9dDZPS3cRkm04I7hA83JtiVj3/pvEtsY28Jf4ldimV
DcI3GgeEjLE0KJd0v0Y9wwS2iPDxDqFsKNun6mOTbyG5jPI794Sv/4Lr2jmOknTp
63lZyb400AVr5ThqL7dDESganHZT2dziziqNmGNdDC+t4b1RSN/SNGrYT9cn1XBP
gJj6IY0VGFTF/IIAhfAU36g2BZ4PXNyv/l61kp5AmyyMAF2JSKuknQIVKVd50paj
IBT34BFzSxVyS2XM8i+fEQtwdwTV/xhtq1qczSr1r25ChHUQrCGNewQpG59oTDUz
nvnbY4ydbruYXwvxEngO92uX4I3wUiUbHyo7IsjEAZ5aRisCAwEAAaNmMGQwDgYD
VR0PAQH/BAQDAgeAMB0GA1UdJQQWMBQGCCsGAQUFBwMCBggrBgEFBQcDATAOBgNV
HQ4EBwQFAQIDBAYwIwYDVR0RBBwwGoIAhwR/AAABhxAAAAAAAAAAAAAAAAAAAAAB
MA0GCSqGSIb3DQEBCwUAA4ICAQCCBQ31mkX5ejdtAmRQJD6gYYJtDJztmiX2xuzr
PPs8Q/NhxHG3JYdk2yiSmU3Jq0WjPsNyAU/XWJ3UnnMD5JhcEUENA8saTmOldFde
MhfeIQAyd+KZtj2KT1oiQalBjSRXMggV57YcMoWDYFUzGOY2ecog548FvKeoOKOo
5ajic8p+hYHjkz8TM+3wZ4wzygj8i7XvD+Hhob8sdU+oTxgIJoV431PaCwxn8lHT
oXHTD1UsGCXm/Supkq3oLB5OfuWE0JSrAaA3Nndt4PnK9kisG1cX8e99OrR/c8eV
JEUsSZxOC4ftjMtGs0J/+DBQs4RTi4+VhHM5xo6HerCLR5/kH2hjxqtnhNFbbev3
4/yb8KPTO3XVf03rJBFlmjjfToTcmNjE8rSDcGtB0/XcyWUYn3fmWntmJbrIVHyF
nkmm2/ZHAMJfIYFxniwF1KAfqMkJsY49ziS0WjjU9VvD7sGSR7KzJFSVc31eIjBf
0hy3NdkgS73JSQo4C61lyIi2w4L02rSn2Gh/b3J26xxxpPVML96uXGFWDpZEJtOR
DqJzOELCZQrh+HKtzauG/fuSa+SpfSC9/aeVh64JkfJmdNN/0yINOO3STUs5YibG
QhZVrqVrfwPNosy/TfhoU8kE8xI9JchbKh5MAg8+rDQRtZ0Lyt8a0rvYTA/EvxrV
i8aCxg==
-----END CERTIFICATE-----
`)

var testCAPEM = []byte(`
-----BEGIN CERTIFICATE-----
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
-----END CERTIFICATE-----
`)

// signalContext is a simplified version of `signal.NotifyContext()` for  golang 1.15 and earlier
func signalContext(parent context.Context, sig ...os.Signal) (context.Context, func()) {
	ctx, cancel := context.WithCancel(parent)
	ch := make(chan os.Signal)
	signal.Notify(ch, sig...)
	if ctx.Err() == nil {
		go func() {
			select {
			case <-ch:
				cancel()
			case <-ctx.Done():
			}
		}()
	}
	return ctx, func() {
		cancel()
		signal.Stop(ch)
	}
}

func TestLoadConfig(t *testing.T) {
	type args struct {
		filename string
	}
	tests := []struct {
		name    string
		args    args
		want    *UserConfig
		wantErr bool
	}{
		{
			name: "Valid config file",
			args: args{
				filename: "testdata/valid_config.yaml",
			},
			want: &UserConfig{
				Listeners: []Listener{
					{
						Name: "clusterA",
						Port: 9092,
						Bigtable: Bigtable{
							ProjectID:           "cassandra-prod-789",
							Instances:           []InstancesMap{{BigtableInstance: "prod-instance-001", Keyspace: "prodinstance001"}},
							SchemaMappingTable:  "prod_table_config",
							DefaultColumnFamily: "cf_default",
							AppProfileID:        "prod-profile-123",
							Session: Session{
								GrpcChannels: 3,
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Non-existent config file",
			args: args{
				filename: "testdata/non_existent.yaml",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Invalid config format",
			args: args{
				filename: "testdata/invalid_config.yaml",
			},
			want:    nil,
			wantErr: true,
		},
		{
			name: "Config with missing required fields",
			args: args{
				filename: "testdata/missing_fields_config.yaml",
			},
			want:    nil,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := LoadConfig(tt.args.filename)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("LoadConfig() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRun(t *testing.T) {
	// Create a context with a timeout for testing
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel() // Ensure resources are released
	t.Setenv("CONFIG_FILE", "../fakedata/testConfigFile.yaml")
	tbData := make(map[string]*schemaMapping.TableConfig)
	bgtmockface := new(mockbigtable.BigTableClientIface)
	bgtmockface.On("GetSchemaMappingConfigs", ctx, "bigtabledevinstancetest", "schema_mapping_test").Return(tbData, nil)
	bgtmockface.On("LoadConfigs", mock.AnythingOfType("*responsehandler.TypeHandler"), mock.AnythingOfType("*schemaMapping.SchemaMappingConfig")).Return(tbData, nil)

	bgtmockface.On("Close").Return()

	// Override the factory function to return the mock
	originalNewBigTableClient := bt.NewBigtableClient
	bt.NewBigtableClient = func(client map[string]*bigtable.Client, adminClients map[string]*bigtable.AdminClient, logger *zap.Logger, config bt.BigtableConfig, responseHandler rh.ResponseHandlerIface, schemaMapping *schemaMapping.SchemaMappingConfig, instancesMap map[string]bt.InstanceConfig) bt.BigTableClientIface {
		return bgtmockface
	}
	defer func() { bt.NewBigtableClient = originalNewBigTableClient }()

	code := Run(ctx, []string{})
	assert.Equal(t, 0, code)
}

func Test_resolveAndListen(t *testing.T) {
	// Create temporary files for TLS certificates
	certFile, err := os.CreateTemp("", "cert*.pem")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(certFile.Name())

	keyFile, err := os.CreateTemp("", "key*.pem")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(keyFile.Name())

	// Write test certificates (using existing test certificates)
	if err := os.WriteFile(certFile.Name(), testCertPEM, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyFile.Name(), testKeyPEM, 0644); err != nil {
		t.Fatal(err)
	}

	logger := zap.NewExample()

	// Create a directory for socket testing with proper permissions initially
	socketDir := t.TempDir()
	if err := os.Chmod(socketDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create a test socket file that can't be removed
	testSocketPath := filepath.Join(socketDir, "test.sock")
	if err := os.WriteFile(testSocketPath, []byte{}, 0444); err != nil {
		t.Fatal(err)
	}
	// Make the directory read-only to prevent file removal
	if err := os.Chmod(socketDir, 0555); err != nil {
		t.Fatal(err)
	}
	// Restore directory permissions after tests complete
	defer func() {
		os.Chmod(socketDir, 0755)
	}()

	tests := []struct {
		name           string
		bind           string
		useUnixSocket  bool
		unixSocketPath string
		certFile       string
		keyFile        string
		wantErr        bool
		errorContains  string
		checkListener  func(t *testing.T, listener net.Listener)
	}{
		{
			name:          "TCP listener success",
			bind:          "localhost:0",
			useUnixSocket: false,
			wantErr:       false,
			checkListener: func(t *testing.T, listener net.Listener) {
				if listener == nil {
					t.Error("Expected TCP listener, got nil")
				}
				if _, ok := listener.(*net.TCPListener); !ok {
					t.Error("Expected TCP listener type")
				}
			},
		},
		{
			name:          "TCP listener invalid address",
			bind:          "invalid:address:",
			useUnixSocket: false,
			wantErr:       true,
			errorContains: "listen tcp: address invalid:address:: too many colons in address",
		},
		{
			name:           "UDS listener success",
			bind:           "localhost:0",
			useUnixSocket:  true,
			unixSocketPath: "/tmp/test.sock",
			wantErr:        false,
			checkListener: func(t *testing.T, listener net.Listener) {
				if listener == nil {
					t.Error("Expected UDS listener, got nil")
				}
				if _, ok := listener.(*net.UnixListener); !ok {
					t.Error("Expected Unix Domain Socket listener type")
				}
			},
		},
		{
			name:           "UDS listener invalid path",
			bind:           "localhost:0",
			useUnixSocket:  true,
			unixSocketPath: "/nonexistent/directory/test.sock",
			wantErr:        true,
			errorContains:  "failed to create Unix Domain Socket",
		},
		{
			name:          "TLS listener success",
			bind:          "localhost:0",
			useUnixSocket: false,
			certFile:      certFile.Name(),
			keyFile:       keyFile.Name(),
			wantErr:       false,
			checkListener: func(t *testing.T, listener net.Listener) {
				if listener == nil {
					t.Error("Expected TLS listener, got nil")
					return
				}

				// Verify it's not a plain TCP or Unix listener
				if _, ok := listener.(*net.TCPListener); ok {
					t.Error("Got TCP listener instead of TLS listener")
					return
				}
				if _, ok := listener.(*net.UnixListener); ok {
					t.Error("Got Unix listener instead of TLS listener")
					return
				}

				// Verify the address configuration
				addr := listener.Addr().(*net.TCPAddr)
				if addr == nil {
					t.Error("Expected non-nil TCP address")
					return
				}

				// Verify the port is assigned (should be non-zero for random port)
				if addr.Port == 0 {
					t.Error("Expected non-zero port")
				}
			},
		},
		{
			name:          "TLS listener with invalid cert file",
			bind:          "localhost:0",
			useUnixSocket: false,
			certFile:      "nonexistent.pem",
			keyFile:       keyFile.Name(),
			wantErr:       true,
			errorContains: "unable to load TLS certificate pair",
		},
		{
			name:          "TLS listener with invalid key file",
			bind:          "localhost:0",
			useUnixSocket: false,
			certFile:      certFile.Name(),
			keyFile:       "nonexistent.pem",
			wantErr:       true,
			errorContains: "unable to load TLS certificate pair",
		},
		{
			name:           "UDS listener with existing socket file",
			bind:           "localhost:0",
			useUnixSocket:  true,
			unixSocketPath: "/tmp/test_existing.sock",
			wantErr:        false,
			checkListener: func(t *testing.T, listener net.Listener) {
				if listener == nil {
					t.Error("Expected UDS listener, got nil")
				}
				if _, ok := listener.(*net.UnixListener); !ok {
					t.Error("Expected Unix Domain Socket listener type")
				}
			},
		},
		{
			name:           "UDS listener with permission issues",
			bind:           "localhost:0",
			useUnixSocket:  true,
			unixSocketPath: "/nonexistent/test.sock", // Using a path that will definitely fail in all environments
			wantErr:        true,
			errorContains:  "failed to create Unix Domain Socket", // Generic error that works in all environments
		},
		{
			name:           "UDS listener with successful socket creation and permissions",
			bind:           "localhost:0",
			useUnixSocket:  true,
			unixSocketPath: "/tmp/test_success.sock",
			wantErr:        false,
			checkListener: func(t *testing.T, listener net.Listener) {
				if listener == nil {
					t.Error("Expected UDS listener, got nil")
				}
				if _, ok := listener.(*net.UnixListener); !ok {
					t.Error("Expected Unix Domain Socket listener type")
				}
				// Verify socket permissions
				info, err := os.Stat("/tmp/test_success.sock")
				if err != nil {
					t.Errorf("Failed to stat socket file: %v", err)
				}
				if info.Mode().Perm() != 0600 {
					t.Errorf("Expected socket permissions 0600, got %o", info.Mode().Perm())
				}
			},
		},
		{
			name:          "TLS listener with successful certificate loading",
			bind:          "localhost:0",
			useUnixSocket: false,
			certFile:      certFile.Name(),
			keyFile:       keyFile.Name(),
			wantErr:       false,
			checkListener: func(t *testing.T, listener net.Listener) {
				if listener == nil {
					t.Error("Expected TLS listener, got nil")
					return
				}

				// Verify it's not a plain TCP or Unix listener
				if _, ok := listener.(*net.TCPListener); ok {
					t.Error("Got TCP listener instead of TLS listener")
					return
				}
				if _, ok := listener.(*net.UnixListener); ok {
					t.Error("Got Unix listener instead of TLS listener")
					return
				}

				// Verify the address configuration
				addr := listener.Addr().(*net.TCPAddr)
				if addr == nil {
					t.Error("Expected non-nil TCP address")
					return
				}

				// Verify the port is assigned (should be non-zero for random port)
				if addr.Port == 0 {
					t.Error("Expected non-zero port")
				}
			},
		},
		{
			name:          "TCP listener with successful creation",
			bind:          "localhost:0",
			useUnixSocket: false,
			wantErr:       false,
			checkListener: func(t *testing.T, listener net.Listener) {
				if listener == nil {
					t.Error("Expected TCP listener, got nil")
					return
				}

				// Type assertion
				tcpListener, ok := listener.(*net.TCPListener)
				if !ok {
					t.Errorf("Expected *net.TCPListener, got %T", listener)
					return
				}

				// Verify address configuration
				addr := tcpListener.Addr().(*net.TCPAddr)
				if addr == nil {
					t.Error("Expected non-nil TCP address")
					return
				}

				if addr.Port == 0 {
					t.Error("Expected non-zero port")
				}
			},
		},
		{
			name:           "UDS listener with permission error on socket removal",
			bind:           "localhost:0",
			useUnixSocket:  true,
			unixSocketPath: testSocketPath,
			wantErr:        true,
			errorContains:  "permission", // More flexible error message check
		},
		{
			name:           "UDS listener with permission error on chmod",
			bind:           "localhost:0",
			useUnixSocket:  true,
			unixSocketPath: filepath.Join(socketDir, "new.sock"),
			wantErr:        true,
			errorContains:  "permission", // More flexible error message check
		},
		{
			name:          "TLS listener with invalid bind address",
			bind:          "256.256.256.256:9042", // Invalid IP address
			useUnixSocket: false,
			certFile:      certFile.Name(),
			keyFile:       keyFile.Name(),
			wantErr:       true,
			errorContains: "failed to create TLS listener",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// For the chmod test, make the directory read-only before the test
			if tt.name == "UDS listener with permission error on chmod" {
				if err := os.Chmod(socketDir, 0555); err != nil {
					t.Fatal(err)
				}
				// Restore permissions after the test
				defer func() {
					if err := os.Chmod(socketDir, 0755); err != nil {
						t.Fatal(err)
					}
				}()
			}

			listener, err := resolveAndListen(tt.bind, tt.useUnixSocket, tt.unixSocketPath, tt.certFile, tt.keyFile, logger)

			if (err != nil) != tt.wantErr {
				t.Errorf("resolveAndListen() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil && tt.errorContains != "" {
				if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tt.errorContains)) {
					t.Errorf("Expected error containing %q, got %q", tt.errorContains, err.Error())
				}
				return
			}

			if err == nil {
				defer listener.Close()
				if tt.checkListener != nil {
					tt.checkListener(t, listener)
				}
			}

			// Clean up any UDS files
			if tt.useUnixSocket && tt.unixSocketPath != "" && !tt.wantErr {
				os.Remove(tt.unixSocketPath)
			}
		})
	}
}
