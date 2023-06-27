package main

import (
	"context"
	"sort"
	"sync"

	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/storage"
)

// InMemoryMetricStorage implements the storage.Appendable and storage.Querier interfaces
// This stores no metric history, only the latest scraped value for each metric in memory
type InMemoryMetricStorage struct {
	appender storage.Appender
}

// DataPoint represents a single metric data point. These are stored in the InMemoryAppender
type DataPoint struct {
	Labels    labels.Labels
	Timestamp int64
	Value     float64
}

// InMemoryAppender implements the storage.Appendable interface and stores only the latest
// DataPoint per metric. It also implements a Query method to retrieve the latest DataPoint
type InMemoryAppender struct {
	data map[uint64]DataPoint
	mu   *sync.Mutex
}

// Appender returns a new InMemoryMetricStorage
func (s InMemoryMetricStorage) Appender(ctx context.Context) storage.Appender {
	return s.appender
}

// Append stores a new DataPoint
func (a *InMemoryAppender) Append(ref storage.SeriesRef, l labels.Labels, t int64, v float64) (storage.SeriesRef, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.data[l.Hash()] = DataPoint{Labels: l, Timestamp: t, Value: v}
	return ref, nil
}

// Commit is a no-op
func (a *InMemoryAppender) Commit() error {
	return nil
}

// Rollback is a no-op
func (a *InMemoryAppender) Rollback() error {
	return nil
}

// AppendExemplar is a no-op. This doesn't seem to be used by the operator metrics endpoint so it is not implemented
func (a *InMemoryAppender) AppendExemplar(ref storage.SeriesRef, l labels.Labels, e exemplar.Exemplar) (storage.SeriesRef, error) {
	println("exemplar", l.String(), e.Ts, e.Value, e.Labels.String())
	return ref, nil
}

// AppendHistogram is a no-op. This doesn't seem to be used by the operator metrics endpoint so it is not implemented
func (a *InMemoryAppender) AppendHistogram(ref storage.SeriesRef, l labels.Labels, t int64, h *histogram.Histogram, fh *histogram.FloatHistogram) (storage.SeriesRef, error) {
	println("histogram", l.String(), t, h, fh)
	return ref, nil
}

// UpdateMetadata is a no-op. This doesn't seem to be used by the operator metrics endpoint so it is not implemented
func (a *InMemoryAppender) UpdateMetadata(ref storage.SeriesRef, l labels.Labels, m metadata.Metadata) (storage.SeriesRef, error) {
	println("metadata", l.String(), m.Help, m.Type, m.Unit)
	return ref, nil
}

// Query returns the latest DataPoint for the given metric and labels
func (a *InMemoryAppender) Query(metric string, l labels.Labels) []DataPoint {
	a.mu.Lock()
	defer a.mu.Unlock()

	dataToReturn := []DataPoint{}

	for _, d := range a.data {
		if d.Labels.Get("__name__") != metric {
			continue
		}

		var isMatch bool = true
		for _, label := range l {
			if d.Labels.Get(label.Name) != label.Value {
				isMatch = false
			}
		}

		if isMatch {
			dataToReturn = append(dataToReturn, d)
		}

	}

	return dataToReturn
}

// Controllers returns a list of all the controller names that have been scraped
func (a *InMemoryAppender) Controllers() []string {
	controllers := map[string]bool{}

	a.mu.Lock()
	defer a.mu.Unlock()

	for _, d := range a.data {
		name := d.Labels.Get("name")
		if name != "" {
			controllers[name] = true
		}
	}

	toReturn := []string{}
	for k := range controllers {
		toReturn = append(toReturn, k)
	}

	sort.Strings(toReturn)

	return toReturn

}
