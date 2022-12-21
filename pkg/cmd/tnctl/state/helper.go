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

package state

import (
	"regexp"
	"strings"

	v1 "k8s.io/api/core/v1"
)

// SecretPrefixes is a list of secret prefixes the controller uses
var SecretPrefixes = []string{
	"tfstate-default-",
	"config-",
	"policy-",
	"cost-",
}

// ConfigurationSecretRegex is the regex for a configuration secret
var ConfigurationSecretRegex = regexp.MustCompile(
	`^(tfstate-default|config|policy|cost)-(([a-z0-9]){8}-([a-z0-9]){4}-([a-z0-9]){4}-([a-z0-9]){4}-([a-z0-9]){12})$`,
)

// findSecretByPrefix is used to find a secret by prefix and uuid
func findSecretByPrefix(prefix, id string, secrets *v1.SecretList) (string, bool) {
	for _, x := range secrets.Items {
		switch {
		case !strings.HasPrefix(x.Name, prefix):
			continue
		case !strings.HasSuffix(x.Name, id):
			continue
		}

		return x.Name, true
	}

	return "", false
}
