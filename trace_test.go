/*
Copyright 2018 Google LLC.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package trace

import (
	"fmt"
	"runtime"
	"testing"
)

func TestFindLastCommonFrame(t *testing.T) {
	for idx, tc := range []struct {
		label                           string
		first, second                   []*FrameInfo
		wantFirstIndex, wantSecondIndex int
	}{
		{
			label:           "equal stacks (special case for top element)",
			first:           fromStrings("Carol", "Bob", "Alice"),
			second:          fromStrings("Carol", "Bob", "Alice"),
			wantFirstIndex:  1,
			wantSecondIndex: 1,
		},
		{
			label:           "equal length stacks, equal after top 2",
			first:           fromStrings("Eric", "Carol", "Bob", "Alice"),
			second:          fromStrings("Felicia", "Dan", "Bob", "Alice"),
			wantFirstIndex:  2,
			wantSecondIndex: 2,
		},
		{
			label:           "first subset of second",
			first:           fromStrings("Bob", "Alice"),
			second:          fromStrings("Felicia", "Dan", "Bob", "Alice"),
			wantFirstIndex:  0,
			wantSecondIndex: 2,
		},
		{
			label:           "second subset of first",
			first:           fromStrings("Eric", "Carol", "Bob", "Alice"),
			second:          fromStrings("Bob", "Alice"),
			wantFirstIndex:  2,
			wantSecondIndex: 0,
		},
		{
			label:           "second subset of first, first repeats",
			first:           fromStrings("Bob", "Alice", "Eric", "Carol", "Bob", "Alice"),
			second:          fromStrings("Bob", "Alice"),
			wantFirstIndex:  4,
			wantSecondIndex: 0,
		},
		{
			label:           "second empty",
			first:           fromStrings("Eric", "Carol", "Bob", "Alice"),
			wantFirstIndex:  4,
			wantSecondIndex: 0,
		},
		{
			label:           "first empty",
			second:          fromStrings("Eric", "Carol", "Bob", "Alice"),
			wantFirstIndex:  0,
			wantSecondIndex: 4,
		},
	} {
		label := fmt.Sprintf("[case %d: %q]", idx, tc.label)
		first, second := findLastCommonFrameIndex(tc.first, tc.second)

		// Test the explicit post-conditions listed in the function description
		firstCommon := tc.first[first:]
		secondCommon := tc.second[second:]
		if fl, sl := len(firstCommon), len(secondCommon); fl != sl {
			t.Errorf("%s subslice lengths unequal: first: %d, seond: %d", label, fl, sl)
		}
		for idx, el := range firstCommon {
			if !el.Same(secondCommon[idx]) {
				t.Errorf("%s element %d in the subslice fails Same()", label, idx)
			}
		}
		if first == 0 && second == 0 {
			t.Errorf("%s returned 0 for both indices", label)
		}

		if got, want := first, tc.wantFirstIndex; got != want {
			t.Errorf("%s first index: got %d, want %d", label, got, want)
		}
		if got, want := second, tc.wantSecondIndex; got != want {
			t.Errorf("%s second index: got %d, want %d", label, got, want)
		}
	}
}

func fromStrings(labels ...string) []*FrameInfo {
	frames := make([]*FrameInfo, len(labels))
	for idx, name := range labels {
		frames[idx] = &FrameInfo{
			Frame: runtime.Frame{File: name},
		}
	}
	return frames
}
