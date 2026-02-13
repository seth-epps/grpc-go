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
	"google.golang.org/protobuf/types/known/structpb"
)

// Locality is the locality of the xDS client.
type Locality struct {
	// Region is the region of the locality.
	Region string
	// Zone is the zone of the locality.
	Zone string
	// SubZone is the sub-zone of the locality.
	SubZone string
}

// Node is the identity of the xDS client.
type Node struct {
	// ID is the unique identifier of the xDS client.
	ID string
	// Cluster is the name of the cluster that the xDS client belongs to.
	Cluster string
	// Locality is the locality of the xDS client.
	Locality Locality
	// Metadata is the metadata of the xDS client.
	Metadata *structpb.Struct
	// UserAgentName is the name of the user agent.
	UserAgentName string
	// UserAgentVersion is the version of the user agent.
	UserAgentVersion string
	// ClientFeatures is the list of client features.
	ClientFeatures []string
}


// ServerIdentifier is the identifier for an xDS server.
type ServerIdentifier struct {
	// ServerURI is the URI of the server.
	ServerURI string
}

// MetricsReporter is the interface for reporting metrics.
type MetricsReporter interface {
	// ReportMetric reports a metric.
	ReportMetric(metric interface{})
}
