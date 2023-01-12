// Copyright 2020 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package driverd_test

import (
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/google/go-cmp/cmp"
)

type cmpMatcher struct {
	t    *testing.T
	want interface{}
	opts []cmp.Option
}

func CmpMatcher(t *testing.T, want interface{}, opts ...cmp.Option) gomock.Matcher {
	return &cmpMatcher{t, want, opts}
}

func (m *cmpMatcher) Matches(actual interface{}) bool {
	return cmp.Equal(m.want, actual, m.opts...)
}

func (m *cmpMatcher) String() string {
	return fmt.Sprintf("is equal to %v using %s", m.want, cmp.Options(m.opts).String())
}
