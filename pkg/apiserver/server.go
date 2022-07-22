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

package apiserver

import (
	"net/http"

	"github.com/gorilla/mux"
	"k8s.io/client-go/kubernetes"

	"github.com/appvia/terranetes-controller/pkg/apiserver/logging"
	"github.com/appvia/terranetes-controller/pkg/apiserver/recovery"
)

// Server is the api server
type Server struct {
	// Client is the controller-runtime client
	Client kubernetes.Interface
	// Namespace is the kubernetes namespace where the jobs are run
	Namespace string
}

// Serve returns the http handler: is externally facing and called from the user namespace
// to retrieve the logs from the builds
func (s *Server) Serve() http.Handler {
	router := mux.NewRouter()
	router.Use(recovery.Recovery())
	router.Use(logging.Logger())

	router.HandleFunc("/healthz", s.handleHealth).Methods(http.MethodGet)
	router.HandleFunc("/v1/builds/{namespace}/{name}/logs", s.handleBuilds).Methods(http.MethodGet)

	return router
}
