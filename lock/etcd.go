package lock

import (
	"encoding/json"
	"errors"

	etcdError "github.com/coreos/locksmith/third_party/github.com/coreos/etcd/error"
	"github.com/coreos/locksmith/third_party/github.com/coreos/go-etcd/etcd"
)

const (
	keyPrefix       = "coreos.com/updateengine/rebootlock"
	holdersPrefix   = keyPrefix + "/holders"
	SemaphorePrefix = keyPrefix + "/semaphore"
)

// etcdInterface is a simple wrapper around the go-etcd client to facilitate testing
type etcdInterface interface {
	Create(key string, value string, ttl uint64) (*etcd.Response, error)
	CompareAndSwap(key string, value string, ttl uint64, prevValue string, prevIndex uint64) (*etcd.Response, error)
	Get(key string, sort, recursive bool) (*etcd.Response, error)
}

// EtcdLockClient is a wrapper around the go-etcd client that provides
// simple primitives to operate on the internal semaphore and holders
// structs through etcd.
type EtcdLockClient struct {
	client etcdInterface
}

func NewEtcdLockClient(machines []string) (client *EtcdLockClient, err error) {
	ec := etcd.NewClient(machines)
	client = &EtcdLockClient{ec}
	err = client.Init()

	return client, err
}

// Init sets an initial copy of the semaphore if it doesn't exist yet.
func (c *EtcdLockClient) Init() (err error) {
	sem := newSemaphore()
	b, err := json.Marshal(sem)
	if err != nil {
		return err
	}

	_, err = c.client.Create(SemaphorePrefix, string(b), 0)
	if err != nil {
		eerr, ok := err.(*etcd.EtcdError)
		if ok && eerr.ErrorCode == etcdError.EcodeNodeExist {
			return nil
		}
	}

	return err
}

// Get fetches the Semaphore from etcd.
func (c *EtcdLockClient) Get() (sem *Semaphore, err error) {
	resp, err := c.client.Get(SemaphorePrefix, false, false)
	if err != nil {
		return nil, err
	}

	sem = &Semaphore{}
	err = json.Unmarshal([]byte(resp.Node.Value), sem)
	if err != nil {
		return nil, err
	}

	sem.Index = resp.Node.ModifiedIndex

	return sem, nil
}

// Set sets a Semaphore in etcd.
func (c *EtcdLockClient) Set(sem *Semaphore) (err error) {
	if sem == nil {
		return errors.New("cannot set nil semaphore")
	}
	b, err := json.Marshal(sem)
	if err != nil {
		return err
	}

	_, err = c.client.CompareAndSwap(SemaphorePrefix, string(b), 0, "", sem.Index)

	return err
}
