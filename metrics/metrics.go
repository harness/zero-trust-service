// Copyright 2026 Harness, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package metrics

// Dimension is a key-value pair attached to a metric observation.
type Dimension struct {
	Key   string
	Value string
}

// Dim is a shorthand constructor for a Dimension.
func Dim(key, value string) Dimension {
	return Dimension{Key: key, Value: value}
}

// Emitter is the interface for recording metrics. Packages define their own
// metric names and dimensions; the metrics package only provides this interface.
type Emitter interface {
	Counter(name string, value float64, dims ...Dimension)
	Histogram(name string, value float64, dims ...Dimension)
	Gauge(name string, value float64, dims ...Dimension)
}
