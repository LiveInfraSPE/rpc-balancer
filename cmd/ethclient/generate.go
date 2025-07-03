package main

import (
	cg "github.com/LiveInfraSPE/rpc-balancer/rpcpool/codegen"
)

func main() {
	cg.GenerateEthClient(cg.EthclientPkg, "ethclient.go", cg.EthClientWrapperTemplate, "ethmethods_generated.go")

}
