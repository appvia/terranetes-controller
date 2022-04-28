/*
 * Copyright (C) 2022  Rohith Jayawardene <gambol99@gmail.com>
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

package main

import (
	"github.com/hashicorp/go-cleanhttp"
	"github.com/hashicorp/go-getter"
)

var goGetterDetectors = []getter.Detector{
	new(getter.GitHubDetector),
	new(getter.GitLabDetector),
	new(getter.GitDetector),
	new(getter.BitBucketDetector),
	new(getter.GCSDetector),
	new(getter.S3Detector),
}

var goGetterNoDetectors = []getter.Detector{}

var goGetterDecompressors = map[string]getter.Decompressor{
	"bz2":      new(getter.Bzip2Decompressor),
	"gz":       new(getter.GzipDecompressor),
	"tar.bz2":  new(getter.TarBzip2Decompressor),
	"tar.gz":   new(getter.TarGzipDecompressor),
	"tar.tbz2": new(getter.TarBzip2Decompressor),
	"tar.xz":   new(getter.TarXzDecompressor),
	"tgz":      new(getter.TarGzipDecompressor),
	"txz":      new(getter.TarXzDecompressor),
	"xz":       new(getter.XzDecompressor),
	"zip":      new(getter.ZipDecompressor),
}

var goGetterGetters = map[string]getter.Getter{
	"file":  new(getter.FileGetter),
	"gcs":   new(getter.GCSGetter),
	"git":   new(getter.GitGetter),
	"hg":    new(getter.HgGetter),
	"s3":    new(getter.S3Getter),
	"http":  getterHTTPGetter,
	"https": getterHTTPGetter,
}

var getterHTTPClient = cleanhttp.DefaultClient()

var getterHTTPGetter = &getter.HttpGetter{
	Client: getterHTTPClient,
	Netrc:  true,
}
