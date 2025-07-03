package main

import (
	cg "github.com/LiveInfraSPE/rpc-balancer/rpcpool/codegen"
)

func main() {

	cg.GenerateEthClient(cg.RpcclientPkg, "client.go", cg.RpcClientWrapperTemplate, "rpcmethods_generated.go")

}
