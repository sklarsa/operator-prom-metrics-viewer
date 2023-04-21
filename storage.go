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

type InMemoryMetricStorage struct {
	appender storage.Appender
}

type DataPoint struct {
	Labels    labels.Labels
	Timestamp int64
	Value     float64
}

type InMemoryAppender struct {
	data map[uint64]DataPoint
	mu   *sync.Mutex
}

func (s InMemoryMetricStorage) Appender(ctx context.Context) storage.Appender {
	return s.appender
}

func (a *InMemoryAppender) Append(ref storage.SeriesRef, l labels.Labels, t int64, v float64) (storage.SeriesRef, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.data[l.Hash()] = DataPoint{Labels: l, Timestamp: t, Value: v}
	return ref, nil
}
func (a *InMemoryAppender) Commit() error {
	return nil
}
func (a *InMemoryAppender) Rollback() error {
	return nil
}
func (a *InMemoryAppender) AppendExemplar(ref storage.SeriesRef, l labels.Labels, e exemplar.Exemplar) (storage.SeriesRef, error) {
	println("exemplar", l.String(), e.Ts, e.Value, e.Labels.String())
	return ref, nil
}
func (a *InMemoryAppender) AppendHistogram(ref storage.SeriesRef, l labels.Labels, t int64, h *histogram.Histogram, fh *histogram.FloatHistogram) (storage.SeriesRef, error) {
	println("histogram", l.String(), t, h, fh)
	return ref, nil
}
func (a *InMemoryAppender) UpdateMetadata(ref storage.SeriesRef, l labels.Labels, m metadata.Metadata) (storage.SeriesRef, error) {
	println("metadata", l.String(), m.Help, m.Type, m.Unit)
	return ref, nil
}

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
