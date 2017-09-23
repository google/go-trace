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
	"log"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Logger defines the output functionality needed by Tracer. Note that
// log.Logger satisfies this interface.
type Logger interface {
	Printf(format string, v ...interface{})
	Println(v ...interface{})
}

type FrameInfo struct {
	runtime.Frame

	// The time at which this stack frame was recorded. Note that
	// parent functions up the stack (until the previous call to
	// Trace()) will have the same TimeRecorded because they were
	// recorded at the same time, even though they were actually
	// entered at different times.
	TimeRecorded time.Time
}

// Copy returns a deep copy of `fr`.
func (fr *FrameInfo) Copy() *FrameInfo {
	if fr == nil {
		return nil
	}
	return &FrameInfo{Frame: fr.Frame, TimeRecorded: fr.TimeRecorded}
}

// Equal returns true if `fr` is identical to `other`.
func (fr *FrameInfo) Equal(other *FrameInfo) bool {
	return *fr == *other
}

// Same returns true if fr.Frame and other.Frame are equal.
func (fr *FrameInfo) Same(other *FrameInfo) bool {
	return fr.Frame == other.Frame
}

// from creates a *FrameInfo from the provided `frame` and `timeStamp`.
func from(frame runtime.Frame, timeStamp time.Time) *FrameInfo {
	return &FrameInfo{Frame: frame, TimeRecorded: timeStamp}
}

type GoroutineInfo struct {
	// ID holds the numerical ID of this subroutine.
	ID int

	// Frames is a list of the current stack frames, with the top
	// of the stack being the 0th element.
	Frames []*FrameInfo

	// TopMessage is the message, if any, logged on the last call
	// to Trace() on this goroutine.
	//
	// Note that only the top frame in the current stack at any
	// moment could possibly have a message. All the previous
	// stack frames are by necessity from a function call site,
	// not from a Trace() call site, so they cannot have any
	// messages in them. As a result, we store the topMessage once
	// per goroutine rather than having mostly empty fields in
	// frames
	TopMessage string

	// History holds all the logging entries ever written for this
	// goroutine.
	History []string
}

// Copy returns a deep copy of `gi`.
func (gi *GoroutineInfo) Copy() *GoroutineInfo {
	if gi == nil {
		return nil
	}
	newGi := &GoroutineInfo{
		ID:         gi.ID,
		Frames:     make([]*FrameInfo, len(gi.Frames)),
		TopMessage: gi.TopMessage,
		History:    make([]string, len(gi.History)),
	}
	for idx, frame := range gi.Frames {
		newGi.Frames[idx] = frame.Copy()
	}
	for idx, entry := range gi.History {
		newGi.History[idx] = entry
	}
	return newGi
}

// Tracer records and echoes the call stack when Trace() is
// invoked. The public parameters configure how Tracer operates, and
// may be changed at run time, in which case they take effect on the
// next call to Trace().
type Tracer struct {
	// On determines whether the Tracer is active or not.
	On bool

	// Out receives the output of the Trace() calls.
	Out Logger

	// Capacity holds the maximum stack size we can accomodate.
	Capacity int

	// SourceLength holds the maxium displayed length,
	// right-justified, of the string specifying the source code
	// file name and line number. The
	SourceLength int

	// LockGoroutine causes Trace() to only record and emit output
	// for the current (ie last invoking) goroutine.
	LockGoroutine bool

	// OmitTime causes Tracer to not output date/time info. This
	// might be useful to shorten output lines if the Logger
	// specified in Out already displays time info. However, it is
	// recommended to leave this set to false so that accurate
	// time stamps are displayed when printing previously recorded
	// frames (if enabled via the OnGoroutinePrint* options).
	OmitTime bool

	// ClockFn is the function that will return the time used to
	// record when Trace() calls were invoked. If not specified,
	// time.Now will be used.
	ClockFn func() time.Time

	// OnGoroutineSwitchPrintCurrentStack prints out the current
	// state of the call stack when switching to a different
	// goroutine. Note that this does NOT print the whole History
	// of the stack for this goroutine.
	OnGoroutineSwitchPrintCurrentStack bool

	// OnGoroutineSwitchPrintStackHistory prints out the entire
	// history of the current call stack when switching to a
	// different goroutine.
	OnGoroutineSwitchPrintStackHistory bool

	goroutines                  map[int]*GoroutineInfo
	mutex                       sync.Mutex
	goroutineID                 int
	indents                     []string
	marker                      string
	calloutPrevious, calloutNew rune
}

// Goroutines returns a map of goroutine IDs to GoroutineInfo objects
// reflecting the current state of `tr`. The returned map is a deep
// copy of the internal state of `tr`.
func (tr *Tracer) Goroutines() map[int]*GoroutineInfo {
	if tr == nil {
		return nil
	}
	res := make(map[int]*GoroutineInfo, len(tr.goroutines))
	for key, val := range tr.goroutines {
		res[key] = val.Copy()
	}
	return res
}

func (tr *Tracer) proceed() bool {
	if tr == nil || !tr.On || tr.Out == nil || tr.Capacity <= 0 {
		return false
	}
	if tr.goroutines == nil {
		tr.goroutines = make(map[int]*GoroutineInfo)
		tr.calloutPrevious = ' '
		tr.calloutNew = '+'
	}
	if tr.ClockFn == nil {
		tr.ClockFn = time.Now
	}
	return true
}

// Trace records and echoes the state of the current goroutine's stack
// since the last time it was recorded. Those "new" entries are
// printed with a "+" marker. Previous entries on the call stack may
// be printed depending on the values of
// OnGoroutineSwitchPrintCurrentStack and
// OnGoroutineSwitchPrintStackHistory; those previous entries will NOT
// have a "+" marker.
//
// Note that the output cannot distinguish between to consecutive
// calls to Trace from the same stack frame, vs two consecutive calls
// to Trace from sibling stack frames.
//
// The parameter `skip` denotes the number of
// stack frames to skip in processing; a value of 0 denotes to start
// processing with the caller of this function as the top of the stack.
func (tr *Tracer) Trace(skip int, args ...interface{}) {
	if !tr.proceed() {
		return
	}

	tr.mutex.Lock()
	defer tr.mutex.Unlock()

	proceed, changedGoroutine, goroutine := tr.setGoroutine()
	if !proceed {
		return
	}

	now := tr.ClockFn()

	allFrameInfos := getFrameInfos(skip+1, tr.Capacity, now)
	goroutine.TopMessage = messageFrom(args...)

	lastCommonFrameStoredIdx, lastCommonFrameNewIdx := findLastCommonFrameIndex(goroutine.Frames, allFrameInfos)

	// Copying this way preserves the metadata in the common trace.Frames
	goroutine.Frames = append(allFrameInfos[:lastCommonFrameNewIdx], goroutine.Frames[lastCommonFrameStoredIdx:]...)

	printFrom := lastCommonFrameNewIdx
	if changedGoroutine {
		if tr.OnGoroutineSwitchPrintStackHistory {
			tr.printHistory(goroutine)
		} else if tr.OnGoroutineSwitchPrintCurrentStack {
			printFrom = -1
		}
	}
	tr.printFrameIndicesLowerThan(goroutine, printFrom, lastCommonFrameNewIdx)
}

func (tr *Tracer) printHistory(goroutine *GoroutineInfo) {
	for _, line := range goroutine.History {
		tr.Out.Println(line)
	}
}

// prints all the frames in the goroutine with indices strictly lower
// (ie frames higher on the stack) than idx, marking as new the ones
// with indices strictly lower (ie frames higher on the stack) than
// markFrom.
func (tr *Tracer) printFrameIndicesLowerThan(goroutine *GoroutineInfo, idx, markFrom int) {
	numFrames := len(goroutine.Frames)
	if idx < 0 {
		idx = numFrames
	}
	if markFrom < 0 {
		markFrom = numFrames
	}
	idx--
	if idx >= numFrames {
		fmt.Printf("error: idx == %d, len(goroutine.Frames) == %d\n", idx, len(goroutine.Frames))
	}
	for ; idx >= 0; idx-- {
		var location string
		frame := goroutine.Frames[idx]
		if tr.SourceLength > 0 {
			location = fmt.Sprintf("%200s:%-4d  p%d g%-3d%%c", frame.File, frame.Line, frame.PC, goroutine.ID)
			if len(location) > tr.SourceLength {
				location = location[len(location)-tr.SourceLength:]
			}
		}

		var timestamp string
		if !tr.OmitTime {
			timestamp = frame.TimeRecorded.Format("2006-01-02 15:04:05.00000000 ")
		}

		var message string
		if idx == 0 {
			message = goroutine.TopMessage
		}
		level := len(goroutine.Frames) - idx - 1
		line := strings.TrimSpace(fmt.Sprintf("%s%s%s %s() %s", timestamp, location, tr.indentation(level), frame.Function, message))
		goroutine.History = append(goroutine.History, fmt.Sprintf(line, tr.calloutPrevious))
		callout := tr.calloutPrevious
		if idx < markFrom {
			callout = tr.calloutNew
		}
		tr.Out.Printf(line, callout)
	}
}

func (tr *Tracer) setGoroutine() (proceed, changed bool, goroutine *GoroutineInfo) {
	goroutineID := GoroutineID()
	if goroutineID != tr.goroutineID {
		if tr.LockGoroutine {
			return false, true, nil
		}
		if len(tr.marker) != tr.SourceLength {
			tr.marker = strings.Repeat("-", tr.SourceLength)
		}
		tr.Out.Printf("%s goroutine switched: %3d -> %-3d %s", tr.marker, tr.goroutineID, goroutineID, tr.marker)
		changed = true
	}
	tr.goroutineID = goroutineID
	goroutine = tr.goroutines[tr.goroutineID]
	if goroutine == nil {
		goroutine = &GoroutineInfo{ID: goroutineID}
		tr.goroutines[goroutineID] = goroutine
	}
	return true, changed, goroutine
}

func (tr *Tracer) indentation(level int) string {
	for level >= len(tr.indents) {
		tr.indents = append(tr.indents, strings.Repeat("  ", len(tr.indents)))
	}
	return tr.indents[level]
}

// skip==0 is the caller of this function
func getFrameInfos(skip, capacity int, now time.Time) []*FrameInfo {
	frames := runtimeFrames(skip+1, capacity)
	allFrameInfos := make([]*FrameInfo, 0, capacity)
	for {
		newFrameInfo, more := frames.Next()
		allFrameInfos = append(allFrameInfos, from(newFrameInfo, now))
		if !more {
			break
		}
	}
	return allFrameInfos
}

// skip==0 is the caller of this function
func runtimeFrames(skip, capacity int) *runtime.Frames {
	pc := make([]uintptr, capacity)
	num := runtime.Callers(2+skip, pc)
	pc = pc[:num]
	return runtime.CallersFrames(pc)
}

// findLastCommonFrameIndex returns the indices in `first` and
// `second` that correspond to the latest (lowest index) common stack
// frame in both slices, except that they will not both return 0.
//
// Post-conditions:
//   len(first[firstIdx:]) == len(second[secondIdx:]) == len
//   first[firstIdx+i].Same(second[secondIdx+i])) == true for all 0 <= i < len
//   (firstIdx==secondIdx==0) == false
func findLastCommonFrameIndex(first, second []*FrameInfo) (firstIdx, secondIdx int) {
	firstLen, secondLen := len(first), len(second)

	// find firstIdx, secondIdx such that
	// len(first[firstIdx:]) == len(second[secondIdx:]),
	// since the latest common frame will be in those parts
	// of the array

	if firstLen > secondLen {
		firstIdx = firstLen - secondLen
	} else {
		secondIdx = secondLen - firstLen
	}

	// avoid omitting consecutive calls from a loop
	if firstIdx == 0 && secondIdx == 0 {
		firstIdx++
		secondIdx++
	}

	var increment int
	for firstIdx < firstLen {
		// If firstIdx and secondIdx denote the latest
		// matching frames, then all of first[firstIdx:]
		// and second[secondIdx:] must match
		matching := true
		for increment = 0; firstIdx+increment < firstLen; increment++ {
			if !first[firstIdx+increment].Same(second[secondIdx+increment]) {
				matching = false
				break
			}
		}
		if matching {
			break
		}

		// Increment the parallel indices to the next location after the mismatch
		firstIdx += increment + 1
		secondIdx += increment + 1
	}
	return firstIdx, secondIdx
}

// messageFrom constructs a string from `args`. Normally, `args[0]` is
// a format string and `args[1:]` are its arguments
func messageFrom(args ...interface{}) string {
	var message string
	num := len(args)
	if num > 0 {
		format, ok := args[0].(string)
		var values []interface{}
		if !ok {
			format = strings.Repeat("%v ", num)
			values = args
		} else if num > 1 {
			values = args[1:]
		}

		message = fmt.Sprintf(format, values...)
	}
	return message

}

// TruncateError returns the string representation of `err` truncated
// to `max` characters.
func TruncateError(err error, max int) string {
	msg := err.Error()
	if len(msg) > max {
		msg = msg[:max]
	}
	return msg
}

// GoroutineID returns the numerical ID of the currently running goroutine.
func GoroutineID() int {
	// Implementation taken from
	// https://groups.google.com/forum/#!topic/golang-nuts/Nt0hVV_nqHE
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	idField := strings.Fields(strings.TrimPrefix(string(buf[:n]), "goroutine "))[0]
	id, err := strconv.Atoi(idField)
	if err != nil {
		panic(fmt.Sprintf("cannot get goroutine id: %v", err))
	}
	return id
}

// Global is the global instance of Tracer, which can be easily
// accessed by the trace.Trace() function. The public parameters of
// Global may be changed dynamically and affect subsequent calls to
// Trace(). Refer to this module's init() function for more
// details.
var Global *Tracer

// Trace prints out the call stack of the current goroutine. The top
// stack frame is annotated with `args`, which are interpreted as
// parameters to fmt.Printf().
func Trace(args ...interface{}) {
	Global.Trace(1, args...)
}

// On turns tracing with the global debugger on or off. It's nothing
// more than a shorthand for setting Global.On manually.
func On(on bool) {
	Global.On = on
}
func init() {
	Global = &Tracer{
		// Any of these settings may be changed dynamically as
		// the tracer is running, and will affect subsequent
		// calls.

		// By default, tracing is off so any calls to Trace()
		// will return quickly. Make sure to turn it from the
		// point in your application where you're interested
		// in tracing. Depending on your needs, this could be
		// in one of your functions, in your main() , or in
		// one of your module init() functions.
		On: false,

		// These default settings seem like the most useful for interactive debugging.
		OnGoroutineSwitchPrintCurrentStack: false,
		OnGoroutineSwitchPrintStackHistory: true,

		Capacity:     100,
		Out:          log.New(os.Stdout, "trace> ", 0),
		SourceLength: 40,
	}
}
