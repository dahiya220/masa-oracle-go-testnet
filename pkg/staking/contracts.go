package staking

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

const (
	rpcURL = "https://sepolia.infura.io/v3/74533a2e74bc430188366f3aa5715ae1" // update to Sepolia - this should be added as an environment variable sometime
)

// MasaTokenAddress Addresses of the deployed contracts (replace with actual addresses)
var MasaTokenAddress = common.HexToAddress("0x26775cD6D7615c8570c8421819c228225543a844")
var OracleNodeStakingContractAddress = common.HexToAddress("0xd925bc5d3eCd899a3F7B8D762397D2DC75E1187b")

// Client StakingClient holds the necessary details to interact with the Ethereum contracts
type Client struct {
	EthClient  *ethclient.Client
	PrivateKey *ecdsa.PrivateKey
}

func getStakingContractABI(jsonPath string) (abi.ABI, error) {
	jsonFile, err := ioutil.ReadFile(jsonPath)
	if err != nil {
		return abi.ABI{}, fmt.Errorf("failed to read ABI: %v", err)
	}

	var contract struct {
		ABI json.RawMessage `json:"abi"`
	}
	err = json.Unmarshal(jsonFile, &contract)
	if err != nil {
		return abi.ABI{}, fmt.Errorf("failed to unmarshal contract JSON: %v", err)
	}

	parsedABI, err := abi.JSON(strings.NewReader(string(contract.ABI)))
	if err != nil {
		return abi.ABI{}, fmt.Errorf("failed to parse ABI: %v", err)
	}

	return parsedABI, nil
}

// NewClient creates a new StakingClient using the Sepolia RPC endpoint
func NewClient(privateKey *ecdsa.PrivateKey) (*Client, error) {
	client, err := ethclient.Dial(rpcURL) // Use the Sepolia RPC URL
	if err != nil {
		return nil, err
	}
	return &Client{
		EthClient:  client,
		PrivateKey: privateKey,
	}, nil
}

// Approve allows the staking contract to spend tokens on behalf of the user
func (sc *Client) Approve(amount *big.Int) (string, error) {

	// Parse the ABI
	parsedABI, err := getStakingContractABI("contracts/build/contracts/MasaToken.json")
	if err != nil {
		return "", err
	}

	// Retrieve the sender's address from the private key
	fromAddress := crypto.PubkeyToAddress(sc.PrivateKey.PublicKey)

	// Get the nonce for the sender's address
	nonce, err := sc.EthClient.PendingNonceAt(context.Background(), fromAddress)
	if err != nil {
		return "", fmt.Errorf("failed to get nonce: %v", err)
	}

	// Define the value to send with the transaction, which is 0 for a token approve
	value := big.NewInt(0)

	// Pack the data to send with the transaction
	data, err := parsedABI.Pack("approve", OracleNodeStakingContractAddress, amount)
	if err != nil {
		return "", fmt.Errorf("failed to pack data for approve: %v", err)
	}

	// Estimate gas limit and gas price dynamically based on the current network conditions
	gasPrice, err := sc.EthClient.SuggestGasPrice(context.Background())
	if err != nil {
		return "", fmt.Errorf("failed to suggest gas price: %v", err)
	}

	// Estimate the gas limit for the approve function call
	msg := ethereum.CallMsg{
		From: fromAddress,
		To:   &MasaTokenAddress,
		Data: data,
	}
	gasLimit, err := sc.EthClient.EstimateGas(context.Background(), msg)
	if err != nil {
		return "", fmt.Errorf("failed to estimate gas: %v", err)
	}

	// Create the transaction
	tx := types.NewTransaction(nonce, MasaTokenAddress, value, gasLimit, gasPrice, data)

	// Sign the transaction
	chainID, err := sc.EthClient.NetworkID(context.Background())
	if err != nil {
		return "", fmt.Errorf("failed to get network ID: %v", err)
	}
	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), sc.PrivateKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign transaction: %v", err)
	}

	// Send the transaction
	err = sc.EthClient.SendTransaction(context.Background(), signedTx)
	if err != nil {
		return "", fmt.Errorf("failed to send transaction: %v", err)
	}

	// Wait for the transaction to be confirmed
	receipt, err := bind.WaitMined(context.Background(), sc.EthClient, signedTx)
	if err != nil {
		return "", fmt.Errorf("failed to get transaction receipt: %v", err)
	}
	if receipt.Status != 1 {
		return "", fmt.Errorf("transaction failed: %v", receipt)
	}

	// Return the transaction hash in hexadecimal format
	return signedTx.Hash().Hex(), nil
}

// Stake allows the user to stake tokens
func (sc *Client) Stake(amount *big.Int) (string, error) {
	// Fetch the chain ID dynamically
	chainID, err := sc.EthClient.NetworkID(context.Background())
	if err != nil {
		return "", fmt.Errorf("failed to get network ID: %v", err)
	}

	// Create an authenticated session
	auth, err := bind.NewKeyedTransactorWithChainID(sc.PrivateKey, chainID)
	if err != nil {
		return "", fmt.Errorf("failed to create keyed transactor: %v", err)
	}

	// Parse the ABI
	parsedABI, err := getStakingContractABI("contracts/build/contracts/OracleNodeStakingContract.json")
	if err != nil {
		return "", err
	}

	// Create an instance of the OracleNodeStakingContract using the parsed ABI and the contract address
	stakingContract := bind.NewBoundContract(OracleNodeStakingContractAddress, parsedABI, sc.EthClient, sc.EthClient, sc.EthClient)
	if err != nil {
		return "", fmt.Errorf("failed to bind staking contract instance: %v", err)
	}

	// Call the stake function of the OracleNodeStakingContract
	tx, err := stakingContract.Transact(auth, "stake", amount)
	if err != nil {
		return "", fmt.Errorf("failed to send stake transaction: %v", err)
	}

	// Wait for the transaction to be confirmed
	receipt, err := bind.WaitMined(context.Background(), sc.EthClient, tx)
	if err != nil {
		return "", fmt.Errorf("failed to get transaction receipt: %v", err)
	}
	if receipt.Status != 1 {
		return "", fmt.Errorf("transaction failed: %v", receipt)
	}

	// Return the transaction hash in hexadecimal format
	return tx.Hash().Hex(), nil
}
