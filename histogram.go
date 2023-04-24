package main

import (
	"errors"
	"math"
	"sort"
	"strconv"

	"github.com/prometheus/prometheus/model/labels"
)

type HistogramData struct {
	buckets []Bucket
	curIdx  int
}

func (d HistogramData) Max() int {
	var max int
	for _, b := range d.buckets {
		if b.Value > max {
			max = b.Value
		}
	}

	return max
}

type Bucket struct {
	Label string
	Value int
}

func (b1 Bucket) Less(b2 Bucket) (bool, error) {
	var (
		b1Val, b2Val float64
		err          error
	)

	if b1.Label == "+Inf" {
		b1Val = math.MaxFloat64
	} else {
		b1Val, err = strconv.ParseFloat(b1.Label, 64)
		if err != nil {
			return false, err
		}
	}

	if b2.Label == "+Inf" {
		b2Val = math.MaxFloat64
	} else {
		b2Val, err = strconv.ParseFloat(b2.Label, 64)
		if err != nil {
			return false, err
		}
	}

	return b1Val < b2Val, nil

}

func FromDataPoint(d DataPoint) (Bucket, error) {
	var (
		leLabel *labels.Label
		b       Bucket
		err     error
	)

	for i := range d.Labels {
		if d.Labels[i].Name == "le" {
			leLabel = &d.Labels[i]
		}
	}

	if leLabel == nil {
		return b, errors.New("no le label found")
	}

	b.Label = leLabel.Value
	b.Value = int(d.Value)

	return b, err

}

func (d *HistogramData) Reset() {
	d.curIdx = 0
}

func (d *HistogramData) HasNext() bool {
	return d.curIdx < (len(d.buckets) - 1)
}

func (d *HistogramData) Next() Bucket {

	var bucket Bucket
	for bucket.Value == 0 {
		if !d.HasNext() {
			return Bucket{}
		}
		bucket = d.buckets[d.curIdx]
		d.curIdx++
	}
	return bucket
}

func (d *HistogramData) BucketCount() int {
	return len(d.buckets)
}

func NewHistogramData(appender *InMemoryAppender, metric, controller string) (*HistogramData, error) {
	histData := appender.Query(
		metric,
		[]labels.Label{
			{
				Name:  "controller",
				Value: controller,
			},
		},
	)

	data := []Bucket{}

	for _, d := range histData {
		b, err := FromDataPoint(d)
		if err != nil {
			return nil, err
		}
		data = append(data, b)
	}

	sort.Slice(data, func(i, j int) bool {
		b1 := data[i]
		b2 := data[j]
		less, err := b1.Less(b2)
		if err != nil {
			panic(err)
		}
		return less
	})

	return &HistogramData{
		buckets: data,
	}, nil
}
