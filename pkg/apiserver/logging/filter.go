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

package logging

import (
	"net/http"

	"github.com/felixge/httpsnoop"
	log "github.com/sirupsen/logrus"

	"github.com/gorilla/mux"
)

// Logger returns the middleware method for the router
func Logger() mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			m := httpsnoop.CaptureMetrics(next, w, req)

			log.WithFields(log.Fields{
				"bytes":    m.Written,
				"ip":       req.RemoteAddr,
				"method":   req.Method,
				"response": m.Code,
				"time":     m.Duration,
			}).Info("received api request")
		})
	}
}
