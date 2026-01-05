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

package eks

import (
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/smithy-go"
	"github.com/stretchr/testify/assert"
	"k8s.io/utils/pointer"
)

func TestIsAWSError(t *testing.T) {
	assert.False(t, IsAWSError(errors.New(""), "code"))
	apiErr1 := &smithy.GenericAPIError{Code: "test", Message: "code"}
	assert.False(t, IsAWSError(apiErr1, "different_code"))
	apiErr2 := &smithy.GenericAPIError{Code: "code", Message: "test"}
	assert.True(t, IsAWSError(apiErr2, "code"))
}

func TestIsAWSErrorType(t *testing.T) {
	assert.False(t, IsAWSErrorType(nil))
	assert.False(t, IsAWSErrorType(errors.New("nope")))
	apiErr := &smithy.GenericAPIError{Code: "yes", Message: "yes"}
	assert.True(t, IsAWSErrorType(apiErr))
}

func TestSantize(t *testing.T) {
	assert.Equal(t, "foo", SanitizeName("foo"))
	assert.Equal(t, "fooname", SanitizeName("fooName"))
	assert.Equal(t, "foo_name", SanitizeName("foo-name"))
	assert.Equal(t, "foo_name", SanitizeName("foo_name"))
}

func TestToMapTags(t *testing.T) {
	assert.Equal(t, []map[string]interface{}{
		{
			"key":    "foo",
			"values": []string{"test"},
		},
	}, ToMapTags(map[string]string{
		"foo": "test",
	}))
}

func TestIsResourceNotFoundException(t *testing.T) {
	assert.False(t, IsResourceNotFoundException(nil))
	assert.False(t, IsResourceNotFoundException(errors.New("nope")))
	apiErr1 := &smithy.GenericAPIError{Code: "nope", Message: "nope"}
	assert.False(t, IsResourceNotFoundException(apiErr1))
	apiErr2 := &smithy.GenericAPIError{Code: "ResourceNotFoundException", Message: ""}
	assert.True(t, IsResourceNotFoundException(apiErr2))
}

func TestHasTag(t *testing.T) {
	tags := []types.Tag{
		{
			Key:   pointer.String("Environment"),
			Value: pointer.String("dev"),
		},
		{
			Key:   pointer.String("Name"),
			Value: pointer.String("test"),
		},
	}
	assert.False(t, HasTag(tags, "foo", "bar"))
	assert.False(t, HasTag(tags, "Environment", "bar"))
	assert.True(t, HasTag(tags, "Environment", "dev"))
	assert.True(t, HasTag(tags, "Envir.*", "dev"))
	assert.True(t, HasTag(tags, "Environment", ".*"))
	assert.True(t, HasTag(tags, "Name", "test"))
	assert.False(t, HasTag(tags, "Name", "nope"))
}

func TestGetTagValue(t *testing.T) {
	tags := []types.Tag{
		{
			Key:   pointer.String("Environment"),
			Value: pointer.String("dev"),
		},
		{
			Key:   pointer.String("Name"),
			Value: pointer.String("test"),
		},
	}
	value, found := GetTagValue(tags, "Name")
	assert.True(t, found)
	assert.Equal(t, "test", value)

	value, found = GetTagValue(tags, "Na.*")
	assert.True(t, found)
	assert.Equal(t, "test", value)

	value, found = GetTagValue(tags, "not_there")
	assert.False(t, found)
	assert.Equal(t, "", value)
}

func TestGetTag(t *testing.T) {
	tags := []types.Tag{
		{
			Key:   pointer.String("Environment"),
			Value: pointer.String("dev"),
		},
		{
			Key:   pointer.String("Name"),
			Value: pointer.String("test"),
		},
	}
	tag, found := GetTag(tags, "Name")
	assert.True(t, found)
	assert.NotNil(t, tag)

	tag, found = GetTag(tags, "Na.*")
	assert.True(t, found)
	assert.NotNil(t, tag)

	tag, found = GetTag(tags, "not_there.*")
	assert.False(t, found)
	assert.Nil(t, tag)
}
