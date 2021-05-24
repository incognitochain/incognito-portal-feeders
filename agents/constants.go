package agents

type PToken struct {
	name     string
	tokenID  string
	decimals uint64
}

const (
	// DefaultFee - default fee
	DefaultFee = 20

	// DefaultExecuteDuration - default execute duration
	DefaultExecuteDuration = 100

	// decimals
	USDdecimals     = 1e9
	ConvertDecimals = 1e3

	//Crypto ID
	BTCSymbol = "BTCUSDT"
	BNBSymbol = "BNBUSDT"

	BTCID   = "ef5947f70ead81a76a53c7c8b7317dd5245510c665d3a13921dc9a581188728b"
	BNBID   = "6abd698ea7ddd1f98b1ecaaddab5db0453b8363ff092f0d8d7d4c6b1155fb693"
	PRVID   = "0000000000000000000000000000000000000000000000000000000000000004"
	PUSDTID = "716fd1009e2a1669caacc36891e707bfdf02590f96ebd897548e8963c95ebac0"
	PUSDCID = "1ff2da446abfebea3ba30385e2ca99b0f0bbeda5c6371f4c23c939672b429a42"
	PDAIID  = "3f89c75324b46f13c7b036871060e641d996a24c09b3065835cb1d38b799d6c1"
	ETHID   = "0000000000000000000000000000000000000000"
)

var ListToken = []PToken{
	{
		name:     "PUSDT",
		tokenID:  PUSDTID,
		decimals: 6,
	},
	{
		name:     "PUSDC",
		tokenID:  PUSDCID,
		decimals: 6,
	},
	{
		name:     "PUSDT",
		tokenID:  PDAIID,
		decimals: 9,
	},
}
