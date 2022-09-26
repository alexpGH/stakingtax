package taxcsv

import (
	"alexp/stakingtax/pkg/utils"
	"fmt"
	_ "log"
	"os"
	"strconv"
	"strings"

	"github.com/gocarina/gocsv"
)

type TaxCsv struct {
	Timestamp          string  `csv:"timestamp"`
	Blockheight        int     `csv:"blockheight"`
	MsgType            string  `csv:"msg_type"`
	ReceivedAmount     float64 `csv:"received_amount"`
	ReceivedCurrency   string  `csv:"received_currency"`
	FeeAmount          float64 `csv:"fee_amount"`
	FeeCurrency        string  `csv:"fee_currency"`
	ReceivedFiatAmount float64 `csv:"received_fiat"`
	FeeFiatAmount      float64 `csv:"fee_fiat"`
	CoinPrice          float64 `csv:"coin_price_that_day"`
	TxId               string  `csv:"tx_id"`
	Addr               string  `csv:"address"`
	Key                string  `csv:"pub_key"`
}

func GetLastBlockHeight(pathFile string) int {
	var blockHeight int
	var err error
	lastLine := ""
	lastLine = utils.GetLastLine(pathFile, true) //true=remove blank lines
	if lastLine != "" {
		//we slcie it ourselves, as we don't have a header for auto-unmarshalling
		lineSliced := strings.Split(lastLine, ",")
		blockHeight, err = strconv.Atoi(lineSliced[1])
		utils.ErrDefaultFatal(err)

	} else {
		blockHeight = 0
	}

	return blockHeight

}

func GetLastTxCount(pathFile string) int {
	var err error
	var txCount int

	lastLine := ""
	lastLine = utils.GetLastLine(pathFile, true) //true=remove blank lines
	if lastLine != "" {
		txCount, err = strconv.Atoi(lastLine)
		utils.ErrDefaultFatal(err)
	} else {
		txCount = 0
	}
	return txCount
}

func UpdateLastTxCount(pathFile string, txCount int) {
	f, err := os.OpenFile(pathFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	utils.ErrDefaultFatal(err)
	_, err = fmt.Fprintf(f, "%d", txCount)

}

func AppendNewTaxRows(pathFile string, allNewTaxCsvRows []*TaxCsv) {

	if len(allNewTaxCsvRows) > 0 {
		f, err := os.OpenFile(pathFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
		utils.ErrDefaultFatal(err)
		defer f.Close()

		err = gocsv.MarshalFile(&allNewTaxCsvRows, f)
		utils.ErrDefaultFatal(err)
	}
}

// // If the file doesn't exist, create it, or append to the file
// f, err := os.OpenFile("access.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
// if err != nil {
// 	log.Fatal(err)
// }

// in, err := os.Open("clients.csv")

// //--- open csv file
// file, err := os.Open(configPathFile)
// utils.ErrDefaultFatal(err) //on err log.Fatal with details
// defer file.Close()
