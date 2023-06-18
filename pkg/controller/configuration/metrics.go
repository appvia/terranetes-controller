/*
 * Copyright (C) 2022  Appvia Ltd <info@appvia.io>
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

package configuration

import (
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

func init() {
	metrics.Registry.MustRegister(
		hourlyCostMetric,
		inSyncMetric,
		monthlyCostMetric,
		statusMetric,
	)
}

var (
	statusMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "configuration_status",
			Help: "Indicates the status of the configuration, 0 = OK, 1 = Error",
		}, []string{"name", "namespace"},
	)
	hourlyCostMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "configuration_hourly_cost_total",
			Help: "The hourly costs of a configuration currently in the system",
		}, []string{"name", "namespace"},
	)
	monthlyCostMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "configuration_monthly_cost_total",
			Help: "The monthly costs of a configuration currently in the system",
		}, []string{"name", "namespace"},
	)
	inSyncMetric = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "configuration_synchronized",
			Help: "Indicates the resources from the configuration are in sync",
		}, []string{"name", "namespace"},
	)
)
