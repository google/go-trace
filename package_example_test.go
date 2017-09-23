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

package trace_test

import (
	"time"
	"trace"
)

func a(n string) {
	trace.Trace("Alice: [%s]", n)
}

func b(n string) {
	trace.Trace("Barb! [%s]", n)
	for i := 1; i < 5; i++ {
		time.Sleep(10 * time.Millisecond)
		a("barb " + n)
	}
	trace.Trace("Bob! [%s]", n)
	for i := 1; i < 5; i++ {
		time.Sleep(10 * time.Millisecond)
		a("bob " + n)
	}
	trace.Trace("Blaise! [%s]", n)
}
func d(n string) {
	trace.Trace("Dely! [%s]", n)
	for i := 1; i < 5; i++ {
		trace.Trace()
	}
	trace.Trace("Dan! [%s]", n)
}

func c(n string) {
	trace.Trace("Carol before [%s]", n)
	time.Sleep(50 * time.Millisecond)
	a("carol " + n)
	trace.Trace("Carol after [%s]", n)

}

func Example() {
	trace.Global.On = true
	trace.Trace("Start")
	c("one")

	if true {
		go a("two")
		go c("three")
		go b("four")
		go b("five")
	} else {
		a("six")
		c("seven")
		b("eight")
		b("nine")
	}
	a("ten")
	d("eleven")

	trace.Trace("The end!")
	time.Sleep(5 * time.Second)
}
