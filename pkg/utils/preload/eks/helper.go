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
	"fmt"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/eks"
)

// SanitizeName sanitizes the given name
func SanitizeName(name string) string {
	return strings.ReplaceAll(strings.ToLower(name), "-", "_")
}

// IsAWSErrorType returns true if the given error is an AWS error
// nolint:errorlint
func IsAWSErrorType(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(awserr.Error)

	return ok
}

// IsAWSError returns true if the given error is an AWS error
// nolint:errorlint
func IsAWSError(err error, code string) bool {
	if err == nil || !IsAWSErrorType(err) {
		return false
	}

	return err.(awserr.Error).Code() == code
}

// ToMapTags converts a list of tags to a map
func ToMapTags(tags map[string]*string) []map[string]interface{} {
	var list []map[string]interface{}

	for key, value := range tags {
		list = append(list, map[string]interface{}{
			"key":    key,
			"values": []string{aws.StringValue(value)},
		})
	}

	return list
}

// IsResourceNotFoundException returns true if the given error is a resource not found error
func IsResourceNotFoundException(err error) bool {
	return IsAWSError(err, eks.ErrCodeResourceNotFoundException)
}

// HasTag returns true if the given tag is present in the given list of tags
func HasTag(tags []*ec2.Tag, key, value string) bool {
	vm := regexp.MustCompile(fmt.Sprintf("^%s$", value))

	tag, found := GetTag(tags, key)
	if !found {
		return false
	}

	return vm.MatchString(aws.StringValue(tag.Value))
}

// GetTagValue returns the value of the given tag key if it exist
func GetTagValue(tags []*ec2.Tag, key string) (string, bool) {
	tag, found := GetTag(tags, key)
	if !found {
		return "", false
	}

	return aws.StringValue(tag.Value), true
}

// GetTag returns the value of the given tag key if it exist
func GetTag(tags []*ec2.Tag, key string) (*ec2.Tag, bool) {
	km := regexp.MustCompile(fmt.Sprintf("^%s$", key))

	for _, tag := range tags {
		if km.MatchString(aws.StringValue(tag.Key)) {
			return tag, true
		}
	}

	return nil, false
}
