package receivent

import "encoding/json"
import "net/http"

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

func (rc *Receiver) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	event := json.RawMessage{}

	err := json.NewDecoder(r.Body).Decode(&event)
	defer r.Body.Close()
	if err != nil {
		http.Error(w, "JSON decoding failed", http.StatusBadRequest)
		return
	}

	err = rc.processor.ProcessEvent(event)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func New(processor EventProcessor) *Receiver {
	return &Receiver{processor: processor}
}
