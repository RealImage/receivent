# Receivent
Event receiver library in Go. Suitable for receiving events over SQS, HTTP, SNS and Lambda. 

When using an event driven architecture like Event Sourcing, it makes a lot of sense for each event handling microservice to be able to receive events off a reliable queue for production, with the same processing happening off HTTP for testing. This library enables setting up an event receiver that provides the following interfaces:

* [**HTTP(S) Handler**](https://golang.org/pkg/net/http/#Handler) for receiving events as JSON via POST requests and acknowledges them with 200 OK when processed without error. You can mount this at whatever path makes sense for your application. 
* [**SNS HTTP(S) Handler**](https://golang.org/pkg/net/http/#Handler) that validates SNS messages and acknowledges them correctly when processed without error. You can mount this at whatever path in your application is configured as the SNS receiver. 
* **SQS Worker Pool** with configurable parallelism that fetches and correctly acknowledges each message when processed without error. 
* [**Lambda Handler**](https://docs.aws.amazon.com/lambda/latest/dg/with-sqs-create-package.html#with-sqs-example-deployment-pkg-go) that allows for easy Lambda deployment and correctly acknowledges messages processed without error. 

## Usage
```go

// The event processor interface has a single method: 
type EventProcessor interface {
	ProcessEvent(event []byte) error
}

// You can also wrap a function to be a processor:
processor := receivent.EventProcessorFunc(
    func(event []byte) error {
        return nil
    })

// Create a receiver by passing in an EventProcessor
receiver := receivent.New(processor)
    
// Start the SQS worker pool with 15 goroutines
sess := session.Must(session.NewSession())
sqsClient := sqs.New(sess)

go receiver.StartSQSWorkerPool(sqsClient, queueURL, 15)

// Hand event POSTs into /receive
http.Handle("/receive", receiver)

// Set up SNS receiver
http.Handle("/sns", receiver.SNSHandler)

// If you're using a pure Lambda
lambda.Start(receiver.LambdaHandler)
```

## Concepts
You need to provide a single processor function of the interface `ProcessEvent(event json.RawMessage) (err)`. All interfaces will call this method and acknowledge messages successfully when `err` is `nil`. The `ProcessEvent` method must be safe for calling from multiple goroutines. 

This simplifes development, testing and deployment, becuase now each microservice essentially boils down to one method that does the required work and emits its own events if required. 

One way of maximizing effectiveness is to use the following pattern when dealing with events:
* Use the `ProcessEvent` method to immediately store the event in a database as a record of *what has happened*. Note that `ProcessEvent` may be called multiples times per event or out of order w.r.t events received, so the events and the processors need to be designed to handle this.
* Spawn a new goroutine to examine what has happned and decide *what to do next*, writing that down in the database. This can be made idempotent by joining or checking the associations between what has happened and what to do next to see if actions have already been recorded, or by making the ID of the action deterministic based on the inputs to it - see [Content Based UULID](https://github.com/sudhirj/uulid.go). 
* Spawn a new goroutine to do what's been commited as what to do next. After doing it, the goroutine can mark the work as completed in the databse. To prevent multiple goroutines from doing the same work at the same time, locks can be used (pg_advisory_locks, redis locks), but the events need to be designed to be idempotent in either case. 

If the pattern above is followed, each microservice can be deployed as a combination of one or more of the above interfaces depending on the environment and execution environment availalbe. 
