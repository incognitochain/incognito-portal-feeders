package main

import (
	"fmt"
	"os"
	"os/signal"
	"portalfeeders/agents"
	"portalfeeders/utils"
	"runtime"
	"syscall"
	"time"

	"github.com/joho/godotenv"
)

type Server struct {
	quit   chan os.Signal
	finish chan bool
	agents []agents.Agent
}

func registerBTCRelayer(
	agentsList []agents.Agent,
) []agents.Agent {
	btcR := &agents.BTCRelayer{}
	btcR.ID = 1
	btcR.Name = "Bitcoin relayer"
	btcR.Frequency = 60
	btcR.Quit = make(chan bool)
	btcR.RPCClient = utils.NewHttpClient("", os.Getenv("INCOGNITO_PROTOCOL"), os.Getenv("INCOGNITO_HOST"), os.Getenv("INCOGNITO_PORT")) // incognito chain rpc endpoint
	btcR.Network = os.Getenv("BTC_NETWORK")                                                                                             // btc network name
	return append(agentsList, btcR)
}

func registerBNBRelayer(
	agentsList []agents.Agent,
) []agents.Agent {
	bnbR := &agents.BNBRelayer{}
	bnbR.ID = 2
	bnbR.Name = "Binance chain relayer"
	bnbR.Frequency = 60
	bnbR.Quit = make(chan bool)
	bnbR.RPCClient = utils.NewHttpClient("", os.Getenv("INCOGNITO_PROTOCOL"), os.Getenv("INCOGNITO_HOST"), os.Getenv("INCOGNITO_PORT")) // incognito chain rpc endpoint
	bnbR.Network = "test" // bnb network name
	return append(agentsList, bnbR)
}

func registerExchangeRatesRelayer(
	agentsList []agents.Agent,
) []agents.Agent {
	restfulClient := utils.NewRestfulClient(os.Getenv("COINMARKETCAP_HOST"), os.Getenv("COINMARKETCAP_VERSION"))

	exchangeRates := &agents.ExchangeRatesRelayer{}
	exchangeRates.ID = 3
	exchangeRates.Name = "Exchange rates relayer"
	exchangeRates.Frequency = 60
	exchangeRates.Quit = make(chan bool)
	exchangeRates.RPCClient = utils.NewHttpClient("", os.Getenv("INCOGNITO_PROTOCOL"), os.Getenv("INCOGNITO_HOST"), os.Getenv("INCOGNITO_PORT")) // incognito chain rpc endpoint
	exchangeRates.RestfulClient = restfulClient
	exchangeRates.Network = "main"
	return append(agentsList, exchangeRates)
}

func NewServer() *Server {
	agents := []agents.Agent{}
	agents = registerBTCRelayer(agents)
	agents = registerBNBRelayer(agents)
	agents = registerExchangeRatesRelayer(agents)

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
	err := godotenv.Load()
	if err != nil {
		panic("Error loading .env file")
	}

	runtime.GOMAXPROCS(runtime.NumCPU())
	s := NewServer()
	s.Run()
	for range s.agents {
		<-s.finish
	}
	fmt.Println("Server stopped gracefully!")
}
