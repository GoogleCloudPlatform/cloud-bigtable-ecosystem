#!/bin/bash

# Copyright 2025 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#    https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# This script builds the cassandra-to-bigtable-proxy binaries for the target
# platforms/architectures below. This script should be executed via Maven, rather
# than invoked directly.
#
# Usage: ./build.sh <platform> <architecture> <version-number> <absolute-project-base-directory>
#
# Target platform/architectures
#  # Linux Builds
#    linux/amd64
#    linux/arm64
#  # Mac Builds
#    darwin/amd64
#    darwin/arm64

set -eo pipefail

export CGO_ENABLED=0 # https://github.com/golang/go/issues/22510
export RELATIVE_SOURCE_DIR=../../cassandra-bigtable-proxy

function build() {
  local env="${1}"
  local arch="${2}"
  local version="${3}"
  local base_dir="${4}"
  local source_dir="${base_dir}/${RELATIVE_SOURCE_DIR}"
  local target_subdir="${version}/${env}/${arch}"
  local target_dir="${base_dir}/target/${target_subdir}"
  local binary_name="cassandra-to-bigtable-proxy"

  mkdir -p ${target_dir} || return 1
  cd $source_dir
  mkdir -p ${target_subdir} || return 1
  # Build binary
  CGO_ENABLED=${CGO_ENABLED} GOOS=${env} GOARCH=${arch} go build -o ${target_subdir}/${binary_name} . || return 1
  mv ${target_subdir}/${binary_name} ${target_dir}/ || return 1
  # Copy NOTICE and licenses
  cp LICENSE ${target_dir}/LICENSE || return 1
  cp NOTICE.md ${target_dir}/NOTICE.md || return 1
  mkdir -p ${target_dir}/third_party/licenses || return 1
  cp -r third_party/licenses ${target_dir}/third_party || return 1
  cd $base_dir
}

usage() {
  echo "Usage: $0 <platform> <architecture> <version-number> <absolute-project-base-directory>"
  exit 1
}

function main() {
  # Print the usage instructions if the --help flag is used
  if [[ "$1" == "--help" ]]; then
    usage
  fi

  # Expect exactly 4 arguments, otherwise print the usage instructions
  if [ $# -ne 4 ]; then
    usage
  fi

  local env=${1}
  local arch=${2}
  local version="${3}"
  local base_dir=${4}

  # Check that the 4 arguments are non-empty strings
  if [[ -z "$env" ]]; then
    echo "Platform must be specified"
    usage
  fi
  if [[ -z "arch" ]]; then
    echo "Architecture must be specified"
    usage
  fi
  if [[ -z "version" ]]; then
    echo "Version number must be specified"
    usage
  fi
  if [[ -z "base_dir" ]]; then
    echo "Absolute project base directory must be specified"
    usage
  fi

  echo "Building cassandra-to-bigtable-proxy binary"
  echo "Building for target: ${env} ${arch}"
  echo "Building version: ${version}"
  echo "Using source code: ${base_dir}/${RELATIVE_SOURCE_DIR}"

  mkdir -p ${base_dir}/target
  builds=()
  build "${env}" "${arch}" ${version} ${base_dir} || return 1
  echo "Done"
}

main "$@"
