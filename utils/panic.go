package utils

import (
	"fmt"
	"github.com/vuuvv/vpacket/log"
)

func PanicIf(err error) {
	if err != nil {
		panic(err)
	}
}

func Panicf(format string, a ...any) {
	panic(fmt.Sprintf(format, a...))
}

func NormalRecover() {
	if r := recover(); r != nil {
		log.Error(r)
	}
}

func Catch(handler func(reason any)) {
	if r := recover(); r != nil {
		log.Error(r)
		handler(r)
	}
}

func RecoverableFunction(fn func()) func() {
	return func() {
		defer NormalRecover()
		fn()
	}
}

func RecoverableFunctionSimple(fn func()) func() {
	return func() {
		defer NormalRecover()
		fn()
	}
}

func SafeCall(fn func()) {
	defer NormalRecover()
	fn()
}
