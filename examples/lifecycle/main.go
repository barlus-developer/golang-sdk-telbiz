// This example authenticates and handles shutdown; it submits no billable request.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	telbiz "github.com/barlus-developer/golang-sdk-telbiz"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	shutdown, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	startup, cancel := context.WithTimeout(shutdown, time.Minute)
	client, err := telbiz.NewWithContext(startup, telbiz.Config{ClientID: os.Getenv("TELBIZ_CLIENT_ID"), ClientSecret: os.Getenv("TELBIZ_CLIENT_SECRET")})
	cancel() // Successful initialization has an independent lifecycle.
	if err != nil {
		return err
	}
	defer client.Close()
	canceled, cancelWait := context.WithCancel(context.Background())
	cancelWait()
	if err := client.RefreshToken(canceled); !errors.Is(err, context.Canceled) {
		return fmt.Errorf("expected refresh wait cancellation")
	}
	fmt.Println("Client renews while idle. Press Ctrl+C to close it.")
	<-shutdown.Done()
	return client.Close() // Concurrent/repeated Close is safe.
}
