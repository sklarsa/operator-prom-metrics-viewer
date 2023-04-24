package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/navidys/tvxwidgets"
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

	table := tview.NewTable()
	table.SetBorders(true).
		SetBorder(true).
		SetTitle("Overview")

	dropdown := tview.NewDropDown().SetFieldWidth(20).SetLabel("Controller:")

	reconcileTimeHist := tvxwidgets.NewBarChart()
	var reconcileTimeData *HistogramData

	histFlex := tview.NewFlex().SetDirection(tview.FlexColumn).AddItem(reconcileTimeHist, 0, 1, false)
	histFlex.SetBorder(true).
		SetTitle("Detail")

	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(table, 0, 1, false).
		AddItem(dropdown, 1, 0, false).
		AddItem(histFlex, 0, 2, false)

	go func() {
		for range ticker.C {

			appender := storage.appender.(*InMemoryAppender)
			controllers := appender.Controllers()

			// Controller dropdown
			if len(controllers) != dropdown.GetOptionCount() {
				dropdown.SetOptions(controllers, nil)
				if len(controllers) > 0 {
					dropdown.SetCurrentOption(0)
				}
			}

			// Overview metric table
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

			// Reconcile time histogram
			_, selectedController := dropdown.GetCurrentOption()
			histData, err := NewHistogramData(appender, "controller_runtime_reconcile_time_seconds_bucket", selectedController)
			if err != nil {
				panic(err)
			}

			var isNew bool
			if reconcileTimeData == nil || histData.BucketCount() != reconcileTimeData.BucketCount() {
				isNew = true

				histFlex.RemoveItem(reconcileTimeHist)
				reconcileTimeHist = tvxwidgets.NewBarChart()
				reconcileTimeHist.SetBorder(true)
				reconcileTimeHist.SetTitle("Reconcile Time")
				histFlex.AddItem(reconcileTimeHist, 0, 1, false)

			}

			for histData.HasNext() {
				b, err := histData.Next()
				if err != nil {
					panic(err)
				}
				if isNew {
					reconcileTimeHist.AddBar(b.Label, int(b.Value), tcell.ColorBlue)
				} else {
					reconcileTimeHist.SetBarValue(b.Label, int(b.Value))
				}

			}
			reconcileTimeHist.SetMaxValue(histData.Max())

			app.Draw()
		}
	}()

	if err := app.SetRoot(flex, true).SetFocus(dropdown).Run(); err != nil {
		panic(err)
	}

}
