#!/bin/bash
#
#  Licensed to the Apache Software Foundation (ASF) under one or more
#  contributor license agreements.  See the NOTICE file distributed with
#  this work for additional information regarding copyright ownership.
#  The ASF licenses this file to You under the Apache License, Version 2.0
#  (the "License"); you may not use this file except in compliance with
#  the License.  You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
#  Unless required by applicable law or agreed to in writing, software
#  distributed under the License is distributed on an "AS IS" BASIS,
#  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#  See the License for the specific language governing permissions and
#  limitations under the License.

go build

# Save the root directory where the script is located
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

for dir in ./test/correctly/*/; do
    if [ ! -d "$dir" ]; then
        continue
    fi
    
    cd "$dir" || exit 1

    dir_name=$(basename "$dir")
    echo "Testing $dir_name..."

    if [ "$dir_name" = "import_nested" ]; then
        echo "Testing import functionality in $dir_name..."
        if [ -f "proto/greet/v1/greet.proto" ] && [ -f "proto/greet/v1/common/common.proto" ]; then
            protoc -I=proto --go_out=proto --go_opt=paths=source_relative --plugin=protoc-gen-go-triple=../../../protoc-gen-go-triple --go-triple_out=proto --go-triple_opt=paths=source_relative proto/greet/v1/greet.proto proto/greet/v1/common/common.proto
        else
            echo "Warning: Required proto files not found in $dir_name"
            cd - || exit 1
            continue
        fi
    else
        if [ -f "./proto/greet.proto" ]; then
            protoc --go_out=. --go_opt=paths=source_relative --plugin=protoc-gen-go-triple=../../../protoc-gen-go-triple --go-triple_out=. ./proto/greet.proto
        else
            echo "Warning: greet.proto not found in $dir_name"
            cd - || exit 1
            continue
        fi
    fi
    
    go mod tidy

    if [ "$dir_name" = "import_nested" ]; then
        if [ -d "proto" ]; then
            cd proto && go vet ./...
        else
            echo "Warning: proto directory not found in $dir_name"
            cd - || exit 1
            continue
        fi
    else
        if [ -d "./proto" ]; then
            go vet ./proto/*.go
        else
            echo "Warning: proto directory not found in $dir_name"
            cd - || exit 1
            continue
        fi
    fi
    result=$?

    if [ $result -ne 0 ]; then
        echo "go vet found issues in $dir_name."
        exit $result
    fi

    echo "No issues found in $dir_name."
    cd "$SCRIPT_DIR" || exit 1
done