// handshake protocal for pipe6
package mux

import (
	"fmt"
	"log"
	"os"
)

var debugOn bool = true

func CloseDebugLog() {
	debugOn = false
}

var std = log.New(os.Stderr, "", log.Ldate|log.Ltime|log.Lshortfile)

func debug(a ...interface{}) {
	if debugOn {
		std.Output(2, fmt.Sprintln(a...))
	}
}
