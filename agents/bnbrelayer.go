package agents

import "fmt"

type BNBRelayer struct {
	AgentAbs
}

func (b *BNBRelayer) Execute() {
	fmt.Println("BNBRelayer agent is executing...")
}
