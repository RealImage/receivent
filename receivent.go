package receivent

import (
	"context"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/aws/aws-sdk-go/service/sqs/sqsiface"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

type EventProcessor interface {
	ProcessEvent(event []byte) error
}

type EventProcessorFunc func(event []byte) error

func (f EventProcessorFunc) ProcessEvent(e []byte) error {
	return f(e)
}

type Receiver interface {
	ServeHTTP(w http.ResponseWriter, r *http.Request)
	StartSQSWorkerPool(sqsClient sqsiface.SQSAPI, queueURL string, parallelism int)
	StartLambdaForSQS()
}

type receiver struct {
	processor EventProcessor
}

func (rc *receiver) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	eventBody, err := ioutil.ReadAll(r.Body)
	defer r.Body.Close()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err = rc.processor.ProcessEvent(eventBody)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (rc *receiver) StartSQSWorkerPool(sqsClient sqsiface.SQSAPI, queueURL string, parallelism int) {
	pool := make(chan *sqs.Message)
	for i := 0; i < parallelism; i++ {
		go func() {
			for message := range pool {
				err := rc.processor.ProcessEvent([]byte(aws.StringValue(message.Body)))
				if err == nil {
					_, _ = sqsClient.DeleteMessage(&sqs.DeleteMessageInput{
						QueueUrl:      aws.String(queueURL),
						ReceiptHandle: message.ReceiptHandle,
					})
				}
			}
		}()
	}
	// Before we the start the inifinite loop, let's listen for closing signals
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	for {
		select {
		case _, signalled := <-sigs:
			if signalled {
				return
			}
		default:
			messageOutput, _ := sqsClient.ReceiveMessage(&sqs.ReceiveMessageInput{
				MaxNumberOfMessages: aws.Int64(int64(parallelism)),
				QueueUrl:            aws.String(queueURL),
				WaitTimeSeconds:     aws.Int64(int64(20)),
			})
			for _, message := range messageOutput.Messages {
				pool <- message
			}
		}
	}
}

func (rc *receiver) StartLambdaForSQS() {
	lambda.Start(func(ctx context.Context, sqsEvent events.SQSEvent) error {
		// TODO parallelize
		for _, record := range sqsEvent.Records {
			err := rc.processor.ProcessEvent([]byte(record.Body))
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func New(processor EventProcessor) Receiver {
	return &receiver{processor: processor}
}
