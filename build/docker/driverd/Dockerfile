# Copyright 2020 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

FROM golang:1.17 AS builder

RUN apt-get update \
    && apt-get install -y protobuf-compiler gettext-base

ADD . /src
WORKDIR /src
RUN go get github.com/golang/protobuf/protoc-gen-go google.golang.org/grpc/cmd/protoc-gen-go-grpc github.com/golang/mock/mockgen \
    && go install github.com/golang/protobuf/protoc-gen-go google.golang.org/grpc/cmd/protoc-gen-go-grpc github.com/golang/mock/mockgen
ARG VERSION COMMIT
RUN make VERSION=$VERSION COMMIT=$COMMIT cmd/driverd/driverd

FROM ubuntu:18.04

RUN apt-get update \
	&& apt-get install -y file util-linux e2fsprogs lvm2 \
	&& apt-get clean

COPY --from=builder /src/cmd/driverd/driverd /driverd

ENTRYPOINT ["/driverd"]
