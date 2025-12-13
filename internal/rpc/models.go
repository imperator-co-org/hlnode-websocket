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
