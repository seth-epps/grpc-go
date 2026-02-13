/*
 *
 * Copyright 2026 gRPC authors.
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

import "context"

// Stream is the interface for a stream to the xDS management server.
type Stream interface {
	// Send sends a message on the stream.
	Send([]byte) error
	// Recv receives a message from the stream.
	Recv() ([]byte, error)
}

// Transport is the interface for the transport used to connect to the xDS
// management server.
type Transport interface {
	// NewStream creates a new stream.
	NewStream(ctx context.Context, method string) (Stream, error)
	// Close closes the transport.
	Close()
}

// TransportBuilder builds a transport.
type TransportBuilder interface {
	// Build creates a new transport.
	Build(serverID string) (Transport, error)
}