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
 */

package lrsclient

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	"google.golang.org/grpc/internal/backoff"
	"google.golang.org/grpc/internal/pretty"
	"google.golang.org/grpc/internal/xds/clients"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"

	v3corepb "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	v3endpointpb "github.com/envoyproxy/go-control-plane/envoy/config/endpoint/v3"
	v3lrspb "github.com/envoyproxy/go-control-plane/envoy/service/load_stats/v3"
)

// streamImpl provides all the functionality associated with an LRS (Load
// Reporting Service) stream on the client-side. It manages the lifecycle of
// the LRS stream, including starting, stopping, and retrying the stream. It
// also provides a LoadStore that can be used to report load, with a Stop
// function that should be called when the load reporting is no longer
// needed.
type streamImpl struct {
	// The following fields are initialized when a stream instance is created
	// and are read-only afterwards, and hence can be accessed without a mutex.
	transport clients.Transport       // Transport to use for LRS stream.
	backoff   func(int) time.Duration // Backoff for retries, after stream failures.
	nodeProto *v3corepb.Node          // Identifies the gRPC application.
	doneCh    chan struct{}           // To notify exit of LRS goroutine.
	logger    *slog.Logger

	cancelStream context.CancelFunc // Cancel the stream. If nil, the stream is not active.
	loadStore    *LoadStore         // LoadStore returned to user for pushing loads.

	finalSendRequest chan struct{} // To request for the final attempt to send loads.
	finalSendDone    chan error    // To signal completion of the final attempt of sending loads.
}

// streamOpts holds the options for creating an lrsStream.
type streamOpts struct {
	transport clients.Transport       // xDS transport to create the stream on.
	backoff   func(int) time.Duration // Backoff for retries, after stream failures.
	nodeProto *v3corepb.Node          // Node proto to identify the gRPC application.
	logger    *slog.Logger            // Logger to use for logging.
}

// newStreamImpl creates a new StreamImpl with the provided options.
//
// The actual streaming RPC call is initiated when the first call to ReportLoad
// is made, and is terminated when the last call to ReportLoad is canceled.
func newStreamImpl(opts streamOpts) *streamImpl {
	ctx, cancel := context.WithCancel(context.Background())

	lrs := &streamImpl{
		transport:        opts.transport,
		backoff:          opts.backoff,
		nodeProto:        opts.nodeProto,
		cancelStream:     cancel,
		doneCh:           make(chan struct{}),
		finalSendRequest: make(chan struct{}, 1),
		finalSendDone:    make(chan error, 1),
	}

	if opts.logger == nil {
		opts.logger = slog.Default()
	}
	lrs.logger = opts.logger.With("lrs-stream", fmt.Sprintf("%p", lrs))
	lrs.loadStore = newLoadStore()
	go lrs.runner(ctx)
	return lrs
}

// runner is responsible for managing the lifetime of an LRS streaming call. It
// creates the stream, sends the initial LoadStatsRequest, receives the first
// LoadStatsResponse, and then starts a goroutine to periodically send
// LoadStatsRequests. The runner will restart the stream if it encounters any
// errors.
func (lrs *streamImpl) runner(ctx context.Context) {
	defer close(lrs.doneCh)

	// This feature indicates that the client supports the
	// LoadStatsResponse.send_all_clusters field in the LRS response.
	node := proto.Clone(lrs.nodeProto).(*v3corepb.Node)
	node.ClientFeatures = append(node.ClientFeatures, "envoy.lrs.supports_send_all_clusters")

	runLoadReportStream := func() error {
		// streamCtx is created and canceled in case we terminate the stream
		// early for any reason, to avoid gRPC-Go leaking the RPC's monitoring
		// goroutine.
		streamCtx, cancel := context.WithCancel(ctx)
		defer cancel()

		stream, err := lrs.transport.NewStream(streamCtx, "/envoy.service.load_stats.v3.LoadReportingService/StreamLoadStats")
		if err != nil {
			lrs.logger.Warn("Failed to create new LRS streaming RPC", "error", err)
			return nil
		}
		lrs.logger.Debug("LRS stream created")

		if err := lrs.sendFirstLoadStatsRequest(stream, node); err != nil {
			lrs.logger.Warn("Sending first LRS request failed", "error", err)
			return nil
		}

		clusters, interval, err := lrs.recvFirstLoadStatsResponse(stream)
		if err != nil {
			lrs.logger.Warn("Reading from LRS streaming RPC failed", "error", err)
			return nil
		}

		// We reset backoff state when we successfully receive at least one
		// message from the server.
		lrs.sendLoads(streamCtx, stream, clusters, interval)
		return backoff.ErrResetBackoff
	}
	backoff.RunF(ctx, runLoadReportStream, lrs.backoff)
}

// sendLoads is responsible for periodically sending load reports to the LRS
// server at the specified interval for the specified clusters, until the passed
// in context is canceled.
func (lrs *streamImpl) sendLoads(ctx context.Context, stream clients.Stream, clusterNames []string, interval time.Duration) {
	tick := time.NewTicker(interval)
	defer tick.Stop()
	for {
		select {
		case <-tick.C:
		case <-ctx.Done():
			return
		case <-lrs.finalSendRequest:
			var finalSendErr error
			lrs.logger.Debug("Final send request received. Attempting final LRS report.")
			if err := lrs.sendLoadStatsRequest(stream, lrs.loadStore.stats(clusterNames)); err != nil {
				lrs.logger.Warn("Failed to send final load report. Writing to LRS stream failed", "error", err)
				finalSendErr = err
			}
			lrs.logger.Debug("Successfully sent final load report.")
			lrs.finalSendDone <- finalSendErr
			return
		}

		if err := lrs.sendLoadStatsRequest(stream, lrs.loadStore.stats(clusterNames)); err != nil {
			lrs.logger.Warn("Failed to send periodic load report. Writing to LRS stream failed", "error", err)
			return
		}
	}
}

func (lrs *streamImpl) sendFirstLoadStatsRequest(stream clients.Stream, node *v3corepb.Node) error {
	req := &v3lrspb.LoadStatsRequest{Node: node}
	lrs.logger.Debug("Sending initial LoadStatsRequest", "raw", pretty.ToJSON(req))
	msg, err := proto.Marshal(req)
	if err != nil {
		lrs.logger.Warn("Failed to marshal LoadStatsRequest", "error", err)
		return err
	}
	err = stream.Send(msg)
	if err == io.EOF {
		return getStreamError(stream)
	}
	return err
}

// recvFirstLoadStatsResponse receives the first LoadStatsResponse from the LRS
// server.  Returns the following:
//   - a list of cluster names requested by the server or an empty slice if the
//     server requested for load from all clusters
//   - the load reporting interval, and
//   - any error encountered
func (lrs *streamImpl) recvFirstLoadStatsResponse(stream clients.Stream) ([]string, time.Duration, error) {
	r, err := stream.Recv()
	if err != nil {
		return nil, 0, fmt.Errorf("lrs: failed to receive first LoadStatsResponse: %v", err)
	}
	var resp v3lrspb.LoadStatsResponse
	if err := proto.Unmarshal(r, &resp); err != nil {
		lrs.logger.Debug("Failed to unmarshal response to LoadStatsResponse", "error", err)
		return nil, time.Duration(0), fmt.Errorf("lrs: unexpected message type %T", r)
	}
	lrs.logger.Debug("Received first LoadStatsResponse", "raw", pretty.ToJSON(&resp))

	internal := resp.GetLoadReportingInterval()
	if internal.CheckValid() != nil {
		return nil, 0, fmt.Errorf("lrs: invalid load_reporting_interval: %v", err)
	}
	loadReportingInterval := internal.AsDuration()

	clusters := resp.Clusters
	if resp.SendAllClusters {
		// Return an empty slice to send stats for all clusters.
		clusters = []string{}
	}

	return clusters, loadReportingInterval, nil
}

func (lrs *streamImpl) sendLoadStatsRequest(stream clients.Stream, loads []*loadData) error {
	clusterStats := make([]*v3endpointpb.ClusterStats, 0, len(loads))
	for _, sd := range loads {
		droppedReqs := make([]*v3endpointpb.ClusterStats_DroppedRequests, 0, len(sd.drops))
		for category, count := range sd.drops {
			droppedReqs = append(droppedReqs, &v3endpointpb.ClusterStats_DroppedRequests{
				Category:     category,
				DroppedCount: count,
			})
		}
		localityStats := make([]*v3endpointpb.UpstreamLocalityStats, 0, len(sd.localityStats))
		for lid, localityData := range sd.localityStats {
			loadMetricStats := make([]*v3endpointpb.EndpointLoadMetricStats, 0, len(localityData.loadStats))
			for name, loadData := range localityData.loadStats {
				loadMetricStats = append(loadMetricStats, &v3endpointpb.EndpointLoadMetricStats{
					MetricName:                    name,
					NumRequestsFinishedWithMetric: loadData.count,
					TotalMetricValue:              loadData.sum,
				})
			}
			localityStats = append(localityStats, &v3endpointpb.UpstreamLocalityStats{
				Locality: &v3corepb.Locality{
					Region:  lid.Region,
					Zone:    lid.Zone,
					SubZone: lid.SubZone,
				},
				TotalSuccessfulRequests: localityData.requestStats.succeeded,
				TotalRequestsInProgress: localityData.requestStats.inProgress,
				TotalErrorRequests:      localityData.requestStats.errored,
				TotalIssuedRequests:     localityData.requestStats.issued,
				LoadMetricStats:         loadMetricStats,
				UpstreamEndpointStats:   nil, // TODO: populate for per endpoint loads.
			})
		}

		clusterStats = append(clusterStats, &v3endpointpb.ClusterStats{
			ClusterName:           sd.cluster,
			ClusterServiceName:    sd.service,
			UpstreamLocalityStats: localityStats,
			TotalDroppedRequests:  sd.totalDrops,
			DroppedRequests:       droppedReqs,
			LoadReportInterval:    durationpb.New(sd.reportInterval),
		})
	}

	req := &v3lrspb.LoadStatsRequest{ClusterStats: clusterStats}
	lrs.logger.Debug("Sending LRS loads", "raw", pretty.ToJSON(req))
	msg, err := proto.Marshal(req)
	if err != nil {
		lrs.logger.Debug("Failed to marshal LoadStatsRequest", "error", err)
		return err
	}
	err = stream.Send(msg)
	if err == io.EOF {
		return getStreamError(stream)
	}
	return err
}

func getStreamError(stream clients.Stream) error {
	for {
		if _, err := stream.Recv(); err != nil {
			return err
		}
	}
}
