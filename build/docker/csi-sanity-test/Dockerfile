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

FROM golang:1.24 AS builder

RUN apt-get update \
    && apt-get install -y protobuf-compiler gettext-base

RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest \
    && go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest \
    && go install github.com/golang/mock/mockgen@latest \
    && go install github.com/kubernetes-csi/csi-test/v5/cmd/csi-sanity@latest

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY pkg/ pkg/
COPY cmd/ cmd/
COPY Makefile ./
COPY configs/ configs/
COPY csi-sanity ./
ARG VERSION=dev
ARG COMMIT=none
RUN make VERSION=$VERSION COMMIT=$COMMIT proto mock build

FROM ubuntu:24.04

RUN apt-get update \
	&& apt-get install -y file util-linux e2fsprogs lvm2 sanlock lvm2-lockd kmod \
	&& apt-get clean

COPY ./configs/lvmctrld/lvm.conf /etc/lvm/lvm.conf
RUN sed -i 's/use_lvmlockd = 1/use_lvmlockd = 0/' /etc/lvm/lvm.conf
RUN ln -s /proc/mounts /etc/mtab
COPY --from=builder /src/cmd/lvmctrld/lvmctrld /usr/local/bin/lvmctrld
COPY --from=builder /src/cmd/driverd/driverd /usr/local/bin/driverd
COPY --from=builder /go/bin/csi-sanity /usr/local/bin/csi-sanity
COPY csi-sanity /usr/local/bin/csi-sanity-test.sh

ENTRYPOINT ["/usr/local/bin/csi-sanity-test.sh"]
