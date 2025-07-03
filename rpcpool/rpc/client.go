package rpc

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/rpc"
	"github.com/golang/glog"
)

//go:generate go run ../../cmd/rpcclient/generate.go

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

type poolClient struct {
	*rpc.Client
	*ClientStatusTracker
	Url string
}

// NewClient manages a pool of ethclient.Client instances with retry logic.
type Client struct {
	poolClients []*poolClient
	mu          sync.Mutex
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

// NewRPCClient creates a new EthClientPool instance.
func NewRPCClient(cfg Config) (*Client, error) {
	urls, err := cfg.RPCURLsSlice()
	if err != nil {
		return nil, fmt.Errorf("invalid RPC URLs: %w", err)
	}
	if len(urls) == 0 {
		return nil, fmt.Errorf("at least one RPC URL is required")
	}

	poolclients := make([]*poolClient, 0, len(urls))

	for _, url := range urls {
		var rpcclient *rpc.Client
		var poolclient *poolClient
		var err error
		if cfg.Ctx != nil {
			rpcclient, err = rpc.DialContext(cfg.Ctx, url)
		} else {
			rpcclient, err = rpc.Dial(url)
		}
		if err != nil {
			glog.Errorf("rpcclient: failed to connect to %s: %v", url, err)
			continue
		}
		poolclient = &poolClient{
			Client: rpcclient,
			ClientStatusTracker: &ClientStatusTracker{
				healthy:      true,
				backoffDelay: 1 * time.Second,
				maxDelay:     cfg.MaxBackoffDelay, // Example max delay for backoff
			},
			Url: url,
		}
		poolclients = append(poolclients, poolclient)
	}

	if len(poolclients) == 0 {
		return nil, fmt.Errorf("no valid RPC clients connections")
	}

	return &Client{
		poolClients: poolclients,
	}, nil
}

// Close closes all underlying clients.
func (pc *Client) Close() {
	for _, client := range pc.poolClients {
		client.Close()
	}
}

// retry executes a function across clients with retry logic.
func (pc *Client) retry(fn func(client *rpc.Client) ([]interface{}, error), expectedResults int) ([]interface{}, error) {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	for {
		poolClient, err := pc.GetNextHealthyClient()
		if err != nil {
			glog.V(6).Infof("rpcclient: %v", err)
			return nil, fmt.Errorf("rpcclient: %v", err)

		}
		results, err := fn(poolClient.Client)
		if err == nil {
			if expectedResults > 0 && len(results) != expectedResults {
				return nil, fmt.Errorf("expected %d results, got %d", expectedResults, len(results))
			}
			poolClient.Reset()
			return results, nil
		}
		poolClient.MarkUnhealthy()
		glog.V(6).Infof("rpcclient: failed response from client: %%s: %v. Retrying with another client", poolClient.Url, err)
	}
}

func (pc *Client) GetNextHealthyClient() (*poolClient, error) {
	for _, client := range pc.poolClients {
		if client.IsHealthy() {
			return client, nil
		}
	}
	return nil, fmt.Errorf("no healthy clients available")
}

func Dial(rawurl string) (*Client, error) {
	cfg := Config{
		RPCURLs:         rawurl,
		MaxBackoffDelay: 128 * time.Second,
	}
	return NewRPCClient(cfg)
}

// DialContext creates a new PoolClient with a context for dialing.
func DialContext(ctx context.Context, rawurl string) (*Client, error) {
	cfg := Config{
		RPCURLs:         rawurl,
		MaxBackoffDelay: 128 * time.Second,
		Ctx:             ctx,
	}
	return NewRPCClient(cfg)
}
