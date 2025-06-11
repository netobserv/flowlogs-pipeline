package prometheus

import (
	"container/list"
	"context"
	"sync"
	"time"
)

type Expiry struct {
	mu       sync.RWMutex
	dblnk    *list.List
	duration time.Duration
}

type entry struct {
	delFn func()
	t     time.Time
}

func NewExpiry(exp *time.Duration) Expiry {
	return Expiry{
		dblnk:    list.New(),
		duration: *exp,
	}
}

func (l *Expiry) add(delFn func()) *list.Element {
	e := &entry{
		delFn: delFn,
		t:     time.Now(),
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.dblnk.PushBack(e)
}

func (l *Expiry) tick(e *list.Element) {
	l.mu.Lock()
	defer l.mu.Unlock()
	ent := e.Value.(*entry)
	ent.t = time.Now()
	l.dblnk.MoveToBack(e)
}

// RunOnce runs the deletion callbacks.
func (l *Expiry) RunOnce() {
	expireTime := time.Now().Add(-l.duration)

	l.mu.Lock()
	defer l.mu.Unlock()
	for {
		e := l.dblnk.Front()
		if e == nil {
			return
		}
		ent := e.Value.(*entry)
		if ent.t.After(expireTime) {
			// no more expired items
			return
		}
		ent.delFn()
		l.dblnk.Remove(e)
	}
}

// RunEvery runs the deletion callbacks every `d` duration. A cancellable context can be passed.
func (l *Expiry) RunEvery(ctx context.Context, d time.Duration) {
	ticker := time.NewTicker(d)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				l.RunOnce()
			}
		}
	}()
}
