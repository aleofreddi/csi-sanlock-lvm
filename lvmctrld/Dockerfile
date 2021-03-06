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

FROM golang:1.13 AS builder

RUN apt-get update \
    && apt-get install -y protobuf-compiler golang-goprotobuf-dev gettext-base

ARG VERSION
ADD . /src
RUN make -C /src clean build.lvmctrld

FROM ubuntu:18.04

RUN apt-get update \
	&& apt-get install -y file util-linux e2fsprogs lvm2 sanlock lvm2-lockd \
	&& apt-get clean

COPY ./lvmctrld/conf/lvm.conf etc/lvm/lvm.conf
COPY --from=builder /src/lvmctrld/bin/lvmctrld /lvmctrld

ENTRYPOINT ["/lvmctrld"]
