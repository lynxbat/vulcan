// Copyright 2016 The Vulcan Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package indexer

import (
	"sync"
	"time"

	"github.com/digitalocean/vulcan/bus"
	"github.com/digitalocean/vulcan/storage"

	log "github.com/Sirupsen/logrus"
	"github.com/prometheus/client_golang/prometheus"
)

type workPayload struct {
	s  *bus.Sample
	wg *sync.WaitGroup
}

// Indexer represents an object that consumes metrics from a message bus and
// writes them to indexing system.
type Indexer struct {
	prometheus.Collector
	Source        bus.AckSource
	SampleIndexer storage.SampleIndexer

	indexDurations     *prometheus.SummaryVec
	errorsTotal        *prometheus.CounterVec
	work               chan workPayload
	numIndexGoroutines int
}

// Config represents the configuration of an Indexer.  It takes an implmenter
// Acksource of the target message bus and an implmenter of SampleIndexer of
// the target indexing system.
type Config struct {
	Source             bus.AckSource
	SampleIndexer      storage.SampleIndexer
	NumIndexGoroutines int
}

// NewIndexer creates a new instance of an Indexer.
func NewIndexer(config *Config) *Indexer {
	i := &Indexer{
		Source:        config.Source,
		SampleIndexer: config.SampleIndexer,

		indexDurations: prometheus.NewSummaryVec(
			prometheus.SummaryOpts{
				Namespace: "vulcan",
				Subsystem: "indexer",
				Name:      "duration_nanoseconds",
				Help:      "Durations of different indexer stages",
			},
			[]string{"stage"},
		),
		errorsTotal: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: "vulcan",
				Subsystem: "indexer",
				Name:      "errors_total",
				Help:      "Total number of errors of indexer stages",
			},
			[]string{"stage"},
		),
		work:               make(chan workPayload),
		numIndexGoroutines: config.NumIndexGoroutines,
	}
	for n := 0; n < i.numIndexGoroutines; n++ {
		go i.worker()
	}
	return i
}

// Describe implements prometheus.Collector.  Sends decriptors of the
// instance's indexDurations, SampleIndexer, and errorsTotal to the parameter ch.
func (i *Indexer) Describe(ch chan<- *prometheus.Desc) {
	i.indexDurations.Describe(ch)
	i.errorsTotal.Describe(ch)
	i.SampleIndexer.Describe(ch)
}

// Collect implements prometheus.Collector.  Sends metrics collected by the
// instance's indexDurations, SampleIndexer, and errorsTotal to the parameter ch.
func (i *Indexer) Collect(ch chan<- prometheus.Metric) {
	i.indexDurations.Collect(ch)
	i.errorsTotal.Collect(ch)
	i.SampleIndexer.Collect(ch)
}

// Run starts the indexer process of consuming from the bus and indexing to
// the target indexing system.
func (i *Indexer) Run() error {
	log.Info("running...")
	ch := i.Source.Chan()

	for payload := range ch {
		log.WithFields(log.Fields{
			"payload": payload.SampleGroup,
		}).Debug("distributing sample group to workers")

		i.indexSampleGroup(payload.SampleGroup)
		payload.Done(nil)
	}

	return i.Source.Err()
}

func (i *Indexer) indexSampleGroup(sg bus.SampleGroup) {
	var (
		t0 = time.Now()
		wg = &sync.WaitGroup{}
	)

	wg.Add(len(sg))

	for _, s := range sg {
		i.work <- workPayload{
			s:  s,
			wg: wg,
		}
	}

	wg.Wait()
	i.indexDurations.WithLabelValues("index_sample_group").Observe(float64(time.Since(t0).Nanoseconds()))
}

func (i *Indexer) worker() {
	for w := range i.work {
		var (
			t0 = time.Now()
			ll = log.WithFields(log.Fields{"sample": w.s})
		)

		ll.Debug("writing sample")

		err := i.SampleIndexer.IndexSample(w.s)
		w.wg.Done()
		if err != nil {
			ll.WithError(err).Error("could not write sample to index storage")

			i.errorsTotal.WithLabelValues("index_sample").Add(1)
			continue
		}

		i.indexDurations.WithLabelValues("index_sample").Observe(float64(time.Since(t0).Nanoseconds()))
	}
}
