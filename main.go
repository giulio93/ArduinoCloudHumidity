package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/antihax/optional"
	iot "github.com/arduino/iot-client-go"
	cc "golang.org/x/oauth2/clientcredentials"
)

type Post struct {
	Id     int    `json:"id"`
	Title  string `json:"title"`
	Body   string `json:"body"`
	UserId int    `json:"userId"`
}

func main() {

	clientID := os.Getenv("clientID")
	clientSecret := os.Getenv("clientSecret")
	audience := os.Getenv("audience")
	tokenUrl := os.Getenv("tokenUrl")
	thingID := os.Getenv("thingID")
	pid := os.Getenv("pid")
	sensorID := os.Getenv("sensorID")
	// We need to pass the additional "audience" var to request an access token
	additionalValues := url.Values{}
	additionalValues.Add("audience", audience)
	// Set up OAuth2 configuration
	config := cc.Config{
		ClientID:       clientID,
		ClientSecret:   clientSecret,
		TokenURL:       tokenUrl,
		EndpointParams: additionalValues,
	}
	// Get the access token in exchange of client_id and client_secret
	tok, err := config.Token(context.Background())
	if err != nil {
		log.Fatalf("Error retrieving access token, %v", err)
	}
	// Confirm we got the token and print expiration time
	log.Printf("Got an access token, will expire on %s", tok.Expiry)

	// We use the token to create a context that will be passed to any API call
	ctx := context.WithValue(context.Background(), iot.ContextAccessToken, tok.AccessToken)

	// Create an instance of the iot-api Go client, we pass an empty config
	// because defaults are ok
	client := iot.NewAPIClient(iot.NewConfiguration())

	// Get the list of devices for the current user
	devices, _, err := client.DevicesV2Api.DevicesV2List(ctx, nil)
	if err != nil {
		log.Fatalf("Error getting devices, %v", err)
	}

	// Print a meaningful message if the api call succeeded
	if len(devices) == 0 {
		log.Printf("No device found")
	} else {
		for _, d := range devices {
			log.Printf("Device found: %s", d.Name)
		}
	}

	thing, _, err := client.ThingsV2Api.ThingsV2Show(ctx, thingID, nil)
	if err != nil {
		log.Fatalf("Error getting thing %s, %v", thingID, err)
	}

	opts := iot.PropertiesV2TimeseriesOpts{
		From: optional.NewString(time.Now().Add(-100 * time.Minute).Format("2006-01-02T15:04:05Z")),
	}

	data, _, err := client.PropertiesV2Api.PropertiesV2Timeseries(ctx, thingID, pid, &opts)
	if err != nil {
		log.Fatalf("Error getting prop, %v, %s", data, err)
	} else {
		log.Printf("Prop found, last humidity is : %v", data)
	}

	var meanValue float64

	for _, d := range data.Data {
		meanValue = meanValue + d.Value
	}
	formTOPala(thing.Id, sensorID, meanValue/float64(len(data.Data)), time.Now())

}

func formTOPala(thingID string, propID string, value float64, timestamp time.Time) {
	// Define the endpoint
	url := os.Getenv("postUrl")

	// Create a new buffer to hold the multipart form data
	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)

	// Get the Unix timestamp
	unixTimestamp := timestamp.Unix()

	// Add the form fields
	formFields := map[string]string{
		"apikey":    thingID,
		"iddevice":  propID,
		"value":     fmt.Sprintf("%v", int(value*10)),
		"type":      "2",
		"timestamp": fmt.Sprintf("%v", unixTimestamp),
	}

	for key, value := range formFields {
		err := writer.WriteField(key, value)
		if err != nil {
			fmt.Printf("Error adding field %s: %v\n", key, err)
			os.Exit(1)
		}
	}

	// Close the writer to finalize the form data
	err := writer.Close()
	if err != nil {
		fmt.Println("Error closing writer:", err)
		os.Exit(1)
	}

	// Create a new HTTP request
	req, err := http.NewRequest("POST", url, &requestBody)
	if err != nil {
		fmt.Println("Error creating request:", err)
		os.Exit(1)
	}

	// Set the Content-Type header, including the boundary of the multipart form
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Send the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error sending request:", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	// Print the response status
	fmt.Println("Response status:", resp.Status)
}
