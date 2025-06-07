#!/bin/bash
# Copyright 2025 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -eo pipefail

## Get the directory of the build script
scriptDir=$(realpath $(dirname "${BASH_SOURCE[0]}"))
## cd to the parent directory, i.e. the root of the git repo
cd ${scriptDir}/..

# include common functions
source ${scriptDir}/common.sh

# cd to the cassandra-bigtable-java-client directory
cd ${scriptDir}/../cassandra-bigtable-migration-tools/cassandra-bigtable-java-client

# Print out Maven & Java version
mvn -version
echo ${JOB_TYPE}

# Support profiles
maven_profiles=("linux-amd64", "linux-arm64", "darwin-amd64", "darwin-arm64")

for maven_profile in "${maven_profiles[@]}"; do

    echo "Building profile: $maven_profile"

    # attempt to install 3 times with exponential backoff (starting with 10 seconds)
    retry_with_backoff 3 10 \
    mvn install -B -V -ntp \
        -P$maven_profile \
        -DskipTests=true \
        -Dclirr.skip=true \
        -Denforcer.skip=true \
        -Dmaven.javadoc.skip=true \
        -Dgcloud.download.skip=true \
        -T 1C


    RETURN_CODE=0
    set +e

    case ${JOB_TYPE} in
    test)
        mvn test -P$maven_profile -B -ntp -Dfmt.skip=true -Dclirr.skip=true -Denforcer.skip=true
        RETURN_CODE=$?
        ;;
    javadoc)
        mvn -P$maven_profile javadoc:javadoc javadoc:test-javadoc -B -ntp -Dfmt.skip=true
        RETURN_CODE=$?
        ;;
    clirr)
        mvn -P$maven_profile -B -ntp -Dfmt.skip=true -Denforcer.skip=true clirr:check
        RETURN_CODE=$?
        ;;
    *)
        ;;
    esac

done

echo "exiting with ${RETURN_CODE}"
exit ${RETURN_CODE}
