package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ses"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

var emailSent bool = false
var appStartTime time.Time = time.Now().UTC()

func main() {
	logFile, err := os.OpenFile("app.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("Failed to open log file: %s", err)
	}
	defer logFile.Close()

	// Configure the logger to write to the file
	log.SetOutput(logFile)

	go pingHttpServer()

	for {

		checkServiceRunning()
		time.Sleep(1 * time.Minute)
	}

}

// pingHandler handles requests to the /ping endpoint
func pingHandler(w http.ResponseWriter, r *http.Request) {
	// Set response content-type to plain text
	w.Header().Set("Content-Type", "text/plain")
	// Write "pong" to the response
	fmt.Fprintf(w, "pong")
}

func pingHttpServer() {

	http.HandleFunc("/ping", pingHandler)

	// Define the address and port to listen on
	addr := ":8080"
	log.Printf("Starting server on %s\n", addr)

	// Start the server
	err := http.ListenAndServe(addr, nil)
	if err != nil {
		log.Fatalf("Could not start server: %s\n", err)
	}
}

func checkServiceRunning() {
	// Replace "ServiceName" with the actual name of the service you want to check
	serviceName := "RabbitMQ"

	// Connect to the Service Control Manager
	mgr, err := mgr.Connect()
	if err != nil {
		log.Println("Failed to connect to Service Control Manager:", err)
		return
	}
	defer mgr.Disconnect()

	// Open the service
	service, err := mgr.OpenService(serviceName)
	if err != nil {
		log.Printf("Failed to open service '%s': %v\n", serviceName, err)
		sendEmail()
		return
	}
	defer service.Close()

	// Get the service status
	status, err := service.Query()
	if err != nil {
		log.Printf("Failed to query service '%s': %v\n", serviceName, err)
		return
	}

	// Check if the service is running
	if status.State == svc.Running {
		log.Printf("Service '%s' is running.\n", serviceName)

	} else {
		log.Printf("Service '%s' is not running (state: %d).\n", serviceName, status.State)
		sendEmail()
	}
}

func sendEmail() {
	if time.Since(appStartTime) > 60*time.Minute {
		log.Println("Last email was sent an hour ago and rabbit mq is still not responding, sending an email")
		emailSent = false
		appStartTime = time.Now().UTC()
	}
	if emailSent {
		log.Println("Not Sending Email as email already sent")
		return
	}

	hostname, err := os.Hostname()
	if err != nil {
		log.Printf("Failed to get hostname: %s", err)
	}

	// Print the hostname
	fmt.Printf("Hostname: %s\n", hostname)

	// Create a new AWS session
	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String("ap-south-1"), // Replace with your desired AWS region
		Credentials: credentials.NewEnvCredentials(),
	})
	if err != nil {
		log.Println("Error creating AWS session:", err)
		return
	}

	// Create an SES client
	svc := ses.New(sess)

	ip := getIpAddress()

	// Define email parameters
	input := &ses.SendEmailInput{
		Destination: &ses.Destination{
			ToAddresses: []*string{
				aws.String("nikhils@readywire.com"),
				aws.String("sanjayp@readywire.com"),
				aws.String("arpithak@readywire.com"),
				aws.String("amans@readywire.com"),
				aws.String("helpdesk@kensium.com"),
			},
		},
		Message: &ses.Message{
			Body: &ses.Body{
				Text: &ses.Content{
					Charset: aws.String("UTF-8"),
					Data:    aws.String(fmt.Sprintf("RabbitMQ services are down for instance %v, IP: %v", hostname, ip)),
				},
			},
			Subject: &ses.Content{
				Charset: aws.String("UTF-8"),
				Data:    aws.String("ALERT: RabbitMQ is down"),
			},
		},
		Source: aws.String("admin01@readywire.com"),
	}

	// Send the email
	result, err := svc.SendEmail(input)
	if err != nil {
		log.Println("Error sending email:", err)
		return
	}

	log.Println("Email sent successfully:", result.MessageId)
	emailSent = true
	appStartTime = time.Now().UTC()
}

func getIpAddress() string {
	resp, err := http.Get("https://api.ipify.org?format=text")
	if err != nil {
		fmt.Println("Error:", err)
		return ""
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error:", err)
		return ""
	}

	fmt.Println("Public IP address:", string(body))

	return string(body)
}