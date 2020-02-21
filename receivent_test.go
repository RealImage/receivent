package receivent

import (
	"bytes"
	"encoding/json"
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
	ensureNoError(t, err)
	done := make(chan struct{})

	testServer := httptest.NewServer(New(EventProcessorFunc(func(input json.RawMessage) error {
		var e testEvent
		err = json.Unmarshal(input, &e)
		ensureNoError(t, err)
		if e.Id != event.Id {
			t.Fatal("Wrong event provided", e, string(input))
		}
		defer close(done)
		return nil
	})))
	resp, err := http.Post(testServer.URL, "application/json", bytes.NewReader(eventJSON))
	ensureNoError(t, err)
	if resp.StatusCode != 200 {
		t.Fatal("Server should complete cleanly")
	}
	<-done
}

func ensureNoError(t *testing.T, err error) {
	if err != nil {
		t.Fatal(err)
	}
}
