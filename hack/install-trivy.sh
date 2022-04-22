#!/bin/bash
#
# Copyright (C) 2021  Rohith Jayawardene <gambol99@gmail.com>
#
# This program is free software; you can redistribute it and/or
# modify it under the terms of the GNU General Public License
# as published by the Free Software Foundation; either version 2
# of the License, or (at your option) any later version.
#
# This program is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
# GNU General Public License for more details.
#
# You should have received a copy of the GNU General Public License
# along with this program.  If not, see <http://www.gnu.org/licenses/>.
#

${TRACE:+set -x}

VERSION="0.16.0"
TRIVY=$(which trivy 2>/dev/null)

[[ -n ${TRIVY} ]] && exit

ARCH=$(uname -s)

case "${ARCH}" in
  Linux*) FILE="trivy_${VERSION}_Linux-64bit.tar.gz" ;;
  Darwin*) FILE="trivy_${VERSION}_macOS-64bit.tar.gz" ;;
  *)
    echo "[error] unsupported architecture"
    exit 1
    ;;
esac

URL="https://github.com/aquasecurity/trivy/releases/download/v${VERSION}/${FILE}"

if ! curl -sL ${URL} -o /tmp/trivy.tar.gz; then
  echo "[error] failed to download trivy release: ${VERSION}"
  exit 1
fi

if ! tar zxf /tmp/trivy.tar.gz -C bin/ trivy; then
  echo "[error] failed to untar the release"
  exit 1
fi
