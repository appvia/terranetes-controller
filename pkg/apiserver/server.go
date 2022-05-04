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
)

// Server is the api server
type Server struct {
	// Client is the controller-runtime client
	Client kubernetes.Interface
	// Namespace is the kubernetes namespace where the jobs are run
	Namespace string
}

// ServerHTTP returns the http handler
func (s *Server) ServerHTTP() http.Handler {
	router := mux.NewRouter()
	router.HandleFunc("/builds", s.handleBuilds).Methods(http.MethodGet)
	router.HandleFunc("/healthz", s.handleHealth).Methods(http.MethodGet)

	return router
}
