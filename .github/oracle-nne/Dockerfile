# Copyright 2023 The Cockroach Authors
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
# SPDX-License-Identifier: Apache-2.0

# Use a private, local copy of the XE image. Original source is
# container-registry.oracle.com/database/express:21.3.0-xe
FROM ghcr.io/cockroachdb/cdc-sink/oracle-express:21.3.0-xe
# Configure with NNE turned on.
COPY sqlnet.ora /opt/oracle/oradata/dbconfig/XE
