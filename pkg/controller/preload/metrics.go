/*
 * Copyright (C) 2023  Appvia Ltd <info@appvia.io>
 *
 * This program is free software; you can redistribute it and/or
 * modify it under the terms of the GNU General Public License
 * as published by the Free Software Foundation; either version 2
 * of the License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package preload

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

func init() {
	metrics.Registry.MustRegister(
		totalSuccess,
		totalFailure,
	)
}

var (
	totalFailure = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "preload_failure_total",
			Help: "Total number of failures when preloading resources",
		},
	)

	totalSuccess = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "preload_success_total",
			Help: "Total number of successful preloads performed",
		},
	)
)
