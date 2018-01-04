package timers

import (
	"time"

	"github.com/robertkrimen/otto"

	"github.com/status-im/status-go/geth/jail/internal/loop"
	"github.com/status-im/status-go/geth/jail/internal/vm"
)

var minDelay = map[bool]int64{
	true:  10,
	false: 4,
}

type intervalFunc func(bool, *loop.Loop) func(call otto.FunctionCall) otto.Value

// Define jail timers
func Define(vm *vm.VM, l *loop.Loop) (err error) {
	if v, err := vm.Get("setTimeout"); err != nil {
		return err
	} else if !v.IsUndefined() {
		return nil
	}

	intervals := map[string]intervalFunc{
		"setInterval": newTimer,
	}
	for k, interval := range intervals {
		if err = vm.Set(k, interval(true, l)); err != nil {
			return
		}
	}

	timers := map[string]intervalFunc{
		"setTimeout":     newTimer,
		"setImmediate":   immediateTimer,
		"clearTimeout":   clearTimeout,
		"clearInterval":  clearTimeout,
		"clearImmediate": clearTimeout,
	}
	for k, timer := range timers {
		if err = vm.Set(k, timer(false, l)); err != nil {
			return
		}
	}

	return
}

func newTimer(interval bool, l *loop.Loop) func(call otto.FunctionCall) otto.Value {
	return func(call otto.FunctionCall) otto.Value {
		delay, _ := call.Argument(1).ToInteger()
		if delay < minDelay[interval] {
			delay = minDelay[interval]
		}

		t := &timerTask{
			duration: time.Duration(delay) * time.Millisecond,
			call:     call,
			interval: interval,
		}

		// If err is non-nil, then the loop is closed and should not
		// be used anymore.
		if err := l.Add(t); err != nil {
			return otto.UndefinedValue()
		}

		t.timer = time.AfterFunc(t.duration, func() {
			l.Ready(t) // nolint: errcheck
		})

		value, newTimerErr := call.Otto.ToValue(t)
		if newTimerErr != nil {
			panic(newTimerErr)
		}

		return value
	}
}

func immediateTimer(interval bool, l *loop.Loop) func(call otto.FunctionCall) otto.Value {
	return func(call otto.FunctionCall) otto.Value {
		t := &timerTask{
			duration: time.Millisecond,
			call:     call,
		}

		// If err is non-nil, then the loop is closed and should not
		// be used anymore.
		if err := l.Add(t); err != nil {
			return otto.UndefinedValue()
		}

		t.timer = time.AfterFunc(t.duration, func() {
			l.Ready(t) // nolint: errcheck
		})

		value, setImmediateErr := call.Otto.ToValue(t)
		if setImmediateErr != nil {
			panic(setImmediateErr)
		}

		return value
	}
}

func clearTimeout(interval bool, l *loop.Loop) func(call otto.FunctionCall) otto.Value {
	return func(call otto.FunctionCall) otto.Value {
		v, _ := call.Argument(0).Export()
		if t, ok := v.(*timerTask); ok {
			t.stopped = true
			t.timer.Stop()
			l.Remove(t)
		}

		return otto.UndefinedValue()
	}
}
