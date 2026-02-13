/*
 *
 * Copyright 2025 gRPC authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package xdsclient

import (
	"strings"
	"testing"
	"time"

	"github.com/sepps/xdsclient/internal/xdsresource"
)

var (
	// ResourceWatchStateForTesting gets the watch state for the resource
	// identified by the given resource type and resource name. Returns a
	// non-nil error if there is no such resource being watched.
	ResourceWatchStateForTesting func(*XDSClient, ResourceType, string) (xdsresource.ResourceWatchState, error)
)

var listenerType = ResourceType{
	TypeURL: "type.googleapis.com/envoy.config.listener.v3.Listener",
}

type s struct{}

func (s) TestXDSClient_New(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr string
	}{
		{
			name: "nil resource types",
			config: Config{
				Node: Node{ID: "node-id"},
			},
			wantErr: "resource types map is nil",
		},
		{
			name: "nil transport builder",
			config: Config{
				Node:          Node{ID: "node-id"},
				ResourceTypes: map[string]ResourceType{xdsresource.V3ListenerURL: listenerType},
			},
			wantErr: "transport builder is nil",
		},
		{
			name: "no servers or authorities",
			config: Config{
				Node:             Node{ID: "node-id"},
				ResourceTypes:    map[string]ResourceType{xdsresource.V3ListenerURL: listenerType},
				TransportBuilder: &fakeTransportBuilder{},
			},
			wantErr: "no servers or authorities specified",
		},
		{
			name: "success with servers",
			config: Config{
				Node:             Node{ID: "node-id"},
				ResourceTypes:    map[string]ResourceType{xdsresource.V3ListenerURL: listenerType},
				TransportBuilder: &fakeTransportBuilder{},
				Servers:          []ServerConfig{{ServerIdentifier: ServerIdentifier{ServerURI: "dummy-server"}}},
			},
			wantErr: "",
		},
		{
			name: "success with servers and empty nodeID",
			config: Config{
				Node:             Node{ID: ""},
				ResourceTypes:    map[string]ResourceType{xdsresource.V3ListenerURL: listenerType},
				TransportBuilder: &fakeTransportBuilder{},
				Servers:          []ServerConfig{{ServerIdentifier: ServerIdentifier{ServerURI: "dummy-server"}}},
			},
			wantErr: "",
		},
		{
			name: "success with authorities",
			config: Config{
				Node:             Node{ID: "node-id"},
				ResourceTypes:    map[string]ResourceType{xdsresource.V3ListenerURL: listenerType},
				TransportBuilder: &fakeTransportBuilder{},
				Authorities:      map[string]Authority{"authority-name": {XDSServers: []ServerConfig{{ServerIdentifier: ServerIdentifier{ServerURI: "dummy-server"}}}}},
			},
			wantErr: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := New(tt.config)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("New(%+v) failed: %v", tt.config, err)
				}
			} else {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("New(%+v) returned error %v, want error %q", tt.config, err, tt.wantErr)
				}
			}
			if c != nil {
				c.Close()
			}
		})
	}
}

func (s) TestXDSClient_Close(t *testing.T) {
	config := Config{
		Node:             Node{ID: "node-id"},
		ResourceTypes:    map[string]ResourceType{xdsresource.V3ListenerURL: listenerType},
		TransportBuilder: &fakeTransportBuilder{},
		Servers:          []ServerConfig{{ServerIdentifier: ServerIdentifier{ServerURI: "dummy-server"}}},
	}
	c, err := New(config)
	if err != nil {
		t.Fatalf("New(%+v) failed: %v", config, err)
	}
	c.Close()
	// Calling close again should not panic.
	c.Close()
}