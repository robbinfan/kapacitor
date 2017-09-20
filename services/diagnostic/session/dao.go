package session

import (
	"errors"
	"sync"
	"time"

	"github.com/influxdata/kapacitor/services/diagnostic/internal/log"
	"github.com/influxdata/kapacitor/uuid"
)

const (
	pageSize = 10
	// TODO: what to make this value
	sessionExipryDuration = 20 * time.Second
)

type SessionsDAO interface {
	Create(tags []log.StringField) *Session
	Get(id string) (*Session, error)
}

type sessionKV struct {
	mu       sync.RWMutex
	sessions map[uuid.UUID]*Session
}

func (kv *sessionKV) Create(tags []log.StringField) *Session {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	s := &Session{
		id:       uuid.New(),
		deadline: time.Now().Add(sessionExipryDuration),
		tags:     tags,
		queue:    &Queue{},
	}

	kv.sessions[s.id] = s

	// TODO: register with Diagnostic service
	return s
}

func (kv *sessionKV) Get(id string) (*Session, error) {
	kv.mu.RLock()
	defer kv.mu.RUnlock()
	sid, err := uuid.Parse(id)
	if err != nil {
		return nil, err
	}

	s, ok := kv.sessions[sid]
	if !ok {
		return nil, errors.New("session not found")
	}

	if time.Now().After(s.deadline) {
		return nil, errors.New("session expired")
	}

	return s, nil
}

type Session struct {
	mu       sync.RWMutex
	id       uuid.UUID
	page     int
	deadline time.Time

	tags []log.StringField

	queue *Queue
}

func (s *Session) ID() string {
	return s.id.String()
}

func (s *Session) Deadline() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.deadline
}

func (s *Session) Page() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.page
}

func (s *Session) GetPage(page int) ([]*Data, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if page != s.page {
		return nil, errors.New("bad page value")
	}
	s.page++
	s.deadline = s.deadline.Add(sessionExipryDuration)

	l := make([]*Data, pageSize)
	for i := 0; i < pageSize; i++ {
		if s.queue.Len() == 0 {
			break
		}
		l = append(l, s.queue.Dequeue())
	}

	return l, nil
}
