package warp

import (
	"fmt"
	"runtime"
)

type StackError struct {
	err   error
	stack []uintptr
}

func NewStackError(e interface{}, skip int) *StackError {
	if e == nil {
		return nil
	}

	var err error
	switch x := e.(type) {
	case error:
		err = x
	default:
		err = fmt.Errorf("%v", x)
	}
	stack := make([]uintptr, 50)
	length := runtime.Callers(2+skip, stack)
	return &StackError{
		err:   err,
		stack: stack[:length],
	}
}

func (se *StackError) Error() string {
	return fmt.Sprintf("%s\nStack Trace:\n%s", se.err.Error(), se.stackTrace())
}

func (se *StackError) stackTrace() string {
	var trace string
	frames := runtime.CallersFrames(se.stack)
	for {
		frame, more := frames.Next()
		trace += fmt.Sprintf("%s:%d %s\n", frame.File, frame.Line, frame.Function)
		if !more {
			break
		}
	}
	return trace
}
