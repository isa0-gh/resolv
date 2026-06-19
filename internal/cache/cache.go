package cache

import (
	"log/slog"
	"sync"
	"time"

	"github.com/miekg/dns"
)

type CacheDB struct {
	Mu sync.RWMutex
	DB map[string]dns.Msg
}

func New() *CacheDB {
	return &CacheDB{
		DB: make(map[string]dns.Msg),
	}
}

func QueryKey(message *dns.Msg) string {
	question := message.Question
	if len(question) == 0 {
		return ""
	}

	return question[0].Name + "|" + string(rune(question[0].Qtype))
}

func (c *CacheDB) Add(message []byte, response []byte) error {
	c.Mu.Lock()
	defer c.Mu.Unlock()

	req := new(dns.Msg)
	resp := new(dns.Msg)
	if err := req.Unpack(message); err != nil {
		return err
	}

	if err := resp.Unpack(response); err != nil {
		return err
	}

	key := QueryKey(req)
	c.DB[key] = *resp
	return nil
}

func (c *CacheDB) Get(message []byte) ([]byte, bool, error) {
	msg := new(dns.Msg)
	if err := msg.Unpack(message); err != nil {
		return nil, false, err
	}
	key := QueryKey(msg)
	c.Mu.RLock()
	defer c.Mu.RUnlock()
	for k, v := range c.DB {
		if k == key {
			v.Id = msg.Id
			resp, err := v.Pack()
			return resp, true, err
		}
	}
	return nil, false, nil
}

func (c *CacheDB) StartFlusher(ttl time.Duration) func() {
	if ttl <= 0 {
		return func() {}
	}

	ticker := time.NewTicker(ttl)
	done := make(chan struct{})
	var stopOnce sync.Once

	go func() {
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				c.Mu.Lock()
				c.DB = make(map[string]dns.Msg) // flush everything
				c.Mu.Unlock()
				slog.Info("Cache flushed")
			case <-done:
				return
			}
		}
	}()

	return func() {
		stopOnce.Do(func() {
			close(done)
		})
	}
}
