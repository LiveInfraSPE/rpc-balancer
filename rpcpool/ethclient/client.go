package ethclient

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/LiveInfraSPE/rpc-balancer/rpcpool/rpc"
	ec "github.com/ethereum/go-ethereum/ethclient"
	"github.com/golang/glog"
)

//go:generate go run ../../cmd/ethclient/generate.go

// NewClient manages a pool of ethclient.Client instances with retry logic.
type Client struct {
	rpcPool *rpc.Client
	mu      sync.Mutex
}

type Config = rpc.Config

// NewEthClient creates a new EthClientPool instance.
func NewEthClient(cfg rpc.Config) (*Client, error) {
	rpcClient, err := rpc.NewRPCClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create RPC pool: %w", err)
	}

	return &Client{
		rpcPool: rpcClient,
	}, nil
}

// Close closes all underlying clients
func (c *Client) Close() {
	c.rpcPool.Close()
}

// retry executes a function across clients with retry logic.
func (c *Client) retry(fn func(client *ec.Client) ([]interface{}, error), expectedResults int) ([]interface{}, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for {
		rpcClient, err := c.rpcPool.GetNextHealthyClient()
		if err != nil {
			return nil, fmt.Errorf("failed to get next healthy client: %w", err)
		}
		ethClient := ec.NewClient(rpcClient.Client)
		results, err := fn(ethClient)
		if err == nil {
			if expectedResults > 0 && len(results) != expectedResults {
				return nil, fmt.Errorf("expected %d results, got %d", expectedResults, len(results))
			}
			rpcClient.Reset()
			return results, nil
		}
		rpcClient.MarkUnhealthy()
		glog.V(6).Infof("rpcclient: failed response from client: %s: %v. Retrying with another client", rpcClient.Url, err)
	}
}

func (c *Client) Client() *rpc.Client {
	return c.rpcPool
}

func Dial(rawurl string) (*Client, error) {
	cfg := Config{
		RPCURLs:         rawurl,
		MaxBackoffDelay: 128 * time.Second,
	}
	return NewEthClient(cfg)
}

// DialContext creates a new PoolClient with a context for dialing.
func DialContext(ctx context.Context, rawurl string) (*Client, error) {
	cfg := Config{
		RPCURLs:         rawurl,
		MaxBackoffDelay: 128 * time.Second,
		Ctx:             ctx,
	}
	return NewEthClient(cfg)
}
