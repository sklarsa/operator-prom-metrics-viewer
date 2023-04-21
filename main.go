package main

import (
	"context"
	"fmt"
	"math"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery/targetgroup"
	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/scrape"
	"github.com/prometheus/prometheus/storage"
	"github.com/rivo/tview"
)

type InMemoryMetricStorage struct {
	appender storage.Appender
}

type RegularValue struct {
	Labels    labels.Labels
	Timestamp int64
	Value     float64
}

func (v RegularValue) String() string {
	var (
		metricName string
		labelPairs []string
	)
	for _, l := range v.Labels {
		switch l.Name {
		case labels.MetricName:
			metricName = l.Value
		case labels.InstanceName, "job":
			continue
		default:
			labelPairs = append(labelPairs, fmt.Sprintf("%s=%s", l.Name, l.Value))
		}
	}

	return fmt.Sprintf("%s{%s} %f", metricName, strings.Join(labelPairs, ","), v.Value)
}

type InMemoryAppender struct {
	data map[string]RegularValue
	mu   *sync.Mutex
}

func (s InMemoryMetricStorage) Appender(ctx context.Context) storage.Appender {
	return s.appender
}

func (a *InMemoryAppender) Append(ref storage.SeriesRef, l labels.Labels, t int64, v float64) (storage.SeriesRef, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.data[l.String()] = RegularValue{Labels: l, Timestamp: t, Value: v}
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

func main() {
	if len(os.Args) != 2 {
		panic("Usage: metrics-viewer <metrics-host>")
	}

	host := os.Args[1]
	scheme := "http"
	scrapeTimeout := time.Millisecond * 500 // todo: make this configurable
	scrapeInterval := time.Second * 1       // todo: make this configurable
	refreshInterval := time.Second * 1      // todo: make this configurable

	ticker := time.NewTicker(refreshInterval)

	cfg := &config.Config{
		ScrapeConfigs: []*config.ScrapeConfig{{
			Scheme:         scheme,
			MetricsPath:    "/metrics",
			JobName:        "metrics-viewer",
			ScrapeInterval: model.Duration(scrapeInterval),
			ScrapeTimeout:  model.Duration(scrapeTimeout),
		}},
	}

	storage := InMemoryMetricStorage{
		appender: &InMemoryAppender{
			data: make(map[string]RegularValue),
			mu:   &sync.Mutex{},
		},
	}

	mgr := scrape.NewManager(&scrape.Options{}, nil, storage)
	err := mgr.ApplyConfig(cfg)
	if err != nil {
		panic(err)
	}

	ts := make(chan map[string][]*targetgroup.Group)
	go mgr.Run(ts)

	defer mgr.Stop()

	res := labels.FromMap(map[string]string{
		model.AddressLabel:        host,
		model.InstanceLabel:       host,
		model.SchemeLabel:         scheme,
		model.MetricsPathLabel:    "/metrics",
		model.JobLabel:            "metrics-viewer",
		model.ScrapeIntervalLabel: scrapeInterval.String(),
		model.ScrapeTimeoutLabel:  scrapeTimeout.String(),
	})

	ls := model.LabelSet{}
	for _, l := range res {
		ls[model.LabelName(l.Name)] = model.LabelValue(l.Value)
	}

	ts <- map[string][]*targetgroup.Group{
		"metrics-viewer": {
			{
				Targets: []model.LabelSet{ls},
			},
		},
	}

	println("waiting to register target...")
	for len(mgr.TargetsActive()) == 0 {
		time.Sleep(250 * time.Millisecond)
	}

	tgt := mgr.TargetsAll()["metrics-viewer"][0]
	fmt.Printf("%s %s %s", tgt.LastScrape(), tgt.LastScrapeDuration(), tgt.Health())

	app := tview.NewApplication()
	list := tview.NewList()
	list.ShowSecondaryText(false)

	go func() {
		for range ticker.C {

			data := storage.appender.(*InMemoryAppender).data
			for k, v := range data {

				found := list.FindItems("dummy value", k, false, true)
				item := v.String()

				switch len(found) {
				case 0:
					if math.IsNaN(data[k].Value) {
						continue
					}

					if list.GetItemCount() == 0 {
						list.AddItem(item, k, 0, nil)
						continue
					}
					var added bool
					for i := 0; i < list.GetItemCount(); i++ {
						_, lk := list.GetItemText(i)
						if k < lk {
							list.InsertItem(i, item, k, 0, nil)
							added = true
							break
						}
					}
					if !added {
						list.AddItem(item, k, 0, nil)
					}

				case 1:
					if math.IsNaN(data[k].Value) {
						list.RemoveItem(found[0])
						continue
					}

					list.SetItemText(found[0], item, k)
				default:
					// todo: handle this better
					panic("found multiple items")

				}
			}

			app.Draw()
		}
	}()

	app.SetRoot(list, true).SetFocus(list).Run()

}
