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

import (
	"context"
)

// fakeStream is a fake stream for testing.
type fakeStream struct {
}

func (f *fakeStream) Send(p []byte) error {
	return nil
}

func (f *fakeStream) Recv() ([]byte, error) {
	return nil, nil
}

// fakeTransport is a fake transport for testing.
type fakeTransport struct {
}

func (f *fakeTransport) NewStream(ctx context.Context, method string) (Stream, error) {
	return &fakeStream{}, nil
}

func (f *fakeTransport) Close() {
}

// fakeTransportBuilder is a fake transport builder for testing.
type fakeTransportBuilder struct {
}

func (f *fakeTransportBuilder) Build(serverID string) (Transport, error) {
	return &fakeTransport{}, nil
}