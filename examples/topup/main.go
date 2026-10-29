// This example submits a billable airtime top-up when explicitly run.
package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
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
	phone := os.Getenv("TELBIZ_PHONE")
	amount, err := strconv.ParseInt(os.Getenv("TELBIZ_AMOUNT"), 10, 64)
	if phone == "" || err != nil || amount < 10000 {
		return fmt.Errorf("set TELBIZ_PHONE and TELBIZ_AMOUNT (whole LAK, at least 10000); running this example submits a billable top-up")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	client := telbiz.New() // Reads credentials once; authenticates on first use.
	defer client.Close()
	_, err = client.Topup(ctx, telbiz.TopupRequest{Phone: phone, Amount: amount})
	if err != nil {
		return err
	}
	fmt.Println("Top-up submission accepted; final airtime credit is unconfirmed.")
	return nil
}
