package main

import (
	"fmt"
	"portalfeeders/agents"
	"portalfeeders/utils"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"
)

type Server struct {
	quit   chan os.Signal
	finish chan bool
	agents []agents.Agent
}

func NewServer() *Server {
	rpcClient := utils.NewHttpClient()
	restfulClient := utils.NewRestfulClient(agents.CoinMarketCapHost, agents.CoinMarketCapVersion)

	btcR := &agents.BTCRelayer{}
	btcR.ID = 1
	btcR.Name = "Bitcoin relayer"
	btcR.Frequency = 3
	btcR.Quit = make(chan bool)
	btcR.RPCClient = rpcClient
	btcR.Network = "main"

	bnbR := &agents.BNBRelayer{}
	bnbR.ID = 2
	bnbR.Name = "Binance chain relayer"
	bnbR.Frequency = 2
	bnbR.Quit = make(chan bool)
	bnbR.RPCClient = rpcClient
	bnbR.Network = "main"

	exchangeRates:= &agents.ExchangeRatesRelayer{}
	exchangeRates.ID = 3
	exchangeRates.Name = "Exchange rates relayer"
	exchangeRates.Frequency = 10
	exchangeRates.Quit = make(chan bool)
	exchangeRates.RPCClient = rpcClient
	exchangeRates.RestfulClient = restfulClient
	exchangeRates.Network = "main"


	//agents := []agents.Agent{btcR, bnbR, exchangeRates}
	agents := []agents.Agent{exchangeRates}
	quitChan := make(chan os.Signal)
	signal.Notify(quitChan, syscall.SIGTERM)
	signal.Notify(quitChan, syscall.SIGINT)
	return &Server{
		quit:   quitChan,
		finish: make(chan bool, len(agents)),
		agents: agents,
	}
}

func (s *Server) NotifyQuitSignal(agents []agents.Agent) {
	sig := <-s.quit
	fmt.Printf("Caught sig: %+v \n", sig)
	// notify all agents about quit signal
	for _, a := range agents {
		a.GetQuitChan() <- true
	}
}

func (s *Server) Run() {
	agents := s.agents
	go s.NotifyQuitSignal(agents)
	for _, a := range agents {
		go executeAgent(s.finish, a)
	}
}

func executeAgent(
	finish chan bool,
	agent agents.Agent,
) {
	agent.Execute() // execute as soon as starting up
	for {
		select {
		case <-agent.GetQuitChan():
			fmt.Printf("Finishing task for %s ...\n", agent.GetName())
			time.Sleep(time.Second * 1)
			fmt.Printf("Task for %s done! \n", agent.GetName())
			finish <- true
			break
		case <-time.After(time.Duration(agent.GetFrequency()) * time.Second):
			agent.Execute()
		}
	}
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	s := NewServer()
	s.Run()
	for range s.agents {
		<-s.finish
	}
	fmt.Println("Server stopped gracefully!")
}
