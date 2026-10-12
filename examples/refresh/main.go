// This example authenticates and manually renews; it submits no SMS or top-up.
package main

import (
	"context"
	"fmt"
	"os"
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
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	client, err := telbiz.New(ctx, telbiz.Config{ClientID: os.Getenv("TELBIZ_CLIENT_ID"), ClientSecret: os.Getenv("TELBIZ_CLIENT_SECRET")})
	if err != nil {
		return err
	}
	defer client.Close()
	if err := client.RefreshToken(ctx); err != nil {
		return err
	}
	fmt.Println("Access token renewed using the same credentials.")
	return nil
}
