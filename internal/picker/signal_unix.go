//go:build unix

package picker

import (
	"os"
	"os/signal"
	"syscall"
)

func signalNotify(ch chan<- os.Signal) {
	signal.Notify(ch, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT)
}

func signalStop(ch chan<- os.Signal) {
	signal.Stop(ch)
}
