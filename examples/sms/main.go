// This example submits a billable SMS when explicitly run with credentials.
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
	phone, message := os.Getenv("TELBIZ_PHONE"), os.Getenv("TELBIZ_MESSAGE")
	if phone == "" || message == "" {
		return fmt.Errorf("set TELBIZ_PHONE and TELBIZ_MESSAGE; running this example submits a billable SMS")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	client, err := telbiz.New(ctx, telbiz.Config{ClientID: os.Getenv("TELBIZ_CLIENT_ID"), ClientSecret: os.Getenv("TELBIZ_CLIENT_SECRET")})
	if err != nil {
		return err
	}
	defer client.Close()
	_, err = client.SendSMS(ctx, telbiz.SMSRequest{Title: telbiz.TitleOTP, Phone: phone, Message: message})
	if err != nil {
		return err
	}
	fmt.Println("SMS submission accepted; final delivery is unconfirmed.")
	return nil
}
