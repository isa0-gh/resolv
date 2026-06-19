package cache

import (
	"sync"
	"time"

	"github.com/miekg/dns"
)

type record struct {
	message   dns.Msg
	expiresAt time.Time
}

type CacheDB struct {
	mu  sync.Mutex
	db  map[string]record
	ttl time.Duration
	now func() time.Time
}

func New(ttl time.Duration) *CacheDB {
	return &CacheDB{
		db:  make(map[string]record),
		ttl: ttl,
		now: time.Now,
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
	req := new(dns.Msg)
	resp := new(dns.Msg)
	if err := req.Unpack(message); err != nil {
		return err
	}

	if err := resp.Unpack(response); err != nil {
		return err
	}

	key := QueryKey(req)
	if c.ttl <= 0 {
		c.mu.Lock()
		delete(c.db, key)
		c.mu.Unlock()
		return nil
	}

	c.mu.Lock()
	c.db[key] = record{
		message:   *resp,
		expiresAt: c.now().Add(c.ttl),
	}
	c.mu.Unlock()
	return nil
}

func (c *CacheDB) Get(message []byte) ([]byte, bool, error) {
	msg := new(dns.Msg)
	if err := msg.Unpack(message); err != nil {
		return nil, false, err
	}
	key := QueryKey(msg)
	c.mu.Lock()
	defer c.mu.Unlock()

	record, ok := c.db[key]
	if !ok {
		return nil, false, nil
	}

	if !c.now().Before(record.expiresAt) {
		delete(c.db, key)
		return nil, false, nil
	}

	resp := record.message
	resp.Id = msg.Id
	packed, err := resp.Pack()
	return packed, true, err
}
