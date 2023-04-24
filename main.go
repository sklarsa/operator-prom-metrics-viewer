package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery/targetgroup"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/scrape"
	"github.com/rivo/tview"
)

var tableMetricsToShow = []string{
	"controller_runtime_active_workers",
	"controller_runtime_max_concurrent_reconciles",
	"controller_runtime_reconcile_errors_total",
	"controller_runtime_reconcile_time_seconds_count",
	"workqueue_depth",
	"workqueue_longest_running_processor_seconds",
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
			data: make(map[uint64]DataPoint),
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
	table := tview.NewTable().SetBorders(true)

	go func() {
		for range ticker.C {

			appender := storage.appender.(*InMemoryAppender)
			controllers := appender.Controllers()

			// Set headers
			table.SetCell(0, 0, tview.NewTableCell(""))
			for c, name := range controllers {
				table.SetCell(0, c+1, tview.NewTableCell(name))
			}

			for r, metric := range tableMetricsToShow {
				table.SetCell(r+1, 0, tview.NewTableCell(metric))
				for c, controller := range controllers {
					var val string

					var queryLabels []labels.Label
					if strings.HasPrefix(metric, "workqueue") {
						queryLabels = append(queryLabels, labels.Label{Name: "name", Value: controller})
					} else {
						queryLabels = append(queryLabels, labels.Label{Name: "controller", Value: controller})
					}

					results := appender.Query(metric, queryLabels)
					if len(results) == 1 {
						val = strconv.FormatFloat(results[0].Value, 'f', 0, 64)
					}
					table.SetCell(r+1, c+1, tview.NewTableCell(val))
				}

			}

			app.Draw()
		}
	}()

	app.SetRoot(table, true).SetFocus(table).Run()

}
