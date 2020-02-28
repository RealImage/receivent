package receivent

import (
	"bytes"
	"encoding/json"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/aws/aws-sdk-go/service/sqs/sqsiface"
	"net/http"
	"net/http/httptest"
	"testing"
)

type testEvent struct {
	Id string `json:"id"`
}

func TestHTTPHandler(t *testing.T) {
	event := testEvent{Id: "abcd1234"}
	eventJSON, err := json.Marshal(event)
	failIfError(t, err)
	done := make(chan struct{})

	testServer := httptest.NewServer(New(EventProcessorFunc(func(input []byte) error {
		var e testEvent
		err = json.Unmarshal(input, &e)
		failIfError(t, err)
		if e.Id != event.Id {
			t.Fatal("Wrong event provided", e, string(input))
		}
		defer close(done)
		return nil
	})))
	resp, err := http.Post(testServer.URL, "application/json", bytes.NewReader(eventJSON))
	failIfError(t, err)
	if resp.StatusCode != 200 {
		t.Fatal("Request should complete cleanly")
	}
	<-done
}

type mockSQS struct {
	sqsiface.SQSAPI
	message  *sqs.Message
	url      string
	doneChan chan struct{}
	t        *testing.T
}

func (s mockSQS) ReceiveMessage(in *sqs.ReceiveMessageInput) (*sqs.ReceiveMessageOutput, error) {
	if aws.StringValue(in.QueueUrl) != s.url {
		s.t.Fatal("wrong queue URL requested", aws.StringValue(in.QueueUrl))
	}
	return &sqs.ReceiveMessageOutput{Messages: []*sqs.Message{s.message}}, nil
}

func (s mockSQS) DeleteMessage(in *sqs.DeleteMessageInput) (*sqs.DeleteMessageOutput, error) {
	if aws.StringValue(in.ReceiptHandle) != aws.StringValue(s.message.ReceiptHandle) {
		s.t.Fatal("wrong handle", aws.StringValue(in.ReceiptHandle))
	}
	s.doneChan <- struct{}{}
	return &sqs.DeleteMessageOutput{}, nil
}

func TestSQSWorkerPool(t *testing.T) {
	event := testEvent{Id: "abcd1234"}
	eventJSON, err := json.Marshal(event)
	failIfError(t, err)
	done := make(chan struct{})

	mockQueue := &mockSQS{
		message: &sqs.Message{
			Body:          aws.String(string(eventJSON)),
			ReceiptHandle: aws.String(string(eventJSON)),
		},
		url:      "queue1",
		doneChan: done,
		t:        t,
	}

	receiver := New(EventProcessorFunc(func(input []byte) error {
		var e testEvent
		err := json.Unmarshal(input, &e)
		failIfError(t, err)
		if e.Id != event.Id {
			t.Fatal("Wrong event provided", e, string(input))
		}
		return nil
	}))

	go receiver.StartSQSWorkerPool(mockQueue, "queue1", 1)

	failIfError(t, err)
	<-done
}

func failIfError(t *testing.T, err error) {
	if err != nil {
		t.Fatal(err)
	}
}
