package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"time"
)

/*

This script gets all the blocks in which state sync txns are present on Mumbai (matic pos testnet) chain from polygonscan. It then checks if there are any difference between the no of txns in these blocks on local node vs remote rpc url and reports such occurrences.

Mumbai API for getting all state sync txs:
https://api-testnet.polygonscan.com/api?module=account&action=txlist&address=0x0000000000000000000000000000000000000000&startblock=1&endblock=99999999&page=1&offset=1000&sort=asc&apikey=YourApiKeyToken


https://api-testnet.polygonscan.com/api?module=account&action=txlist&address=0x0000000000000000000000000000000000000000&startblock=1&endblock=1000&page=1&offset=1000&sort=asc



1. Get all state sync txs in a block range from polygonscan
2. For these txs, check if we have these txs in our localhost bor rpc
3. If no, append output to a file

*/

type PolygonScanResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Result  []struct {
		BlockNumber       string `json:"blockNumber"`
		TimeStamp         string `json:"timeStamp"`
		Hash              string `json:"hash"`
		Nonce             string `json:"nonce"`
		BlockHash         string `json:"blockHash"`
		TransactionIndex  string `json:"transactionIndex"`
		From              string `json:"from"`
		To                string `json:"to"`
		Value             string `json:"value"`
		Gas               string `json:"gas"`
		GasPrice          string `json:"gasPrice"`
		IsError           string `json:"isError"`
		TxreceiptStatus   string `json:"txreceipt_status"`
		Input             string `json:"input"`
		ContractAddress   string `json:"contractAddress"`
		CumulativeGasUsed string `json:"cumulativeGasUsed"`
		GasUsed           string `json:"gasUsed"`
		Confirmations     string `json:"confirmations"`
	} `json:"result"`
}

type Tx struct {
	BlockNumber string
	Hash        string
}

type TxResponse struct {
	Jsonrpc string            `json:"jsonrpc"`
	ID      int               `json:"id"`
	Result  *TxResponseResult `json:"result"`
}

type TxResponseResult struct {
	BlockHash        string `json:"blockHash"`
	BlockNumber      string `json:"blockNumber"`
	From             string `json:"from"`
	Gas              string `json:"gas"`
	GasPrice         string `json:"gasPrice"`
	Hash             string `json:"hash"`
	Input            string `json:"input"`
	Nonce            string `json:"nonce"`
	To               string `json:"to"`
	TransactionIndex string `json:"transactionIndex"`
	Value            string `json:"value"`
	Type             string `json:"type"`
	V                string `json:"v"`
	R                string `json:"r"`
	S                string `json:"s"`
}

var psCount int
var missingTxs int

// Put Your Api Key Token here generated from polygonscan
// for example: var YourApiKeyToken = "xyz"
var YourApiKeyToken = ""

func getStateSyncTxns(start, end int) []Tx {
	var txs []Tx
	psMumbaiApiUrl := "https://api.polygonscan.com/api?module=account&action=txlist&address=0x0000000000000000000000000000000000000000&startblock=" + strconv.Itoa(start) + "&endblock=" + strconv.Itoa(end) + "&sort=asc&apikey=" + YourApiKeyToken
	fmt.Println("Fetching data from ", psMumbaiApiUrl)

	resp, err := http.Get(psMumbaiApiUrl)
	if err != nil {
		fmt.Println("PS: No response from request")
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body) // response body is []byte

	var result PolygonScanResponse
	if err := json.Unmarshal(body, &result); err != nil { // Parse []byte to the go struct pointer
		fmt.Println("PS: Can not unmarshal JSON:")
		fmt.Println(body)
		fmt.Print("\n")
	}

	// fmt.Println(PrettyPrint(result))

	fmt.Println("Got records: ", len(result.Result)/2)
	for i, rec := range result.Result {

		if i%2 == 0 && result.Result[i].From == "0x0000000000000000000000000000000000000000" && result.Result[i].To == "0x0000000000000000000000000000000000000000" {
			txs = append(txs, Tx{BlockNumber: rec.BlockNumber, Hash: rec.Hash})
			psCount += 1
		}
	}
	return txs
}

func PrettyPrint(i interface{}) string {
	s, _ := json.MarshalIndent(i, "", "\t")
	return string(s)
}

func main() {
	var txs []Tx
	currTime := time.Now().Format("2006-01-02T15:04:05")
	path := "missing_ss_txs" + currTime + ".json"
	var file, err = os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0755)

	path2 := "found_ss_txs" + currTime + ".json"
	var file2, _ = os.OpenFile(path2, os.O_RDWR|os.O_CREATE, 0755)
	if err != nil {
		return
	}

	maxBlockNo := 5000000
	currBlockNo := 0
	count := 0
	for currBlockNo < maxBlockNo {
		nextBlockNo := currBlockNo + 25000
		txs = getStateSyncTxns(currBlockNo, nextBlockNo)
		for _, tx := range txs {
			marshalled, _ := json.Marshal(tx)
			file2.WriteString(string(marshalled))
		}
		checkTxs(txs, file)
		currBlockNo = nextBlockNo
		if count%5 == 0 {
			time.Sleep(1 * time.Second)
		}
		count += 1
	}
	fmt.Println("Total no of records from PS: ", psCount)
	fmt.Println("Total no of missing txs in Bor: ", missingTxs)
	fmt.Println()
	defer file.Close()
	defer file2.Close()

}

func checkTxs(txs []Tx, file *os.File) {

	// curl localhost:8545 -X POST -H "Content-Type: application/json" -d '{"jsonrpc":"2.0","method":"eth_getTransactionByHash","params":["0xd96ecec3ac99e7e0f1edc62cff7d349c8c51cbfd0efc72f00662ecee6d41b14a"],"id":0}'

	url := "http://localhost:8545"

	for _, tx := range txs {
		var jsonStr = []byte(`{"jsonrpc":"2.0","method":"eth_getTransactionByHash","params":["` + tx.Hash + `"],"id":0}`)
		req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonStr))
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			panic(err)
		}
		defer resp.Body.Close()

		// fmt.Println("response Status:", resp.Status)
		// fmt.Println("response Headers:", resp.Header)
		body, _ := ioutil.ReadAll(resp.Body)

		var result TxResponse
		if err := json.Unmarshal(body, &result); err != nil { // Parse []byte to the go struct pointer
			fmt.Println("Bor: Cannot unmarshal JSON")
			fmt.Println(string(body))
		}

		// fmt.Println("response Body:", string(body))

		jsonStr, err = json.Marshal(tx)
		if err != nil {
			fmt.Println("Bor: Error dumping json")
		}

		if result.Result == nil {
			file.WriteString(string(jsonStr) + "\n")
			missingTxs += 1
		}

	}
}
