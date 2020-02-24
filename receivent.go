package receivent

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/aws/aws-sdk-go/service/sqs/sqsiface"
	"io/ioutil"
	"net/http"
)

type EventProcessor interface {
	ProcessEvent(event []byte) error
}

type EventProcessorFunc func(event []byte) error

func (f EventProcessorFunc) ProcessEvent(e []byte) error {
	return f(e)
}

type Receiver struct {
	processor EventProcessor
}

func (receiver *Receiver) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	eventBody, err := ioutil.ReadAll(r.Body)
	defer r.Body.Close()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err = receiver.processor.ProcessEvent(eventBody)
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
				err := receiver.processor.ProcessEvent([]byte(aws.StringValue(message.Body)))
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
