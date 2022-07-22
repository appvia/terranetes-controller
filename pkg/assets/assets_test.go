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

package assets

import (
	"html/template"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/appvia/terranetes-controller/pkg/utils"
)

func TestJobTemplateParsable(t *testing.T) {
	tl, err := Asset("job.yaml.tpl")
	assert.NoError(t, err)
	assert.NotEmpty(t, tl)

	tpl, err := template.New("main").Funcs(utils.GetTxtFunc()).Parse(string(tl))
	assert.NoError(t, err)
	assert.NotNil(t, tpl)
}

func TestAssetNames(t *testing.T) {
	assert.Equal(t, []string{"job.yaml.tpl"}, AssetNames())
}

func TestAsset(t *testing.T) {
	b, err := Asset("job.yaml.tpl")
	assert.NoError(t, err)
	assert.NotEmpty(t, b)

	b, err = Asset("not_there")
	assert.Error(t, err)
	assert.Empty(t, b)
}

func TestMustAssetNotThere(t *testing.T) {
	defer func() {
		assert.NotNil(t, recover())
	}()
	MustAsset("not_there")
}

func TestMustAsset(t *testing.T) {
	assert.NotEmpty(t, MustAsset("job.yaml.tpl"))
}
