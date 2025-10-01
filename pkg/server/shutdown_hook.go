package server

import (
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func AddShutdownHook(closers ...io.Closer) {
	c := make(chan os.Signal, 1)
	signal.Notify(
		c, os.Interrupt, syscall.SIGHUP, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM,
	)

	<-c

	for _, closer := range closers {
		if err := closer.Close(); err != nil {
			log.Printf("failed to stop closer: %v\n", err)
		}
	}
}
