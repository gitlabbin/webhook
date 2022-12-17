//go:build !windows
// +build !windows

package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

var onlyOneSignalHandler = make(chan struct{})
var sysSignals = []os.Signal{syscall.SIGUSR1, syscall.SIGHUP, os.Interrupt, syscall.SIGTERM}

// a WaitGroup waits for a collection of goroutines to finish, pass this by address
var waitGroup = sync.WaitGroup{}

// SetupSignalHandler registers for SIGTERM and SIGINT. A context is returned
// which is canceled on one of these signals. If a second signal is caught, the program
// is terminated with exit code 1.
func SetupSignalHandler(maxWorkers uint32) context.Context {
	close(onlyOneSignalHandler) // panics when called twice

	ctx, cancel := context.WithCancel(context.Background())
	waitGroup.Add(1 + int(maxWorkers)) // adds delta, if the counter becomes zero, all goroutines blocked on Wait are released

	c := make(chan os.Signal, 2)
	signal.Notify(c, sysSignals...)
	go func() {
		sig := <-c
		switch sig {
		case syscall.SIGUSR1:
			log.Println("caught USR1 signal")
			reloadAllHooks()

		case syscall.SIGHUP:
			log.Println("caught HUP signal")
			reloadAllHooks()

		case os.Interrupt, syscall.SIGTERM:
			log.Printf("caught %s signal; exiting\n", sig)
			cancel()
			if pidFile != nil {
				err := pidFile.Remove()
				if err != nil {
					log.Print(err)
				}
			}
			//os.Exit(0)
			//<-c
			waitGroup.Wait() // it blocks until the WaitGroup counter is zero
			os.Exit(1)       // second signal. Exit directly.

		default:
			log.Printf("caught unhandled signal %+v\n", sig)
		}
	}()

	return ctx
}

func setupSignals() {
	log.Printf("setting up os signal watcher\n")

	signals = make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGUSR1)
	signal.Notify(signals, syscall.SIGHUP)
	signal.Notify(signals, syscall.SIGTERM)
	signal.Notify(signals, os.Interrupt)

	go watchForSignals()
}

func watchForSignals() {
	log.Println("os signal watcher ready")

	for {
		sig := <-signals
		switch sig {
		case syscall.SIGUSR1:
			log.Println("caught USR1 signal")
			reloadAllHooks()

		case syscall.SIGHUP:
			log.Println("caught HUP signal")
			reloadAllHooks()

		case os.Interrupt, syscall.SIGTERM:
			log.Printf("caught %s signal; exiting\n", sig)
			if pidFile != nil {
				err := pidFile.Remove()
				if err != nil {
					log.Print(err)
				}
			}
			os.Exit(0)

		default:
			log.Printf("caught unhandled signal %+v\n", sig)
		}
	}
}
