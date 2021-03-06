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

package ingester

import (
	"sync"

	"github.com/digitalocean/vulcan/bus"

	log "github.com/Sirupsen/logrus"
)

// Ingester consumes TimeSeriesBatch from a source and writes them at a
// specified level of concurrency.
type Ingester struct {
	NumWorkers int
	Source     bus.Source
	Writer     Writer
}

// Run executes until an error occurs.
func (i *Ingester) Run() error {
	log.WithField("num_workers", i.NumWorkers).Info("starting workers")

	var (
		once     sync.Once
		outerErr error
		wg       sync.WaitGroup
	)
	done := make(chan struct{})
	ch := i.Source.Messages()
	wg.Add(i.NumWorkers)
	for n := 0; n < i.NumWorkers; n++ {
		go func() {
			defer wg.Done()
			err := work(done, ch, i.Writer)
			if err != nil {
				once.Do(func() {
					close(done)
					outerErr = err
				})
			}
		}()
	}
	wg.Wait()
	// return error that caused a worker to fail
	if outerErr != nil {
		return outerErr
	}
	// return error that caused Source to stop
	return i.Source.Error()
}

func work(done <-chan struct{}, ch <-chan *bus.SourcePayload, w Writer) error {
	for {
		select {
		case <-done:
			return nil
		case m, ok := <-ch:
			if !ok {
				return nil
			}
			err := w.Write(m.TimeSeriesBatch)
			if err != nil {
				return err
			}
			m.Ack()
		}
	}
}
