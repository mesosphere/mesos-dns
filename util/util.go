/*
Copyright 2014 Google Inc. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package util

import (
	"fmt"
	"log"
	"runtime"
	"time"
)

// For testing, bypass HandleCrash.
var ReallyCrash bool

// PanicHandlers is a list of functions which will be invoked when a panic happens.
var PanicHandlers = []func(interface{}){logPanic}

// HandleCrash simply catches a crash and logs an error. Meant to be called via defer.
func HandleCrash() {
	if ReallyCrash {
		return
	}
	if r := recover(); r != nil {
		for _, fn := range PanicHandlers {
			fn(r)
		}
	}
}

// logPanic logs the caller tree when a panic occurs.
func logPanic(r interface{}) {
	callers := ""
	for i := 0; true; i++ {
		_, file, line, ok := runtime.Caller(i)
		if !ok {
			break
		}
		callers = callers + fmt.Sprintf("%v:%v\n", file, line)
	}
	log.Printf("Recovered from panic: %#v (%v)\n%v", r, r, callers)
}

func Forever(f func(), period time.Duration) {
	Until(f, period, nil)
}

// periodically execute the given function, stopping once stopCh is closed.
// this func blocks until stopCh is closed, it's intended to be run as a goroutine.
func Until(f func(), period time.Duration, stopCh <-chan struct{}) {
	if f == nil {
		return
	}
	for {
		select {
		case <-stopCh:
			return
		default:
		}
		func() {
			defer HandleCrash()
			f()
		}()
		select {
		case <-stopCh:
		case <-time.After(period):
		}
	}
}

func OnError(abort <-chan struct{}, errCh <-chan error, f func(error)) <-chan struct{} {
	ch := make(chan struct{})
	go func() {
		select {
		case <-abort:
		case e := <-errCh:
			if e != nil {
				defer close(ch)
				f(e)
			}
		}
	}()
	return ch
}
