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

package create

import (
	"bytes"
	"encoding/json"

	terraformv1alpha1 "github.com/appvia/terranetes-controller/pkg/apis/terraform/v1alpha1"
	"github.com/appvia/terranetes-controller/pkg/utils/similarity"
)

// SuggestContextualInput is responsible for suggesting contextual input based on the
// current state of the cluster
func SuggestContextualInput(input string, list *terraformv1alpha1.ContextList, min float64) (Input, bool) {
	switch {
	case list == nil:
		return Input{}, false
	case len(list.Items) == 0:
		return Input{}, false
	}

	var scores []similarity.Score
	options := make(map[string]map[string]interface{})

	for _, x := range list.Items {
		// @step: we start by building a collection of suggestions from the contexts
		for name, variable := range x.Spec.Variables {
			item := make(map[string]interface{})
			if err := json.NewDecoder(bytes.NewReader(variable.Raw)).Decode(&item); err != nil {
				return Input{}, false
			}
			item["name"] = name
			item["context"] = x.Name

			// @step: we grab the description if there is one
			description, ok := item["description"].(string)
			if !ok {
				continue
			}
			options[description] = item
		}

		var suggestions []string
		for description := range options {
			suggestions = append(suggestions, description)
		}

		// @step: now we have the options
		hist := similarity.Closeness(input, suggestions, similarity.Filter{Min: min, TopN: 1})
		if len(hist.Scores) > 0 {
			scores = append(scores, hist.Scores...)
		}
	}

	switch len(scores) {
	// if we didn't get any suggestions, we return
	case 0:
		return Input{}, false
	case 1:
		variable := options[scores[0].Input]

		return Input{
			Context:     variable["context"].(string),
			Description: variable["description"].(string),
			Key:         variable["name"].(string),
		}, true
	}

	variable := options[scores[0].Input]

	return Input{
		Context:     variable["context"].(string),
		Description: variable["description"].(string),
		Key:         variable["name"].(string),
	}, true
}
