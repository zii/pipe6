// handshake protocal for pipe6
package mux

import (
	"fmt"
	"log"
	"os"
)

var std = log.New(os.Stderr, "", log.Ldate|log.Ltime|log.Lshortfile)

func debug(a ...interface{}) {
	std.Output(2, fmt.Sprintln(a...))
}
