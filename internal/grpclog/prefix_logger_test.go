/*
 *
 * Copyright 2024 gRPC authors.
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

package grpclog

import (
	"strings"
	"testing"

	"google.golang.org/grpc/grpclog"
)

type testLogger struct {
	grpclog.DepthLoggerV2
	info    []string
	warning []string
	err     []string
}

func (l *testLogger) InfoDepth(depth int, args ...any) {
	l.info = append(l.info, args[0].(string))
}

func (l *testLogger) WarningDepth(depth int, args ...any) {
	l.warning = append(l.warning, args[0].(string))
}

func (l *testLogger) ErrorDepth(depth int, args ...any) {
	l.err = append(l.err, args[0].(string))
}

func (l *testLogger) V(level int) bool {
	return true
}

func TestPrefixLogger_SlogLogger(t *testing.T) {
	tl := &testLogger{}
	pl := NewPrefixLogger(tl, "[prefix] ")
	sl := pl.SlogLogger()

	sl.Info("info message")
	if len(tl.info) != 1 || tl.info[0] != "[prefix] info message" {
		t.Errorf("Expected info message '[prefix] info message', got %v", tl.info)
	}

	sl.Warn("warn message")
	if len(tl.warning) != 1 || tl.warning[0] != "[prefix] warn message" {
		t.Errorf("Expected warn message '[prefix] warn message', got %v", tl.warning)
	}

	sl.Error("error message")
	if len(tl.err) != 1 || tl.err[0] != "[prefix] error message" {
		t.Errorf("Expected error message '[prefix] error message', got %v", tl.err)
	}

	sl.Info("message with attrs", "key", "val")
	if len(tl.info) != 2 || !strings.Contains(tl.info[1], "key=val") {
		t.Errorf("Expected message with attrs to contain 'key=val', got %v", tl.info[1])
	}
}
