# receivent
Event receiver library in Go. Suitable for receiving events over SQS, HTTP, SNS and Lambda. 

When using an event driven architecture like Event Sourcing, it makes a lot of sense for each event handling microservice to be able to receive events off a reliable queue for production, with the same processing happening off HTTP for testing. This library enables setting up an event receiver that provides the following interfaces:

* [**HTTP(S) Handler**](https://golang.org/pkg/net/http/#Handler) for receiving events as JSON via POST requests and acknowledges them with 200 OK when processed without error. You can mount this at whatever path makes sense for your application. 
* [**SNS HTTP(S) Handler**](https://golang.org/pkg/net/http/#Handler) that validates SNS messages and acknowledges them correctly when processed without error. You can mount this at whatever path in your application is configured as the SNS receiver. 
* **SQS Worker Pool** with conigurable parallelism that fetches and correctly acknowledges each message when processed without error. 
* [**Lambda Handler**](https://docs.aws.amazon.com/lambda/latest/dg/with-sqs-create-package.html#with-sqs-example-deployment-pkg-go) that allows for easy Lambda deployment and correctly acknowledges messages processed without error. 

You need to provide a single processor function of the interface `ProcessEvent(event json.RawMessage, emitter EventEmitter) (err)`. All interfaces will call this method and acknowledge messages successfully when `err` is `nil`. The `ProcessEvent` method must be safe for calling from multiple goroutines. 

This simplifes development, testing and deployment, becuase now each microservice essentially boils down to one method that does the required work and emits its own events if required. 

The `EventEmitter` passed in to the processor is not guaranteed to succeed, so it safe to hold on to it and send events on a separate goroutine based off a committed record like a database. 

One way of maximizing effectiveness is to use the following pattern when dealing with events:
* Use the `ProcessEvent` method to immediately store the event in a database as a record of *what has happened*. 
* Spawn a new goroutine to examine what has happned and decide *what to do next*, writing that down in the database. This can be made idempotent by joining or checking the associations between what has happened and what to do next to see actions have already been recorded. 
* Spawn a new goroutine to do what's been commited as what to do next. After doing it, the goroutine can mark the work as completed in the databse. To prevent multiple goroutines from doing the same work at the same time, locks can be used (pg_advisory_locks, redis locks), but the events need to be designed to be idempotent in either case. 

