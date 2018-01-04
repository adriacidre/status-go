package timers

import (
	"time"

	"github.com/robertkrimen/otto"
	"github.com/status-im/status-go/geth/jail/internal/loop"
	"github.com/status-im/status-go/geth/jail/internal/vm"
)

type timerTask struct {
	id       int64
	timer    *time.Timer
	duration time.Duration
	interval bool
	call     otto.FunctionCall
	stopped  bool
}

func (t *timerTask) SetID(id int64) { t.id = id }
func (t *timerTask) GetID() int64   { return t.id }

func (t *timerTask) Execute(vm *vm.VM, l *loop.Loop) error {
	var arguments []interface{}

	if len(t.call.ArgumentList) > 2 {
		tmp := t.call.ArgumentList[2:]
		arguments = make([]interface{}, 2+len(tmp))

		for i, value := range tmp {
			arguments[i+2] = value
		}
	} else {
		arguments = make([]interface{}, 1)
	}

	arguments[0] = t.call.ArgumentList[0]

	if _, err := vm.Call(`Function.call.call`, nil, arguments...); err != nil {
		return err
	}

	if t.interval && !t.stopped {
		t.timer.Reset(t.duration)
		if err := l.Add(t); err != nil {
			return err
		}
	}

	return nil
}

func (t *timerTask) Cancel() {
	t.timer.Stop()
}
