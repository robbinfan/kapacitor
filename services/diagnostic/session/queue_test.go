package session_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/influxdata/kapacitor/services/diagnostic/internal/log"
	"github.com/influxdata/kapacitor/services/diagnostic/session"
)

func TestQueue(t *testing.T) {
	now := time.Now()
	data := []*session.Data{
		{
			Time:    now,
			Message: "0",
			Level:   "info",
			Fields:  []log.Field{log.String("test", "0")},
		},
		{
			Time:    now,
			Message: "1",
			Level:   "debug",
			Fields:  []log.Field{log.String("test", "1")},
		},
		{
			Time:    now,
			Message: "2",
			Level:   "warn",
			Fields:  []log.Field{log.String("test", "2")},
		},
		{
			Time:    now,
			Message: "3",
			Level:   "warn",
			Fields:  []log.Field{log.String("test", "3"), log.Int("number", 3)},
		},
	}

	q := &session.Queue{}
	// Verify null state
	if exp, got := 0, q.Len(); exp != got {
		t.Fatalf("expected length of queue to be %v, got %v", exp, got)
	}

	for i, d := range data {
		q.Enqueue(d)
		// Verify length of queue
		if exp, got := i+1, q.Len(); exp != got {
			t.Fatalf("expected length of queue to be %v, got %v", exp, got)
		}
	}

	for i, d := range data {
		// Verify contents of dequeue
		if exp, got := d, q.Dequeue(); !reflect.DeepEqual(exp, got) {
			t.Fatalf("expected %v\ngot: %v", exp, got)
		}
		// Verify length of queue
		if exp, got := len(data)-i-1, q.Len(); exp != got {
			t.Fatalf("expected length of queue to be %v, got %v", exp, got)
		}
	}

}

func TestEnqueDequeueDequeue(t *testing.T) {
	now := time.Now()
	data := &session.Data{
		Time:    now,
		Message: "0",
		Level:   "info",
		Fields:  []log.Field{log.String("test", "0")},
	}

	q := &session.Queue{}

	q.Enqueue(data)
	// Verify length of queue
	if exp, got := 1, q.Len(); exp != got {
		t.Fatalf("expected length of queue to be %v, got %v", exp, got)
	}

	if exp, got := data, q.Dequeue(); !reflect.DeepEqual(exp, got) {
		t.Fatalf("expected %v\ngot: %v", exp, got)
	}

	// Verify length of queue
	if exp, got := 0, q.Len(); exp != got {
		t.Fatalf("expected length of queue to be %v, got %v", exp, got)
	}

	var nilData *session.Data
	if exp, got := nilData, q.Dequeue(); !reflect.DeepEqual(nilData, got) {
		t.Fatalf("expected %v\ngot: %v", exp, got)
	}

	// Verify length of queue
	if exp, got := 0, q.Len(); exp != got {
		t.Fatalf("expected length of queue to be %v, got %v", exp, got)
	}

}
