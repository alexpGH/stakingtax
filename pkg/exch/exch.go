//exch.go
package exch

import (
	"alexp/stakingtax/pkg/configData"
	"alexp/stakingtax/pkg/taxcsv"
	"alexp/stakingtax/pkg/utils"
	"encoding/json"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"strconv"
	"time"
)

type funcEndPointHandler func(time.Time, string, string) (float64, bool)

var endpointM = map[string]funcEndPointHandler{
	"cbpro":   GetFiatBaseFactCBPro,
	"binance": GetFiatBaseFactBinance,
}

func AddFiatBaseInfo2TaxCsvData(tradePairs4Tax *configData.TradePairs4TaxType, allNewTaxCsvRows []*taxcsv.TaxCsv, sLogSep string) {
	var amountBase float64
	var tOk bool
	var fact float64

	for _, row := range allNewTaxCsvRows {

		// get conversion fact (we can neglect tOk here, as it is handled internally)
		amountBase, fact, tOk = GetFiatBaseAmountForDay(tradePairs4Tax, row.Timestamp, row.ReceivedAmount, sLogSep)

		//if not ok we leave the value as initialized (0)
		if tOk {
			row.ReceivedFiatAmount = amountBase

			//no need to query again, we know the factor:
			row.FeeFiatAmount = row.FeeAmount * fact
		}

	}

}

//Gets the close price in teax base for a given trading pair and day/time string in RFC3339 format "2006-01-02T15:04:05Z07:00", "2006-01-02T15:04:05Z"
//via coinbase pro API.
//The tradePairs are executed in the given sequence, the final unit is regarded as base unit. E.g. [FET-BTC BTC-EUR]
func GetFiatBaseAmountForDay(tradePairs4Tax *configData.TradePairs4TaxType, sDate string, amount float64, sLogSep string) (float64, float64, bool) {
	tOk := false
	var baseAmount float64
	baseAmount = 0.0
	var fact, factOut float64

	//convert date
	layout := "2006-01-02T15:04:05Z07:00"
	t, err := time.Parse(layout, sDate)
	utils.ErrDefaultFatal(err)

	//get api snippets for this endpoint (excluding the pair)

	fact = 1.0
	for _, pair := range tradePairs4Tax.Pairs {

		//call the endpoint related function
		factOut, tOk = endpointM[tradePairs4Tax.EndPoint](t, pair, sLogSep+"   ")

		fact = fact * factOut //we have 0.0 returned in case retrieval failed (this was reported in the routine)

		//no need to continue if one conversion failed
		if !tOk {
			break
		}
	}

	baseAmount = amount * fact //will be 0.0 if retrieval of any conversion fact failed
	return baseAmount, fact, tOk

}

func GetFiatBaseFactCBPro(t time.Time, pair string, sLogSep string) (float64, bool) {
	var err error
	var url string
	var factOut float64
	var tSucc bool
	var closeTime0 float64
	var tResClose0 time.Time

	//https://go.dev/play/p/0MUY-yOYII how to unmarshal mixed json (with / without name: value)

	//type CBProData [][6]interface{}
	type CBProData [2][6]float64
	/* https://docs.cloud.coinbase.com/exchange/reference/exchangerestapi_getproductcandles
	   coinbase pro provides a vector of 6 float64 fields
	       time bucket start time
	       low lowest price during the bucket interval
	       high highest price during the bucket interval
	       open opening price (first trade) in the bucket interval
	       close closing price (last trade) in the bucket interval
	       volume volume of trading activity during the bucket interval
	*/

	layout2 := "2006-01-02"

	//CBPro requires us to query 2 days to get a result.
	// e.g. the 05-14 - 05-15 for the 05-15
	// the result is then the bucket start time at 05-15 00:00 and start time 05-14 00:00 (time sequence in reverse order)
	// switched to open, as close will not be defined when running on the current date
	// open: we use the first result time and price
	// close: -> we need to check the first buckets start time, as it is the second buckets close time,
	// and use the second buckets close

	//add a day for query end
	t1 := t.AddDate(0, 0, -1)

	//cbpro
	sApiBase := "https://api.pro.coinbase.com/products/"
	sApiTail := "/candles?start=" + t1.Format(layout2) + "T00:00:00Z" + "&end=" + t.Format(layout2) + "&granularity=86400"

	cbProData := new(CBProData)

	url = sApiBase + pair + sApiTail

	//get reponse from exchange
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("Accept", "application/json")
	res, err := http.DefaultClient.Do(req)
	utils.ErrDefaultFatal(err)
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	utils.ErrDefaultFatal(err)

	err = json.Unmarshal(body, &cbProData) //default unmarshal uses []int8, not the reader (Body)
	utils.ErrDefaultFatal(err)

	tSucc = true
	factOut = 0.0

	//sanity check format of response
	if len(cbProData) != 2 || len(cbProData[0]) != 6 {
		log.Printf(sLogSep+"[WARN] CB Pro data result does not fit expected structure of [2][6]. It is [%v][%v]. Skipping Fiat value for this data point! "+utils.FatalDetails(), len(cbProData), len(cbProData[0]))
		tSucc = false
	}

	//ass discussed above: [0] bucket's start time is the [1] buckets close time (which is what we are interested in)
	closeTime0 = cbProData[0][0]
	tResClose0 = time.Unix(int64(math.Round(closeTime0)), 0)

	//sanity check time diff or close time
	tDiff := math.Abs(t.Sub(tResClose0).Hours())

	if tDiff > 24 {
		log.Printf(sLogSep+"[WARN] CB Pro result should give us a closeTime for this day -> less than 24h away. In fact, time diff was %v. Trade time was %v and closeRes was %v . Skipping Fiat value for this data point! "+utils.FatalDetails(), tDiff, t, tResClose0)
		tSucc = false
	}

	if tSucc {
		//as discussed above: the [1] buckets close value is what we are interested in
		factOut = cbProData[1][3] //3 is now open price, close was 4
	}

	return factOut, tSucc

}

func GetFiatBaseFactBinance(t time.Time, pair string, sLogSep string) (float64, bool) {
	var err error
	var url string
	var factOut float64
	var tSucc bool
	var closeTime float64
	var tResClose time.Time

	//https://go.dev/play/p/0MUY-yOYII how to unmarshal mixed json (with / without name: value)

	type BinanceData [1][12]interface{}
	/* https://github.com/binance/binance-spot-api-docs/blob/master/rest-api.md#klinecandlestick-data
	   binance provides a vector of 12 mixed fields (partly string, partyl float64)
		1499040000000,      // Open time
	    "0.01634790",       // Open
	    "0.80000000",       // High
	    "0.01575800",       // Low
	    "0.01577100",       // Close
	    "148976.11427815",  // Volume
	    1499644799999,      // Close time
	    "2434.19055334",    // Quote asset volume
	    308,                // Number of trades
	    "1756.87402397",    // Taker buy base asset volume
	    "28.46694368",      // Taker buy quote asset volume
	    "17928899.62484339" // Ignore.
	*/

	//binance give us the open from the NEXT! day start -> we need to subtract a day for the query
	//we use the close at this day
	tU := t.AddDate(0, 0, -1)
	binanceData := new(BinanceData)

	sApiBase := "https://api.binance.com/api/v3/klines?symbol="
	sApiTail := "&interval=1d&startTime=" + strconv.FormatInt(tU.UnixMilli(), 10) + "&limit=1"

	url = sApiBase + pair + sApiTail

	//get reponse from exchange
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("Accept", "application/json")
	res, err := http.DefaultClient.Do(req)
	utils.ErrDefaultFatal(err)
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	utils.ErrDefaultFatal(err)

	err = json.Unmarshal(body, &binanceData) //default unmarshal uses []int8, not the reader (Body)
	utils.ErrDefaultFatal(err)

	tSucc = true
	factOut = 0.0

	//sanity check format of response
	if len(binanceData) != 1 || len(binanceData[0]) != 12 {
		log.Printf(sLogSep+"[WARN] Binance data result does not fit expected structure of [1][12]. It is [%v][%v]. Skipping Fiat value for this data point! "+utils.FatalDetails(), len(binanceData), len(binanceData[0]))
		tSucc = false
	}

	// we opted for the open price, as close is not defined if the code is running on the day of the reward
	closeTime = binanceData[0][0].(float64) //close was 6
	tResClose = time.UnixMilli(int64(math.Round(closeTime)))

	//sanity check time diff or close time
	tDiff := math.Abs(t.Sub(tResClose).Hours())

	if tDiff > 24 {
		log.Printf(sLogSep+"[WARN] Binance result should give us a closeTime for this day -> less than 24h away. In fact, time diff was %v. Trade time was %v and closeRes was %v . Skipping Fiat value for this data point! "+utils.FatalDetails(), tDiff, t, tResClose)
		tSucc = false
	}

	if tSucc {
		sClose := binanceData[0][1].(string) //close was [4]
		factOut, err = strconv.ParseFloat(sClose, 64)
		if err != nil {
			log.Printf(sLogSep+"[WARN] Can not convert binance close to float! Skipping Fiat value for this data point! Error was %w ."+utils.FatalDetails(), err)
			tSucc = false
		}

	}

	return factOut, tSucc

}
