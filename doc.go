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

/*
Package trace allows debugging programs via trace statements inserted
in code. Sometimes you need to see the sequence in which functions are
called, and stepping through each manually with a debugger is not the
appropriate answer. If you've ever sprinkled "Printf" statements
throughout your code to get insight into what's happenning, this
package is strictly better: in addition to printing the messages you
specify, it denotes the structure of the stack by appropriately
indenting them (and indenting the names of the callers), and printing
time stamps of when the trace calls occurred.

Tracing is as simple as inserting a Trace statements where you want to
do the tracing. These statements will print all the currrent stack
frames to the most recent previously traced stack frame that is on the
current stack.

  trace.Trace()

You may also pass Printf-style arguments to Trace to display
interesting state of your program:

  trace("Label: my var=%v", myvar)

You should turn on tracing before the point in your program where you
want to trace. If you want to trace a package init() function, turn it
on there. This function call is idempotent. It is merely a shorthand
for setting Global.On:

  trace.On(true)

You may also turn off tracing at any point you wish:

  trace.On(false)

In fact, you may change many of the seetings of an active Tracer
object by modifying them directly. For example, to make global tracer
only trace the last reported goroutine, use

  trace.Global.LockGoRoutine = true

While in most cases you'll want to use the trace.Global logger
(accessible directly and through the trace.Trace function), you can
also create custom Tracer objects and use them via Tracer.Trace().

Tracer is concurrency-safe. When Trace() is called from a different
goroutine than its previous call, it prints a warning about a
goroutine switch. Optionally, it can print just the current goroutine
stack frame including frames recorded earlier (if
OnGoroutineSwitchPrintCurrentStack is set; default is false in
Global), or print the entire history of the stack for this goroutine
(if OnGoroutineSwitchPrintStackHistory is set; default is true in
Global).





*/
package trace
