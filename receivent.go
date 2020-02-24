package receivent

import (
	"encoding/json"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/aws/aws-sdk-go/service/sqs/sqsiface"
	"net/http"
	"strings"
)

type EventProcessor interface {
	ProcessEvent(event json.RawMessage) error
}

type EventProcessorFunc func(event json.RawMessage) error

func (f EventProcessorFunc) ProcessEvent(e json.RawMessage) error {
	return f(e)
}

type Receiver struct {
	processor EventProcessor
}

func (receiver *Receiver) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	event := json.RawMessage{}

	err := json.NewDecoder(r.Body).Decode(&event)
	defer r.Body.Close()
	if err != nil {
		http.Error(w, "JSON decoding failed", http.StatusBadRequest)
		return
	}

	err = receiver.processor.ProcessEvent(event)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (receiver *Receiver) StartSQSWorkerPool(sqsClient sqsiface.SQSAPI, queueURL string, parallelism int) error {
	pool := make(chan *sqs.Message)
	for i := 0; i < parallelism; i++ {
		go func() {
			for message := range pool {
				event := json.RawMessage{}
				_ = json.NewDecoder(strings.NewReader(aws.StringValue(message.Body))).Decode(&event)
				err := receiver.processor.ProcessEvent(event)
				if err == nil {
					_, _ = sqsClient.DeleteMessage(&sqs.DeleteMessageInput{
						QueueUrl:      aws.String(queueURL),
						ReceiptHandle: message.ReceiptHandle,
					})
				}
			}
		}()
	}
	for {
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

func New(processor EventProcessor) *Receiver {
	return &Receiver{processor: processor}
}
