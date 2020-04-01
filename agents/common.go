package agents
//
//import (
//	"encoding/json"
//	"errors"
//	"fmt"
//	"github.com/incognitochain/incognito-chain/common"
//	"github.com/incognitochain/incognito-chain/common/base58"
//	"github.com/incognitochain/incognito-chain/incognitokey"
//	"github.com/incognitochain/incognito-chain/metadata"
//	"github.com/incognitochain/incognito-chain/privacy"
//	zkp "github.com/incognitochain/incognito-chain/privacy/zeroknowledge"
//	"github.com/incognitochain/incognito-chain/rpcserver/jsonresult"
//	"github.com/incognitochain/incognito-chain/transaction"
//	"github.com/incognitochain/incognito-chain/wallet"
//	"math/big"
//	"portalfeeders/entities"
//	"portalfeeders/utils"
//	"sort"
//	"strconv"
//	"time"
//)
//
//// Init - init value for tx from inputcoin(old output coin from old tx)
//// create new outputcoin and build privacy proof
//// if not want to create a privacy tx proof, set hashPrivacy = false
//// database is used like an interface which use to query info from transactionStateDB in building tx
//func CreateIncognitoNormalTx(
//	rpcClient *utils.HttpClient,
//	keyWallet *wallet.KeyWallet,
//	paymentInfo []*privacy.PaymentInfo,
//	fee uint64,
//	isPrivacy bool,
//	metaData metadata.Metadata,
//	info []byte,
//	txVersion int8) (*transaction.Tx, error) {
//	var err error
//
//	// get public key last byte of sender
//	senderFullKey := keyWallet.KeySet
//	pkLastByteSender := senderFullKey.PaymentAddress.Pk[len(senderFullKey.PaymentAddress.Pk)-1]
//	senderPrivateKey := senderFullKey.PrivateKey
//
//	// get input coins to spent
//	inputCoins, _, err := GetInputCoinsToCreateNormalTx(rpcClient, &senderPrivateKey, paymentInfo, fee)
//	if err != nil {
//		return nil, err
//	}
//
//	fmt.Printf("Payment info after GetInputCoinsToCreateNormalTx: %v\n", paymentInfo)
//
//	// check valid of input coins, payment infos
//	if len(inputCoins) > 255 {
//		return nil, errors.New("number of input coins is exceed 255")
//	}
//	if len(paymentInfo) > 254 {
//		return nil, errors.New("number of output coins is exceed 255")
//	}
//	limitFee := uint64(0)
//	estimateTxSizeParam := transaction.NewEstimateTxSizeParam(len(inputCoins), len(paymentInfo),
//		isPrivacy, nil, nil, limitFee)
//	if txSize := transaction.EstimateTxSize(estimateTxSizeParam); txSize > common.MaxTxSize {
//		return nil, fmt.Errorf("max tx size is %v, but got %v", common.MaxTxSize, txSize)
//	}
//
//	// set tokenID is PRVID
//	tokenID := &common.Hash{}
//	err = tokenID.SetBytes(common.PRVCoinID[:])
//	if err != nil {
//		return nil, errors.New("TokenID is invalid")
//	}
//
//	// init tx
//	tx := new(transaction.Tx)
//	tx.Version = txVersion
//
//	if tx.LockTime == 0 {
//		tx.LockTime = time.Now().Unix()
//	}
//
//	// init info of tx
//	tx.Info = []byte{}
//	lenTxInfo := len(info)
//	if lenTxInfo > 0 {
//		if lenTxInfo > transaction.MaxSizeInfo {
//			return nil, errors.New("Length of info is exceed max size info")
//		}
//
//		tx.Info = info
//	}
//
//	// set metadata
//	tx.Metadata = metaData
//
//	// set tx type
//	tx.Type = common.TxNormalType
//
//	shardID := common.GetShardIDFromLastByte(pkLastByteSender)
//	var commitmentIndexs []uint64   // array index random of commitments in transactionStateDB
//	var myCommitmentIndexs []uint64 // index in array index random of commitment in transactionStateDB
//
//	if isPrivacy {
//		if len(inputCoins) == 0 {
//			return nil, errors.New("Input coins is empty")
//		}
//		commitmentIndexs, myCommitmentIndexs, _ = RandomCommitmentsProcess(rpcClient, inputCoins, shardID, tokenID)
//
//		// Check number of list of random commitments, list of random commitment indices
//		if len(commitmentIndexs) != len(inputCoins)*privacy.CommitmentRingSize {
//			return nil, errors.New("Random commitment error")
//		}
//
//		if len(myCommitmentIndexs) != len(inputCoins) {
//			return nil, errors.New("number of list my commitment indices must be equal to number of input coins")
//		}
//	}
//
//	// Calculate sum of all output coins' value
//	sumOutputValue := uint64(0)
//	for _, p := range paymentInfo {
//		sumOutputValue += p.Amount
//	}
//
//	// Calculate sum of all input coins' value
//	sumInputValue := uint64(0)
//	for _, coin := range inputCoins {
//		sumInputValue += coin.CoinDetails.GetValue()
//	}
//
//	// Calculate over balance, it will be returned to sender
//	overBalance := int64(sumInputValue - sumOutputValue - fee)
//
//	// Check if sum of input coins' value is at least sum of output coins' value and tx fee
//	if overBalance < 0 {
//		return nil, errors.New(fmt.Sprintf("input value less than output value. sumInputValue=%d sumOutputValue=%d fee=%d", sumInputValue, sumOutputValue, fee))
//	}
//
//	// if overBalance > 0, create a new payment info with pk is sender's pk and amount is overBalance
//	if overBalance > 0 {
//		changePaymentInfo := new(privacy.PaymentInfo)
//		changePaymentInfo.Amount = uint64(overBalance)
//		changePaymentInfo.PaymentAddress = senderFullKey.PaymentAddress
//		paymentInfo = append(paymentInfo, changePaymentInfo)
//	}
//
//	// create new output coins
//	outputCoins := make([]*privacy.OutputCoin, len(paymentInfo))
//
//	// create SNDs for output coins
//	ok := true
//	sndOuts := make([]*privacy.Scalar, 0)
//
//	for ok {
//		for i := 0; i < len(paymentInfo); i++ {
//			sndOut := privacy.RandomScalar()
//			keyWallet := new(wallet.KeyWallet)
//			keyWallet.KeySet.PaymentAddress = paymentInfo[i].PaymentAddress
//			paymentAddrStr := keyWallet.Base58CheckSerialize(wallet.PaymentAddressType)
//			for {
//				ok1, err := CheckSNDerivatorExistence(rpcClient, paymentAddrStr, []*privacy.Scalar{sndOut})
//				if err != nil || ok1[0] {
//					sndOut = privacy.RandomScalar()
//				} else {
//					break
//				}
//			}
//			sndOuts = append(sndOuts, sndOut)
//		}
//
//		// if sndOuts has two elements that have same value, then re-generates it
//		ok = privacy.CheckDuplicateScalarArray(sndOuts)
//		if ok {
//			sndOuts = make([]*privacy.Scalar, 0)
//		}
//	}
//
//	// create new output coins with info: Pk, value, last byte of pk, snd
//	for i, pInfo := range paymentInfo {
//		outputCoins[i] = new(privacy.OutputCoin)
//		outputCoins[i].CoinDetails = new(privacy.Coin)
//		outputCoins[i].CoinDetails.SetValue(pInfo.Amount)
//		if len(pInfo.Message) > 0 {
//			if len(pInfo.Message) > privacy.MaxSizeInfoCoin {
//				return nil, errors.New(fmt.Sprintf("message size %v is exceed MaxSizeInfoCoin %+v", pInfo.PaymentAddress, privacy.MaxSizeInfoCoin))
//			}
//		}
//		outputCoins[i].CoinDetails.SetInfo(pInfo.Message)
//
//		PK, err := new(privacy.Point).FromBytesS(pInfo.PaymentAddress.Pk)
//		if err != nil {
//			return nil, errors.New(fmt.Sprintf("can not decompress public key from %+v", pInfo.PaymentAddress))
//		}
//		outputCoins[i].CoinDetails.SetPublicKey(PK)
//		outputCoins[i].CoinDetails.SetSNDerivator(sndOuts[i])
//	}
//
//	// assign fee tx
//	tx.Fee = fee
//
//	// create zero knowledge proof of payment
//	tx.Proof = &zkp.PaymentProof{}
//
//	// get list of commitments for proving one-out-of-many from commitmentIndexs
//	commitmentProving := make([]*privacy.Point, len(commitmentIndexs))
//	for i, cmIndex := range commitmentIndexs {
//		temp, err := GetCommitmentByIndex(rpcClient, tokenID, cmIndex, shardID)
//		if err != nil {
//			return nil, fmt.Errorf("can not get commitment from index=%d shardID=%+v", cmIndex, shardID)
//		}
//		commitmentProving[i] = new(privacy.Point)
//		commitmentProving[i], err = commitmentProving[i].FromBytesS(temp)
//		if err != nil {
//			return nil, errors.New(fmt.Sprintf("can not get commitment from index=%d shardID=%+v value=%+v", cmIndex, shardID, temp))
//		}
//	}
//
//	// prepare witness for proving
//	witness := new(zkp.PaymentWitness)
//	paymentWitnessParam := zkp.PaymentWitnessParam{
//		HasPrivacy:              isPrivacy,
//		PrivateKey:              new(privacy.Scalar).FromBytesS(senderPrivateKey),
//		InputCoins:              inputCoins,
//		OutputCoins:             outputCoins,
//		PublicKeyLastByteSender: pkLastByteSender,
//		Commitments:             commitmentProving,
//		CommitmentIndices:       commitmentIndexs,
//		MyCommitmentIndices:     myCommitmentIndexs,
//		Fee:                     fee,
//	}
//	err = witness.Init(paymentWitnessParam)
//	if err.(*privacy.PrivacyError) != nil {
//		jsonParam, _ := json.MarshalIndent(paymentWitnessParam, common.EmptyString, "  ")
//		return nil, errors.New(string(jsonParam))
//	}
//
//	tx.Proof, err = witness.Prove(isPrivacy)
//	if err.(*privacy.PrivacyError) != nil {
//		jsonParam, _ := json.MarshalIndent(paymentWitnessParam, common.EmptyString, "  ")
//		return nil, errors.New(string(jsonParam))
//	}
//
//
//	// set private key for signing tx
//	sigPrivKey := []byte{}
//	if isPrivacy {
//		randSK := witness.GetRandSecretKey()
//		sigPrivKey = append(senderPrivateKey, randSK.ToBytesS()...)
//
//		// encrypt coin details (Randomness)
//		// hide information of output coins except coin commitments, public key, snDerivators
//		for i := 0; i < len(tx.Proof.GetOutputCoins()); i++ {
//			err = tx.Proof.GetOutputCoins()[i].Encrypt(paymentInfo[i].PaymentAddress.Tk)
//			if err.(*privacy.PrivacyError) != nil {
//				return nil, err
//			}
//			tx.Proof.GetOutputCoins()[i].CoinDetails.SetSerialNumber(nil)
//			tx.Proof.GetOutputCoins()[i].CoinDetails.SetValue(0)
//			tx.Proof.GetOutputCoins()[i].CoinDetails.SetRandomness(nil)
//		}
//
//		// hide information of input coins except serial number of input coins
//		for i := 0; i < len(tx.Proof.GetInputCoins()); i++ {
//			tx.Proof.GetInputCoins()[i].CoinDetails.SetCoinCommitment(nil)
//			tx.Proof.GetInputCoins()[i].CoinDetails.SetValue(0)
//			tx.Proof.GetInputCoins()[i].CoinDetails.SetSNDerivator(nil)
//			tx.Proof.GetInputCoins()[i].CoinDetails.SetPublicKey(nil)
//			tx.Proof.GetInputCoins()[i].CoinDetails.SetRandomness(nil)
//		}
//
//	} else {
//		randSK := big.NewInt(0)
//		sigPrivKey = append(senderPrivateKey, randSK.Bytes()...)
//	}
//
//	// sign tx
//	tx.PubKeyLastByteSender = pkLastByteSender
//	err = SignIncognitoNormalTx(tx, sigPrivKey)
//	if err != nil {
//		return nil, err
//	}
//
//	return tx, nil
//}
//
//// signTx - signs tx
//func SignIncognitoNormalTx(tx *transaction.Tx, sigPrivKey []byte) error {
//	//Check input transaction
//	if tx.Sig != nil {
//		return errors.New("input transaction must be an unsigned one")
//	}
//
//	/****** using Schnorr signature *******/
//	// sign with sigPrivKey
//	// prepare private key for Schnorr
//	sk := new(privacy.Scalar).FromBytesS(sigPrivKey[:common.BigIntSize])
//	r := new(privacy.Scalar).FromBytesS(sigPrivKey[common.BigIntSize:])
//	sigKey := new(privacy.SchnorrPrivateKey)
//	sigKey.Set(sk, r)
//
//	// save public key for verification signature tx
//	tx.SigPubKey = sigKey.GetPublicKey().GetPublicKey().ToBytesS()
//
//	signature, err := sigKey.Sign(tx.Hash()[:])
//	if err != nil {
//		return err
//	}
//
//	// convert signature to byte array
//	tx.Sig = signature.Bytes()
//
//	return nil
//}
//
//func RandomCommitmentsProcess (rpcClient *utils.HttpClient, inputCoins []*privacy.InputCoin, shardID byte, tokenID *common.Hash) ([]uint64, []uint64, error) {
//	// call RPC to random commitments
//	return []uint64{}, []uint64{}, nil
//
//}
//
//func CheckSNDerivatorExistence(rpcClient *utils.HttpClient, paymentAddressStr string, sndOut []*privacy.Scalar) ([]bool, error) {
//	var relayingBlockRes entities.RelayingBlockRes
//	sndStrs := make([]interface{}, len(sndOut))
//	for i, sn := range sndOut {
//		sndStrs[i] = base58.Base58Check{}.Encode(sn.ToBytesS(), common.Base58Version)
//	}
//	params := []interface{}{
//		paymentAddressStr,
//		sndStrs,
//	}
//	err := rpcClient.RPCCall("hassnderivators", params, &relayingBlockRes)
//	if err != nil {
//		return nil, err
//	}
//	if relayingBlockRes.RPCError != nil {
//		return nil, errors.New(relayingBlockRes.RPCError.Message)
//	}
//
//	res, ok := relayingBlockRes.Result.([]bool)
//	if !ok {
//		return nil, errors.New("Invalid response")
//	}
//
//	return res, nil
//}
//
//func GetCommitmentByIndex (rpcClient *utils.HttpClient, tokenID *common.Hash, cmIndex uint64, shardID byte) ([]byte, error) {
//	return []byte{}, nil
//}
//
//func NewOutputCoinsFromResponse(outCoins []jsonresult.OutCoin) ([]*privacy.OutputCoin, error) {
//	outputCoins := make([]*privacy.OutputCoin, len(outCoins))
//	for i, outCoin := range outCoins {
//		outputCoins[i] = new(privacy.OutputCoin).Init()
//		publicKey, _, _ := base58.Base58Check{}.Decode(outCoin.PublicKey)
//		publicKeyPoint, _ := new(privacy.Point).FromBytesS(publicKey)
//		outputCoins[i].CoinDetails.SetPublicKey(publicKeyPoint)
//
//		cmBytes, _, _ := base58.Base58Check{}.Decode(outCoin.CoinCommitment)
//		cmPoint, _ := new(privacy.Point).FromBytesS(cmBytes)
//		outputCoins[i].CoinDetails.SetCoinCommitment(cmPoint)
//
//		sndBytes, _, _ := base58.Base58Check{}.Decode(outCoin.SNDerivator)
//		sndScalar := new(privacy.Scalar).FromBytesS(sndBytes)
//		outputCoins[i].CoinDetails.SetSNDerivator(sndScalar)
//
//		randomnessBytes, _, _ := base58.Base58Check{}.Decode(outCoin.Randomness)
//		randomnessScalar := new(privacy.Scalar).FromBytesS(randomnessBytes)
//		outputCoins[i].CoinDetails.SetRandomness(randomnessScalar)
//
//		value, _ := strconv.Atoi(outCoin.Value)
//		outputCoins[i].CoinDetails.SetValue(uint64(value))
//	}
//
//	return outputCoins, nil
//}
//
//// GetListOutputCoins calls Incognito RPC to get all output coins of the account
//func GetListOutputCoins(rpcClient *utils.HttpClient, paymentAddress string, viewingKey string) ([]*privacy.OutputCoin, error) {
//	var relayingBlockRes entities.RelayingBlockRes
//	params := []interface{}{
//		0,
//		999999,
//		[]map[string]string{
//			 {
//				"PaymentAddress": paymentAddress,
//				"ReadonlyKey": viewingKey,
//			 },
//		},
//	}
//	err := rpcClient.RPCCall("listoutputcoins", params, &relayingBlockRes)
//	if err != nil {
//		return nil, err
//	}
//	if relayingBlockRes.RPCError != nil {
//		return nil, errors.New(relayingBlockRes.RPCError.Message)
//	}
//
//	res, ok := relayingBlockRes.Result.(jsonresult.ListOutputCoins)
//	if !ok {
//		return nil, errors.New("Invalid response")
//	}
//	outputCoins, err := NewOutputCoinsFromResponse(res.Outputs[viewingKey])
//	if err != nil {
//		return nil, err
//	}
//	return outputCoins, nil
//}
//
//// CheckExistenceSerialNumber calls Incognito RPC to check existence serial number on network
//// to check output coins is spent or unspent
//func CheckExistenceSerialNumber(rpcClient *utils.HttpClient, paymentAddressStr string, sns []*privacy.Point) ([]bool, error){
//	var relayingBlockRes entities.RelayingBlockRes
//	snStrs := make([]interface{}, len(sns))
//	for i, sn := range sns {
//		snStrs[i] = base58.Base58Check{}.Encode(sn.ToBytesS(), common.Base58Version)
//	}
//	params := []interface{}{
//		paymentAddressStr,
//		snStrs,
//	}
//	err := rpcClient.RPCCall("hasserialnumbers", params, &relayingBlockRes)
//	if err != nil {
//		return nil, err
//	}
//	if relayingBlockRes.RPCError != nil {
//		return nil, errors.New(relayingBlockRes.RPCError.Message)
//	}
//
//	res, ok := relayingBlockRes.Result.([]bool)
//	if !ok {
//		return nil, errors.New("Invalid response")
//	}
//
//	return res, nil
//}
//
//func DeriveSerialNumbers(privateKey *privacy.PrivateKey, outputCoins []*privacy.OutputCoin) ([]*privacy.Point, error){
//	serialNumbers := make([]*privacy.Point, len(outputCoins))
//	for i, coin := range outputCoins {
//		coin.CoinDetails.SetSerialNumber(
//			new(privacy.Point).Derive(
//				privacy.PedCom.G[privacy.PedersenPrivateKeyIndex],
//				new(privacy.Scalar).FromBytesS(*privateKey),
//				coin.CoinDetails.GetSNDerivator()))
//		serialNumbers[i] = coin.CoinDetails.GetSerialNumber()
//	}
//
//	return serialNumbers, nil
//}
//
//// GetUnspentOutputCoins return utxos of an account
//func GetUnspentOutputCoins(rpcClient *utils.HttpClient, keyWallet *wallet.KeyWallet) ([]*privacy.OutputCoin, error) {
//	privateKey := &keyWallet.KeySet.PrivateKey
//	paymentAddressStr := keyWallet.Base58CheckSerialize(wallet.PaymentAddressType)
//	viewingKeyStr := keyWallet.Base58CheckSerialize(wallet.ReadonlyKeyType)
//
//	outputCoins, err := GetListOutputCoins(rpcClient, paymentAddressStr, viewingKeyStr)
//	if err != nil {
//		return nil, err
//	}
//
//	serialNumbers, err := DeriveSerialNumbers(privateKey, outputCoins)
//	if err != nil {
//		return nil, err
//	}
//
//	isExisted, err := CheckExistenceSerialNumber(rpcClient, paymentAddressStr, serialNumbers)
//	if err != nil {
//		return nil, err
//	}
//
//	utxos := make([]*privacy.OutputCoin, 0)
//	for i, out := range outputCoins {
//		if !isExisted[i] {
//			utxos = append(utxos, out)
//		}
//	}
//
//	return utxos, nil
//}
//
//// ChooseBestOutCoinsToSpent returns list of unspent coins for spending with amount
//func ChooseBestOutCoinsToSpent(utxos []*privacy.OutputCoin, amount uint64) (
//	resultOutputCoins []*privacy.OutputCoin,
//	remainOutputCoins []*privacy.OutputCoin,
//	totalResultOutputCoinAmount uint64, err error) {
//
//	resultOutputCoins = make([]*privacy.OutputCoin, 0)
//	remainOutputCoins = make([]*privacy.OutputCoin, 0)
//	totalResultOutputCoinAmount = uint64(0)
//
//	// either take the smallest coins, or a single largest one
//	var outCoinOverLimit *privacy.OutputCoin
//	outCoinsUnderLimit := make([]*privacy.OutputCoin, 0)
//	for _, outCoin := range utxos {
//		if outCoin.CoinDetails.GetValue() < amount {
//			outCoinsUnderLimit = append(outCoinsUnderLimit, outCoin)
//		} else if outCoinOverLimit == nil {
//			outCoinOverLimit = outCoin
//		} else if outCoinOverLimit.CoinDetails.GetValue() > outCoin.CoinDetails.GetValue() {
//			remainOutputCoins = append(remainOutputCoins, outCoin)
//		} else {
//			remainOutputCoins = append(remainOutputCoins, outCoinOverLimit)
//			outCoinOverLimit = outCoin
//		}
//	}
//	sort.Slice(outCoinsUnderLimit, func(i, j int) bool {
//		return outCoinsUnderLimit[i].CoinDetails.GetValue() < outCoinsUnderLimit[j].CoinDetails.GetValue()
//	})
//	for _, outCoin := range outCoinsUnderLimit {
//		if totalResultOutputCoinAmount < amount {
//			totalResultOutputCoinAmount += outCoin.CoinDetails.GetValue()
//			resultOutputCoins = append(resultOutputCoins, outCoin)
//		} else {
//			remainOutputCoins = append(remainOutputCoins, outCoin)
//		}
//	}
//	if outCoinOverLimit != nil && (outCoinOverLimit.CoinDetails.GetValue() > 2*amount || totalResultOutputCoinAmount < amount) {
//		remainOutputCoins = append(remainOutputCoins, resultOutputCoins...)
//		resultOutputCoins = []*privacy.OutputCoin{outCoinOverLimit}
//		totalResultOutputCoinAmount = outCoinOverLimit.CoinDetails.GetValue()
//	} else if outCoinOverLimit != nil {
//		remainOutputCoins = append(remainOutputCoins, outCoinOverLimit)
//	}
//	if totalResultOutputCoinAmount < amount {
//		return resultOutputCoins, remainOutputCoins, totalResultOutputCoinAmount, errors.New("Not enough coin")
//	} else {
//		return resultOutputCoins, remainOutputCoins, totalResultOutputCoinAmount, nil
//	}
//}
//
//func GetInputCoinsToCreateNormalTx(
//	rpcClient *utils.HttpClient,
//	senderPrivateKey *privacy.PrivateKey,
//	paymentInfos []*privacy.PaymentInfo,
//	fee uint64,
//)([]*privacy.InputCoin, uint64, error) {
//	// get unspent output coins (UTXOs)
//	keyWallet := new(wallet.KeyWallet)
//	err := keyWallet.KeySet.InitFromPrivateKey(senderPrivateKey)
//	if err != nil {
//		return nil, uint64(0), err
//	}
//
//	utxos, err := GetUnspentOutputCoins(rpcClient, keyWallet)
//	if err != nil {
//		return nil, uint64(0), err
//	}
//
//	// calculate total amount to send (include fee)
//	totalAmount := uint64(0)
//	for _, receiver := range paymentInfos {
//		totalAmount += receiver.Amount
//	}
//	totalAmount += fee
//	if len(utxos) == 0 && totalAmount > 0 {
//		return nil, uint64(0), errors.New("not enough utxos to spent")
//	}
//
//	// choose best UTXOs to spend
//	candidateOutputCoins, _, candidateOutputCoinAmount, err := ChooseBestOutCoinsToSpent(utxos, totalAmount)
//	if err != nil {
//		return nil, uint64(0), err
//	}
//
//	// refund out put for sender
//	overBalanceAmount := candidateOutputCoinAmount - totalAmount
//	if overBalanceAmount > 0 {
//		paymentInfos = append(paymentInfos, &privacy.PaymentInfo{
//			PaymentAddress: keyWallet.KeySet.PaymentAddress,
//			Amount:         overBalanceAmount,
//		})
//	}
//
//	// convert to inputcoins
//	inputCoins := transaction.ConvertOutputCoinToInputCoin(candidateOutputCoins)
//	return inputCoins, candidateOutputCoinAmount, nil
//}
//
//func SendIncognitoNormalTx(rpcClient *utils.HttpClient, tx *transaction.Tx) (string, error) {
//	txBytes, _ := json.Marshal(tx)
//	txStr := base58.Base58Check{}.Encode(txBytes, common.Base58Version)
//
//	var relayingBlockRes entities.RelayingBlockRes
//	params := []interface{}{
//		txStr,
//	}
//	err := rpcClient.RPCCall("sendtransaction", params, &relayingBlockRes)
//	if err != nil {
//		return "", err
//	}
//	if relayingBlockRes.RPCError != nil {
//		return "", errors.New(relayingBlockRes.RPCError.Message)
//	}
//
//	res, ok := relayingBlockRes.Result.(jsonresult.CreateTransactionResult)
//	if !ok {
//		return "", errors.New("Invalid response")
//	}
//
//	return res.TxID, nil
//}
//
//func CreateAndSendTxRelayBNBBlock(rpcClient *utils.HttpClient, privateKeyStr string, bnbHeaderStr string, bnbHeaderBlockHeight int64) (string, error) {
//	// deserialize private key
//	keyWallet, err := wallet.Base58CheckDeserialize(privateKeyStr)
//	if err != nil {
//		return "", err
//	}
//	senderPrivateKey := keyWallet.KeySet.PrivateKey
//	// create sender's key set from sender's private key
//	senderFullKey := incognitokey.KeySet{}
//	err = senderFullKey.InitFromPrivateKey(&senderPrivateKey)
//	if err != nil {
//		return "", errors.New("Sender private key is invalid")
//	}
//	paymentAddrStr := keyWallet.Base58CheckSerialize(wallet.PaymentAddressType)
//
//	metadata, _ := metadata.NewRelayingHeader(
//		metadata.RelayingBNBHeaderMeta, paymentAddrStr, bnbHeaderStr, uint64(bnbHeaderBlockHeight))
//
//	tx, err := CreateIncognitoNormalTx(
//		rpcClient, keyWallet, []*privacy.PaymentInfo{}, DefaultFee, false, metadata, nil, 1)
//	if err != nil {
//		return "", err
//	}
//
//	txID, err := SendIncognitoNormalTx(rpcClient, tx)
//	if err != nil {
//		return "", err
//	}
//
//	return txID, nil
//}
//
//func CreateAndSendNormalTx(rpcClient *utils.HttpClient,  privateKeyStr string, paymentInfo []*privacy.PaymentInfo) (string, error){
//	// deserialize private key
//	keyWallet, err := wallet.Base58CheckDeserialize(privateKeyStr)
//	if err != nil {
//		return "", err
//	}
//	senderPrivateKey := keyWallet.KeySet.PrivateKey
//	// create sender's key set from sender's private key
//	senderFullKey := incognitokey.KeySet{}
//	err = senderFullKey.InitFromPrivateKey(&senderPrivateKey)
//	if err != nil {
//		return "", errors.New("Sender private key is invalid")
//	}
//	//paymentAddrStr := keyWallet.Base58CheckSerialize(wallet.PaymentAddressType)
//
//	tx, err := CreateIncognitoNormalTx(
//		rpcClient, keyWallet, []*privacy.PaymentInfo{}, DefaultFee, false, nil, nil, 1)
//	if err != nil {
//		return "", err
//	}
//
//	txID, err := SendIncognitoNormalTx(rpcClient, tx)
//	if err != nil {
//		return "", err
//	}
//
//	return txID, nil
//}
//
