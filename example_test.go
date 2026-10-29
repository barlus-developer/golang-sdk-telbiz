package telbiz_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"

	telbiz "github.com/barlus-developer/golang-sdk-telbiz"
)

func ExampleClient_SendSMS() {
	// Executable documentation uses only a local mock with fabricated inputs.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/Connect/token" {
			io.WriteString(w, `{"accessToken":"fake-token","expire":3600,"code":"SUCCESS_OPERATION","success":true}`)
			return
		}
		io.WriteString(w, `{"response":{"code":"SENDING_SMS_INPROGRESS","success":true},"key":{"partitionKey":"","rangeKey":""}}`)
	}))
	defer server.Close()
	client := telbiz.New(telbiz.Config{ClientID: "fake-client", ClientSecret: "fake-secret", BaseURL: server.URL})
	defer client.Close()
	result, err := client.SendSMS(context.Background(), telbiz.SMSRequest{Title: telbiz.TitleInfo, Phone: "2012345678", Message: "Fabricated example"})
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(result.Response.Success, result.Response.Code)
	// Output: true SENDING_SMS_INPROGRESS
}
