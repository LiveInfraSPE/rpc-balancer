package examples

import (
	"context"
	"testing"
	"time"

	ec "github.com/LiveInfraSPE/rpc-balancer/rpcpool/ethclient"
	rc "github.com/LiveInfraSPE/rpc-balancer/rpcpool/rpc"
	"github.com/ethereum/go-ethereum/common"
)

// ExampleNewPoolClient demonstrates creating a RPCPool client with multiple RPC URLs.
func TestNewEthClient(t *testing.T) {
	cfg := ec.Config{
		RPCURLs:         "http://localhost:3010/c3748b2af38304295e10dfeea8691eae13d23f249487a8d0c7bbb3084c69fead/,https://arbitrum.rpcgarage.xyz/5ba9f135ff1d2128a9884ed807ff38d7e84628b446442b30370824884cd432b9/",
		MaxBackoffDelay: 32 * time.Second,
	}
	client, err := ec.NewEthClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create PoolClient: %v\n", err)
	}
	defer client.Close()

	t.Log("PoolClient created successfully")
	// Output: PoolClient created successfully
}

func TestNewRPCClient(t *testing.T) {
	cfg := rc.Config{
		RPCURLs:         "http://localhost:3010/c3748b2af38304295e10dfeea8691eae13d23f249487a8d0c7bbb3084c69fead/,https://arbitrum.rpcgarage.xyz/5ba9f135ff1d2128a9884ed807ff38d7e84628b446442b30370824884cd432b9/",
		MaxBackoffDelay: 32 * time.Second,
	}
	client, err := rc.NewRPCClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create PoolClient: %v\n", err)
	}
	defer client.Close()

	t.Log("PoolClient created successfully")
	// Output: PoolClient created successfully
}

// ExamplePoolClient_CallContext demonstrates using CallContext to fetch the latest block number.
func TestClient_CallContext(t *testing.T) {
	cfg := ec.Config{
		RPCURLs:         "http://localhost:3010/c3748b2af38304295e10dfeea8691eae13d23f249487a8d0c7bbb3084c69fead/,https://arbitrum.rpcgarage.xyz/5ba9f135ff1d2128a9884ed807ff38d7e84628b446442b30370824884cd432b9/",
		MaxBackoffDelay: 32 * time.Second,
	}
	client, err := ec.NewEthClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create PoolClient: %v\n", err)
	}
	defer client.Close()

	var result string
	err = client.Client().CallContext(context.Background(), &result, "eth_blockNumber")
	if err != nil {
		t.Fatalf("CallContext failed: %v\n", err)
	}
	t.Logf("Latest block number: %s\n", result)
	if result != "" {
		t.Log("CallContext executed successfully")
	}
	// Output: CallContext executed successfully")
}

func TestRPCClient_CallContext(t *testing.T) {
	cfg := rc.Config{
		RPCURLs:         "http://localhost:3010/c3748b2af38304295e10dfeea8691eae13d23f249487a8d0c7bbb3084c69fead/,https://arbitrum.rpcgarage.xyz/5ba9f135ff1d2128a9884ed807ff38d7e84628b446442b30370824884cd432b9/",
		MaxBackoffDelay: 32 * time.Second,
	}
	client, err := rc.NewRPCClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create PoolClient: %v\n", err)
	}
	defer client.Close()

	var result string
	err = client.CallContext(context.Background(), &result, "eth_blockNumber")
	if err != nil {
		t.Fatalf("CallContext failed: %v\n", err)
	}
	t.Logf("Latest block number: %s\n", result)
	if result != "" {
		t.Log("CallContext executed successfully")
	}
	// Output: CallContext executed successfully")
}

// ExamplePoolClient_TransactionByHash demonstrates fetching a transaction by hash.
func TestClient_TransactionByHash(t *testing.T) {
	cfg := ec.Config{
		RPCURLs:         "http://localhost:3010/c3748b2af38304295e10dfeea8691eae13d23f249487a8d0c7bbb3084c69fead/,https://arbitrum.rpcgarage.xyz/5ba9f135ff1d2128a9884ed807ff38d7e84628b446442b30370824884cd432b9/",
		MaxBackoffDelay: 32 * time.Second,
	}
	client, err := ec.NewEthClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create PoolClient: %v\n", err)
	}
	defer client.Close()

	txHash := common.HexToHash("0xae68b055b31e293f43b92f6dbbdb1573201939f76dbec03477018dca61c0129f")
	tx, isPending, err := client.TransactionByHash(context.Background(), txHash)
	if err != nil {
		t.Fatalf("TransactionByHash failed: %v\n", err)
	}
	if isPending {
		t.Log("Transaction is pending")
	} else {
		t.Logf("Transaction found: %s\n", tx.Hash().Hex())
	}
}

// ExamplePoolClient_Client demonstrates accessing the underlying rpc.Client.
func TestClient_Client(t *testing.T) {
	cfg := ec.Config{
		RPCURLs:         "http://localhost:3010/c3748b2af38304295e10dfeea8691eae13d23f249487a8d0c7bbb3084c69fead/,https://arbitrum.rpcgarage.xyz/5ba9f135ff1d2128a9884ed807ff38d7e84628b446442b30370824884cd432b9/",
		MaxBackoffDelay: 32 * time.Second,
	}
	client, err := ec.NewEthClient(cfg)
	if err != nil {
		t.Fatalf("Failed to create PoolClient: %v\n", err)
	}
	defer client.Close()

	rpcClient := client.Client()

	t.Log("Retrieved rpc.Client successfully")
	// make an example call to demonstrate usage
	var result string
	err = rpcClient.CallContext(context.Background(), &result, "eth_blockNumber")
	if err != nil {
		t.Fatalf("CallContext failed: %v\n", err)
	}
	t.Logf("Latest block number: %s\n", result)
}
