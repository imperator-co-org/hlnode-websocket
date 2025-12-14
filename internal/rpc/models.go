package rpc

// Log represents an Ethereum log entry
type Log struct {
	Address          string   `json:"address"`
	Topics           []string `json:"topics"`
	Data             string   `json:"data"`
	BlockNumber      string   `json:"blockNumber"`
	BlockHash        string   `json:"blockHash"`
	TransactionHash  string   `json:"transactionHash"`
	TransactionIndex string   `json:"transactionIndex"`
	LogIndex         string   `json:"logIndex"`
	Removed          bool     `json:"removed"`
	BlockTimestamp   string   `json:"blockTimestamp,omitempty"`
}

// FullBlockHeader represents a complete block header for newHeads subscription
type FullBlockHeader struct {
	Number                string `json:"number"`
	Hash                  string `json:"hash"`
	ParentHash            string `json:"parentHash"`
	Nonce                 string `json:"nonce,omitempty"`
	Sha3Uncles            string `json:"sha3Uncles"`
	LogsBloom             string `json:"logsBloom"`
	TransactionsRoot      string `json:"transactionsRoot"`
	StateRoot             string `json:"stateRoot"`
	ReceiptsRoot          string `json:"receiptsRoot"`
	Miner                 string `json:"miner"`
	Difficulty            string `json:"difficulty,omitempty"`
	TotalDifficulty       string `json:"totalDifficulty,omitempty"`
	ExtraData             string `json:"extraData"`
	Size                  string `json:"size,omitempty"`
	GasLimit              string `json:"gasLimit"`
	GasUsed               string `json:"gasUsed"`
	Timestamp             string `json:"timestamp"`
	BaseFeePerGas         string `json:"baseFeePerGas,omitempty"`
	MixHash               string `json:"mixHash,omitempty"`
	WithdrawalsRoot       string `json:"withdrawalsRoot,omitempty"`
	BlobGasUsed           string `json:"blobGasUsed,omitempty"`
	ExcessBlobGas         string `json:"excessBlobGas,omitempty"`
	ParentBeaconBlockRoot string `json:"parentBeaconBlockRoot,omitempty"`
}

// TransactionReceipt represents a transaction receipt
type TransactionReceipt struct {
	BlockHash         string `json:"blockHash"`
	BlockNumber       string `json:"blockNumber"`
	ContractAddress   string `json:"contractAddress,omitempty"`
	CumulativeGasUsed string `json:"cumulativeGasUsed"`
	EffectiveGasPrice string `json:"effectiveGasPrice"`
	From              string `json:"from"`
	GasUsed           string `json:"gasUsed"`
	Logs              []Log  `json:"logs"`
	LogsBloom         string `json:"logsBloom"`
	Status            string `json:"status"`
	To                string `json:"to,omitempty"`
	TransactionHash   string `json:"transactionHash"`
	TransactionIndex  string `json:"transactionIndex"`
	Type              string `json:"type"`
}

// BlockReceipts represents receipts for an entire block
type BlockReceipts struct {
	BlockNumber string               `json:"blockNumber"`
	BlockHash   string               `json:"blockHash"`
	Receipts    []TransactionReceipt `json:"receipts"`
}

// GasPriceInfo represents gas price information for subscription
type GasPriceInfo struct {
	GasPrice         string `json:"gasPrice"`
	BigBlockGasPrice string `json:"bigBlockGasPrice,omitempty"`
	BlockNumber      string `json:"blockNumber"`
}

// SyncStatus represents the syncing status (matches eth_syncing response)
// When syncing: returns object with progress info
// When not syncing: returns false (handled separately)
type SyncStatus struct {
	Syncing          bool   `json:"syncing"`
	StartingBlock    string `json:"startingBlock,omitempty"`
	CurrentBlock     string `json:"currentBlock,omitempty"`
	HighestBlock     string `json:"highestBlock,omitempty"`
	HealedBytecodes  string `json:"healedBytecodes,omitempty"`
	HealedTrienodes  string `json:"healedTrienodes,omitempty"`
	HealingBytecode  string `json:"healingBytecode,omitempty"`
	HealingTrienodes string `json:"healingTrienodes,omitempty"`
	SyncedAccounts   string `json:"syncedAccounts,omitempty"`
	SyncedBytecodes  string `json:"syncedBytecodes,omitempty"`
	SyncedStorage    string `json:"syncedStorage,omitempty"`
}
