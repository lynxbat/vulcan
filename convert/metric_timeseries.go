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

package convert

import (
	"github.com/digitalocean/vulcan/model"
	"github.com/prometheus/prometheus/storage/metric"
)

// MetricToTimeSeries converts the prometheus storage metric type to a Vulcan
// model TimeSeries.
func MetricToTimeSeries(m metric.Metric) model.TimeSeries {
	ts := model.TimeSeries{
		Labels:  make(map[string]string, len(m.Metric)),
		Samples: []*model.Sample{},
	}
	for k, v := range m.Metric {
		ts.Labels[string(k)] = string(v)
	}
	return ts
}
