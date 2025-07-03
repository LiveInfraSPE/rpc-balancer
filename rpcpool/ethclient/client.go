package ethclient

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	ec "github.com/ethereum/go-ethereum/ethclient"
	"github.com/golang/glog"
)

//go:generate go run ../../cmd/ethclient/generate.go
//go:generate go run ../../cmd/rpcclient/generate.go

// NewClient manages a pool of ethclient.Client instances with retry logic.
type Client struct {
	clients              []*ec.Client
	clientStatusTrackers []ClientStatusTracker
	mu                   sync.Mutex
}

type ClientStatusTracker struct {
	healthy      bool
	backoffDelay time.Duration
	maxDelay     time.Duration
	mu           sync.Mutex
}

func (cst *ClientStatusTracker) IsHealthy() bool {
	cst.mu.Lock()
	defer cst.mu.Unlock()
	return cst.healthy
}

func (cst *ClientStatusTracker) Reset() {
	cst.healthy = true
	cst.backoffDelay = 1 * time.Second // Reset to initial backoff delay
}

func (cst *ClientStatusTracker) MarkUnhealthy() {
	cst.mu.Lock()
	defer cst.mu.Unlock()
	cst.healthy = false
	if cst.backoffDelay < cst.maxDelay {
		cst.backoffDelay = cst.backoffDelay * 2
	}

	glog.V(4).Infof("rpcclient: marking client as unhealthy, retry delay: %s", cst.backoffDelay.String())
	go func() {
		<-time.After(cst.backoffDelay)
		cst.mu.Lock()
		defer cst.mu.Unlock()
		cst.healthy = true
	}()
}

// Config holds configuration for EthClientPool.
type Config struct {
	RPCURLs         string
	MaxBackoffDelay time.Duration
	Ctx             context.Context
}

func (pcc *Config) RPCURLsSlice() ([]string, error) {
	if pcc.RPCURLs == "" {
		return nil, fmt.Errorf("RPC URLs cannot be empty")
	}
	urls := strings.Split(pcc.RPCURLs, ",")
	for i := range urls {
		urls[i] = strings.TrimSpace(urls[i])
		if urls[i] == "" {
			return nil, fmt.Errorf("RPC URL cannot be empty")
		}
		if !strings.HasPrefix(urls[i], "http://") && !strings.HasPrefix(urls[i], "https://") && !strings.HasPrefix(urls[i], "wss://") && !strings.HasPrefix(urls[i], "ws://") {
			return nil, fmt.Errorf("invalid RPC URL: %s", urls[i])
		}
		if !strings.Contains(urls[i], "://") {
			return nil, fmt.Errorf("invalid RPC URL format: %s", urls[i])
		}
		urls[i] = strings.TrimSuffix(urls[i], "/") // Remove trailing slash if present
	}
	if len(urls) == 0 {
		return nil, fmt.Errorf("no valid RPC URLs provided")
	}
	return urls, nil
}

// NewRPCPool creates a new EthClientPool instance.
func NewRPCPool(cfg Config) (*Client, error) {
	urls, err := cfg.RPCURLsSlice()
	if err != nil {
		return nil, fmt.Errorf("invalid RPC URLs: %w", err)
	}
	if len(urls) == 0 {
		return nil, fmt.Errorf("at least one RPC URL is required")
	}

	clients := make([]*ec.Client, 0, len(urls))
	ClientStatusTrackers := make([]ClientStatusTracker, 0, len(urls))

	for _, url := range urls {
		var client *ec.Client
		var err error
		if cfg.Ctx != nil {
			client, err = ec.DialContext(cfg.Ctx, url)
		} else {
			client, err = ec.Dial(url)
		}
		if err != nil {
			glog.Errorf("rpcclient: failed to connect to %s: %v", url, err)
			continue
		}
		clients = append(clients, client)

		ClientStatusTrackers = append(ClientStatusTrackers, ClientStatusTracker{
			healthy:      true,
			backoffDelay: 1 * time.Second,
			maxDelay:     cfg.MaxBackoffDelay, // Example max delay for backoff
		})
	}

	if len(clients) == 0 {
		return nil, fmt.Errorf("no valid RPC clients connections")
	}

	return &Client{
		clients:              clients,
		clientStatusTrackers: ClientStatusTrackers,
	}, nil
}

// Close closes all underlying clients.
func (pc *Client) Close() {
	for _, client := range pc.clients {
		client.Close()
	}
}

// retry executes a function across clients with retry logic.
func (pc *Client) retry(fn func(client *ec.Client) ([]interface{}, error), expectedResults int) ([]interface{}, error) {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	for idx, client := range pc.clients {
		if !pc.clientStatusTrackers[idx].IsHealthy() {
			continue
		}
		results, err := fn(client)
		if err == nil {
			if expectedResults > 0 && len(results) != expectedResults {
				return nil, fmt.Errorf("expected %d results, got %d", expectedResults, len(results))
			}
			pc.clientStatusTrackers[idx].Reset()
			return results, nil
		}
		pc.clientStatusTrackers[idx].MarkUnhealthy()
		glog.V(6).Infof("rpcclient: failed response from client ID: %d: %v. Retrying with another client", idx, err)
	}
	return nil, fmt.Errorf("no healthy clients available to process the request after retries: %w", fmt.Errorf("all clients failed"))
}

func Dial(rawurl string) (*Client, error) {
	cfg := Config{
		RPCURLs:         rawurl,
		MaxBackoffDelay: 128 * time.Second,
	}
	return NewRPCPool(cfg)
}

// DialContext creates a new PoolClient with a context for dialing.
func DialContext(ctx context.Context, rawurl string) (*Client, error) {
	cfg := Config{
		RPCURLs:         rawurl,
		MaxBackoffDelay: 128 * time.Second,
		Ctx:             ctx,
	}
	return NewRPCPool(cfg)
}
