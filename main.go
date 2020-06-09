package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"portalfeeders/agents"
	"portalfeeders/utils"
	"runtime"
	"syscall"
	"time"

	"github.com/tendermint/tendermint/rpc/client"

	"github.com/btcsuite/btcd/rpcclient"
	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
)

type Server struct {
	quit   chan os.Signal
	finish chan bool
	agents []agents.Agent
}

func instantiateLogger(workerName string) (*logrus.Entry, error) {
	var log = logrus.New()
	logsPath := filepath.Join(".", "logs")
	os.MkdirAll(logsPath, os.ModePerm)
	file, err := os.OpenFile(fmt.Sprintf("%s/%s.log", logsPath, workerName), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Infof("Failed to log to file - with error: %v", err)
		return nil, err
	}
	log.Out = file
	logger := log.WithFields(logrus.Fields{
		"worker": workerName,
	})
	return logger, nil
}

func buildBTCClient() (*rpcclient.Client, error) {
	connCfg := &rpcclient.ConnConfig{
		Host:         fmt.Sprintf("%s:%s", os.Getenv("BTC_NODE_HOST"), os.Getenv("BTC_NODE_PORT")),
		User:         os.Getenv("BTC_NODE_USERNAME"),
		Pass:         os.Getenv("BTC_NODE_PASSWORD"),
		HTTPPostMode: true, // Bitcoin core only supports HTTP POST mode
		DisableTLS:   true, // Bitcoin core does not provide TLS by default
	}
	return rpcclient.New(connCfg, nil)
}

func registerBTCRelayer(
	agentsList []agents.Agent,
) []agents.Agent {
	btcR := &agents.BTCRelayerV2{}
	btcR.ID = 1
	btcR.Name = "bitcoin-relayer"
	btcR.Frequency = 60
	btcR.Quit = make(chan bool)
	btcR.RPCBTCRelayingReader = utils.NewHttpClient(
		"",
		os.Getenv("INCOGNITO_READER_PROTOCOL"),
		os.Getenv("INCOGNITO_READER_HOST"),
		os.Getenv("INCOGNITO_READER_PORT"),
	) // incognito chain reader rpc endpoint
	btcR.RPCClient = utils.NewHttpClient(
		"",
		os.Getenv("INCOGNITO_PROTOCOL"),
		os.Getenv("INCOGNITO_HOST"),
		os.Getenv("INCOGNITO_PORT"),
	) // incognito chain rpc endpoint
	btcR.Network = os.Getenv("BTC_NETWORK") // btc network name
	logger, err := instantiateLogger(btcR.Name)
	if err != nil {
		panic("Could instantiate a logger for bitcoin relayer")
	}
	btcR.Logger = logger

	btcClient, err := buildBTCClient()
	// defer btcClient.Shutdown()
	if err != nil {
		panic("Could instantiate btc client for bitcoin relayer")
	}
	btcR.BTCClient = btcClient
	return append(agentsList, btcR)
}

func registerBTCRelayingAlerter(
	agentsList []agents.Agent,
) []agents.Agent {
	btcR := &agents.BTCRelayingAlerter{}
	btcR.ID = 1
	btcR.Name = "bitcoin-relaying-alerter"
	btcR.Frequency = 600
	btcR.Quit = make(chan bool)
	btcR.RPCBTCRelayingReader = utils.NewHttpClient(
		"",
		os.Getenv("INCOGNITO_READER_PROTOCOL"),
		os.Getenv("INCOGNITO_READER_HOST"),
		os.Getenv("INCOGNITO_READER_PORT"),
	) // incognito chain reader rpc endpoint
	btcR.RPCClient = utils.NewHttpClient(
		"",
		os.Getenv("INCOGNITO_PROTOCOL"),
		os.Getenv("INCOGNITO_HOST"),
		os.Getenv("INCOGNITO_PORT"),
	) // incognito chain rpc endpoint
	btcR.Network = os.Getenv("BTC_NETWORK") // btc network name
	logger, err := instantiateLogger(btcR.Name)
	if err != nil {
		panic("Could instantiate a logger for bitcoin relaying alerter")
	}
	btcR.Logger = logger
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
	bnbR.Network = "test"                                                                                                               // bnb network name
	bnbR.BNBClient = client.NewHTTP(bnbR.GetServerAddress(), "/websocket")
	return append(agentsList, bnbR)
}

func registerExchangeRatesRelayer(
	agentsList []agents.Agent,
) []agents.Agent {
	restfulClient := utils.NewRestfulClient(os.Getenv("BINNANCE_HOST"), os.Getenv("BINNANCE_VERSION"))

	exchangeRates := &agents.ExchangeRatesRelayer{}
	exchangeRates.ID = 3
	exchangeRates.Name = "exchange-rates-relayer"
	exchangeRates.Frequency = 5
	exchangeRates.Quit = make(chan bool)
	exchangeRates.RPCClient = utils.NewHttpClient("", os.Getenv("INCOGNITO_PROTOCOL"), os.Getenv("INCOGNITO_HOST"), os.Getenv("INCOGNITO_PORT")) // incognito chain rpc endpoint
	exchangeRates.RestfulClient = restfulClient
	exchangeRates.Network = "main"
	logger, err := instantiateLogger(exchangeRates.Name)
	if err != nil {
		panic("Could instantiate a logger for exchange rates relayer")
	}
	exchangeRates.Logger = logger
	return append(agentsList, exchangeRates)
}

func NewServer() *Server {
	agents := []agents.Agent{}
	agents = registerBTCRelayer(agents)
	// agents = registerBTCRelayingAlerter(agents)
	// agents = registerBNBRelayer(agents)
	// agents = registerExchangeRatesRelayer(agents)

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

	var myEnv map[string]string
	myEnv, _ = godotenv.Read()
	fmt.Println("=========Config============")
	for key, value := range myEnv {
		fmt.Println(key + ": " + value)
	}
	fmt.Println("=========End============")

	runtime.GOMAXPROCS(runtime.NumCPU())
	s := NewServer()

	// split utxos before executing agents
	// if os.Getenv("SPLITUTXO") == "true" {
	// 	incognitoPrivateKey := os.Getenv("INCOGNITO_PRIVATE_KEY")
	// 	minNumUTXOTmp := os.Getenv("NUMUTXO")
	// 	minNumUTXOs, _ := strconv.Atoi(minNumUTXOTmp)

	// 	rpcClient := utils.NewHttpClient("", os.Getenv("INCOGNITO_PROTOCOL"), os.Getenv("INCOGNITO_HOST"), os.Getenv("INCOGNITO_PORT")) // incognito chain rpc endpoint
	// 	err := agents.SplitUTXOs(rpcClient, incognitoPrivateKey, minNumUTXOs)
	// 	if err != nil {
	// 		fmt.Printf("Split utxos error: %v\n", err)
	// 		return
	// 	}
	// }

	s.Run()
	for range s.agents {
		<-s.finish
	}
	fmt.Println("Server stopped gracefully!")
}
