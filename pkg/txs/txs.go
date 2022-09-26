package txs

import (
	"alexp/stakingtax/pkg/configData"
	"alexp/stakingtax/pkg/exch"
	nw "alexp/stakingtax/pkg/network"
	"alexp/stakingtax/pkg/taxcsv"
	"alexp/stakingtax/pkg/utils"

	"encoding/json"
	"fmt"
	"log"
	"math"
	_ "os"
	"os/exec"
	"strconv"

	"golang.org/x/exp/slices"
)

type TxsResp struct {
	TotalCount string `json:"total_count"`
	Count      string `json:"count"`
	PageNumber string `json:"page_number"`
	PageTotal  string `json:"page_total"`
	Txs        []struct {
		Height    string `json:"height"`
		TxHash    string `json:"txhash"`
		Timestamp string `json:"timestamp"`
		Logs      []struct {
			Events []struct {
				Type       string `json:"type"`
				Attributes []struct {
					Key   string `json:"key"`
					Value string `json:"value"`
				} `json:"attributes"`
			} `json:"events"`
		} `json:"logs"`
		Tx struct {
			AuthInfo struct {
				SignerInfos []struct {
					PublicKey struct {
						Key string `json:"key"`
					} `json:"public_key"`
				} `json:"signer_infos"`
				Fee struct {
					Amount []struct {
						Denom  string `json:"denom"`
						Amount string `json:"amount"`
					} `json:"amount"`
				} `json:"fee"`
			} `json:"auth_info"`
		} `json:"tx"`
	} `json:"txs"`
}

func GetProcessTxsForNetworks(cfg *configData.Cfg, cfgAdr *configData.CfgAdr, chainInfos []nw.ChainInfo) {
	pageLimit := cfg.Query.PageLimit
	var networkIdx int
	var addrs []string
	var pubKeys []string
	var blockHeightOld, blockHeight int
	var txCountOld, txCountUsed int
	var page int
	var ourPubKey string
	var pageTotal, totalCount int
	var err error
	var sLogSep string
	tradePairs4Tax := &configData.TradePairs4TaxType{}

	//get cfg's networks and relevant message types
	networks := cfg.GetNetworksFieldString("Name")

	log.Println("Querying networks for txs =======================================================================")

	for i, network := range cfgAdr.Addresses {
		//get the idx in the cfg
		networkIdx = slices.Index(networks, network.ChainName)
		if networkIdx == -1 {
			log.Println("[W] Skipping unknonw network:" + network.ChainName + " given in addr.yaml, but not present in config")
			continue //next network
		}

		if !chainInfos[networkIdx].TProcess {
			log.Println("[I] Skipping inactivated network: " + network.ChainName + " (in this run due to unresponsive node)")
			continue //next network
		}

		//here we have a valid network; extract given addr and pubkeys as slice (for simple Contains check later on)
		addrs = cfgAdr.GetFieldString(i, "Addr")
		pubKeys = cfgAdr.GetFieldString(i, "PubKey")

		log.Println("Querying " + network.ChainName + "--------------------------------------------------------")
		//we need to handle each address individually, as cosmos query does not provide || for event filter (only &&)
		//-> we can only get the txs per address individually
		for j, ourAddr := range addrs {
			ourPubKey = pubKeys[j]

			log.Println("   addr: " + ourAddr)
			log.Println("   Checking totalCount hypothesis")

			//=== get most current retrieved txs' blockheight (last line) from csv file and tx count
			blockHeightOld = taxcsv.GetLastBlockHeight(network.ChainName + "_" + ourAddr + ".csv")
			txCountOld = taxcsv.GetLastTxCount(network.ChainName + "_" + ourAddr + "_count.txt")

			//=== get only 1 tx to get header info about nr of total transactions
			totalCount = queryTotalCount(chainInfos[networkIdx].DaemonName, ourAddr)
			blockHeight = queryHeight(chainInfos[networkIdx].DaemonName, ourAddr, totalCount)
			log.Println("      [I] txCountOld/totalCount: " + strconv.Itoa(txCountOld) + "/" + strconv.Itoa(totalCount))

			//=== hypothesis check
			txCountUsed = txCountOld
			if txCountOld != 0 {
				txCountUsed = checkHypothesisUpdateTxCount(txCountOld, totalCount, blockHeight, blockHeightOld, chainInfos[networkIdx].DaemonName, ourAddr, cfg.Query.TxStepBack)
				//it is ensured that txCountUsed<=totalcount and in case they match, that also blockHeights match!
			}

			//replace it to use the possibly updated value
			txCountOld = txCountUsed

			log.Println("      [I] using txCount/totalCount: " + strconv.Itoa(txCountOld) + "/" + strconv.Itoa(totalCount))

			//=== new txs not yet retrieved?
			if totalCount == txCountOld {
				log.Println("   [OK] nothing to do")
				continue //noting to do
			}

			allNewTaxCsvRows := []*taxcsv.TaxCsv{}

			pageTotal = int(math.Ceil(float64(totalCount) / float64(pageLimit))) //with limit 1 totalPages would be totalCount
			utils.ErrDefaultFatal(err)                                           //on err log.Fatal with details

			//page to use in query
			page = int(math.Floor(float64(txCountOld)/float64(pageLimit))) + 1

			if page > pageTotal {
				continue //this should not happen, would be catched by totalCount above
			}

			for {
				//query 1st bunch
				txsResp := &TxsResp{}
				log.Println("   [I] querying page: " + strconv.Itoa(page) + "/" + strconv.Itoa(pageTotal) + " - this may take some time!")

				//--- query txs
				out, err := exec.Command(chainInfos[networkIdx].DaemonName, "query", "txs", "--events", "'message.sender="+ourAddr+"'", "--page", strconv.Itoa(page), "--limit", strconv.Itoa(pageLimit), "-out", "json").CombinedOutput()
				if err != nil {
					s := string(out)
					err = fmt.Errorf("%w; %v", err, s)
					utils.ErrDefaultFatal(err) //on err log.Fatal with detail
				}

				err = json.Unmarshal(out, &txsResp)
				utils.ErrDefaultFatal(err) //on err log.Fatal with details

				//get []*taxcsv.TaxCsv holding the relevant rows
				newTaxCsvRows := processRecTxs(txsResp, blockHeightOld, ourAddr, ourPubKey, cfg, networkIdx)
				if len(newTaxCsvRows) > 0 {
					allNewTaxCsvRows = append(allNewTaxCsvRows, newTaxCsvRows...)
				}

				//println(len(newTaxCsvRows))

				if txsResp.PageNumber == txsResp.PageTotal {
					//update lastCount
					txCountOld, err = strconv.Atoi(txsResp.TotalCount)
					utils.ErrDefaultFatal(err)
					//write to file
					taxcsv.UpdateLastTxCount(network.ChainName+"_"+ourAddr+"_count.txt", txCountOld)
					break // the loop over new pages
				}

				//increase page
				page += 1

			} //for over the pages

			//=== add FIAT base value for receivedAmount and feeAmount
			if len(allNewTaxCsvRows) > 0 {
				sLogSep = "   "
				log.Println(sLogSep + "Getting Fiat conversion for received and fee amounts")

				//get trade pairs sub struct for conversion
				tradePairs4Tax = &cfg.Networks[networkIdx].TradePairs4Tax

				//do the conversion for all rows
				exch.AddFiatBaseInfo2TaxCsvData(tradePairs4Tax, allNewTaxCsvRows, sLogSep+"   ")

				//=== append new rows to csv file
				taxcsv.AppendNewTaxRows(network.ChainName+"_"+ourAddr+".csv", allNewTaxCsvRows)

			}

			log.Println("   [OK] done, txCount now: " + strconv.Itoa(txCountOld))

		} //for over networks addresses in cfgAdr

	} // for over the networks

	log.Println("[OK] Querying networks for txs ==================================================================")

} //GetProcessTxsForNetworks

func processRecTxs(txsResp *TxsResp, blockHeightOld int, ourAddr string, ourPubKey string, cfg *configData.Cfg, networkIdx int) []*taxcsv.TaxCsv {

	taxRelMessageTypes := cfg.TaxRelevantMessageTypes
	newTaxCsvRows := []*taxcsv.TaxCsv{}
	var newTaxCsvRow *taxcsv.TaxCsv
	var tAtt, tMess, tWeSigned bool
	var currMess, s1, s2 string
	var n1, n2 int
	var feeAmount, recAmount float64
	var feeCurrency string
	var tAddedFees, tCoinReceived, tFeeRow, tFeesToBeAdded bool

	for _, tx := range txsResp.Txs {
		//--- check for blockheight newer than what we have
		heightInt, err := strconv.Atoi(tx.Height)
		utils.ErrDefaultFatal(err)

		if heightInt <= blockHeightOld {
			continue
		}

		//if heightInt == {
		//	println("Debug")
		//}
		//--------------------------------------------------------------
		//--- check for payed fees (if we paid)
		feeAmount = 0.0
		feeCurrency = ""
		tAddedFees = false //used below in cycle over events. strategy: add fees if we signed to first tax relevant message; in case there are several, the first has the fees; in case none, then teh fees are irrelevant
		tFeesToBeAdded = false
		tWeSigned = false

		//extract fee info
		for _, signInfo := range tx.Tx.AuthInfo.SignerInfos {
			if signInfo.PublicKey.Key == ourPubKey {
				tWeSigned = true
				break
			}
		}

		//process fee info
		if tWeSigned {
			if len(tx.Tx.AuthInfo.Fee.Amount) > 1 {
				log.Fatal("More than one fee entries, while we expected exact one for tx with hash: " + tx.TxHash + " for network: " + cfg.Networks[networkIdx].Name + ". " + utils.FatalDetails())
			}
			//if fee is 0 there might be no amount entry!
			if len(tx.Tx.AuthInfo.Fee.Amount) == 1 {
				if tx.Tx.AuthInfo.Fee.Amount[0].Denom != cfg.Networks[networkIdx].FeeDenom {
					log.Fatal("Fee denom: " + tx.Tx.AuthInfo.Fee.Amount[0].Denom + "does not match expected denom: " + cfg.Networks[networkIdx].FeeDenom + " for network: " + cfg.Networks[networkIdx].Name + ". " + utils.FatalDetails())
				} else {
					feeAmount, err = strconv.ParseFloat(tx.Tx.AuthInfo.Fee.Amount[0].Amount, 64)
					utils.ErrDefaultFatal(err)
					feeAmount = feeAmount / math.Pow10(cfg.Networks[networkIdx].Exponent)
					feeCurrency = cfg.Networks[networkIdx].Denom
					tFeesToBeAdded = true
				}
			}
		}
		//-> set feeAmount & feeCurrency
		//--- check for payed fees (if we paid)
		//--------------------------------------------------------------

		//--------------------------------------------------------------
		//--- extract relevant event info's
		//    be carefule: log contains several -events sections which each contains individial events (like 'coin_received')
		for _, logEvents := range tx.Logs {

			tCoinReceived = false
			tFeeRow = false
			//new data -> new empty row (only appended if MsgType matches)
			newTaxCsvRow = new(taxcsv.TaxCsv)
			newTaxCsvRow.Blockheight = heightInt
			newTaxCsvRow.Timestamp = tx.Timestamp
			newTaxCsvRow.Addr = ""
			newTaxCsvRow.Key = tx.Tx.AuthInfo.SignerInfos[0].PublicKey.Key
			newTaxCsvRow.MsgType = ""
			newTaxCsvRow.TxId = tx.TxHash
			newTaxCsvRow.Addr = ourAddr
			newTaxCsvRow.Key = ourPubKey
			newTaxCsvRow.ReceivedCurrency = cfg.Networks[networkIdx].Denom

			for _, event := range logEvents.Events {

				//log.Println("Event-type: " + event.Type)

				// keep coin received infos in case it is a tex relevant tx (see message check)
				if event.Type == "coin_received" {

					tAtt = false
					for _, attr := range event.Attributes {

						//log.Printf("Key: %v bool: %v attr.Value %v bool: %v", attr.Key, (attr.Key == "receiver"), attr.Value, (attr.Value == ourAddr))

						if attr.Key == "receiver" && attr.Value == ourAddr {
							tAtt = true //makr for next attr (is amount)
							tCoinReceived = true
							continue //next attribute
						}

						//if previous attr was receiver and our addr was given
						if attr.Key == "amount" && tAtt {
							//this is received on ourAddr
							//received is numberDenom
							n1 = len(cfg.Networks[networkIdx].FeeDenom)
							n2 = len(attr.Value)

							s1 = attr.Value[:n2-n1]
							s2 = attr.Value[n2-n1:]

							if s2 != cfg.Networks[networkIdx].FeeDenom {
								log.Fatal("Received value denom: " + s2 + "does not match expected denom: " + cfg.Networks[networkIdx].FeeDenom + " for network: " + cfg.Networks[networkIdx].Name + ". " + utils.FatalDetails())
							} else {
								recAmount, err = strconv.ParseFloat(s1, 64)
								utils.ErrDefaultFatal(err)
								newTaxCsvRow.ReceivedAmount += recAmount / math.Pow10(cfg.Networks[networkIdx].Exponent)
							}

						}

						//reset tAtt
						tAtt = false
					} //for over attributes of this event

				}

				//in message event look for action's value:
				if event.Type == "message" {
					tMess = false
					for _, attr := range event.Attributes {
						if attr.Key == "action" {
							currMess = attr.Value
							if slices.Contains(taxRelMessageTypes, currMess) {
								tMess = true
								if newTaxCsvRow.MsgType != "" {
									log.Println("Warning: second message/action for tx: " + tx.TxHash + " for network: " + cfg.Networks[networkIdx].Name + ". " + utils.FatalDetails())
									log.Println("	-> check this!")
								}
								newTaxCsvRow.MsgType += currMess //append, as we might have a mulit message tx
								//break
							}
						}
					}
					//if we did not find a tax relevant message, report the message type
					if !tMess {
						log.Println("   [I] skipping unhandled MsgType: " + currMess)
					}
				} // if event Message

			} // for over events entries - the events: contents

			// continue for tax-irrelevant message type
			if newTaxCsvRow.MsgType == "" {
				continue
			}

			//
			//check do we process this message; not empty message -> tax relevant message, coins received are tax relevant
			//we already called continue above in case of irrelevant message
			if newTaxCsvRow.MsgType != "" {

				//once add the fees (to the first tax relevant row)
				if !tAddedFees && tFeesToBeAdded {
					//
					newTaxCsvRow.FeeAmount = feeAmount
					newTaxCsvRow.FeeCurrency = feeCurrency
					tAddedFees = true //used below in cycle over events. strategy: add fees if we signed to first tax relevant message; in case there are several, the first has the fees; in case none, then teh fees are irrelevant
					tFeeRow = true
				}
				//--- append the row only if not blank
				if tFeeRow || tCoinReceived {
					newTaxCsvRows = append(newTaxCsvRows, newTaxCsvRow)
				}

			}

		} // for over events (the logs: -events)
		//--- extract relevant event info
		//--------------------------------------------------------------

	} //for over all received transactions

	return newTaxCsvRows
}

//
//returns correct txCount
func checkHypothesisUpdateTxCount(txCountOld int, totalCount int, blockHeight int, blockHeightOld int, daemonName string, ourAddr string, txStepBack int) int {

	//=== hypothesis check: blockHeightOld is from last tx we received for txCountOld; if there has been pruning in the meantime,
	//	  this does not match anymore. Cases:
	//    1) totalCount < txCountOld -> there was pruning, we need to go back in txs until we find a blockHeight<= blockHeightOld
	//    2) totalCount == txCountOld, a) blockHeight = blockHeightOld: no new txs or b) blockHeight > blockHeightOld more pruned -> correct txCountOld as with 1)
	//    3) totalCount > txCountOld, a) blockHeight = blockHeightOld: only new txs or b) blockHeight < blockHeightOld there was pruning formerly, but not in this query -> we can keep txCountOld

	var height int
	var tUpdateTxCount bool
	var txCountOldUpdated int
	var tFound bool
	tUpdateTxCount = false

	//=== check if ne need to scan backwards for tx
	if totalCount < txCountOld {
		log.Println("      [I] totalCount is smaller than txCountOld -> there has been pruning -> scanning (backwards) for matching and re-syncing txCount")
		tUpdateTxCount = true

	} else {
		if totalCount > txCountOld {
			//fetch blockHeight for our last count, to see if it matches
			blockHeight = queryHeight(daemonName, ourAddr, txCountOld)
		}

		if blockHeight == blockHeightOld {
			log.Println("   [OK] totalCount matches txCountOld and blockHeights match")

		} else if blockHeight > blockHeightOld {
			tUpdateTxCount = true
			log.Println("      [I] blockHeight > blockHeightOld -> there has been pruning -> scanning (backwards) for matching tx and re-syncing txCount")

		} else {
			//here we are left with blockHeight < blockHeightOld
			log.Println("      [I] blockHeight < blockHeightOld -> there has been pruning formerly, but now we have more txs again -> we can however go on with txCountOld")
		}

	}

	if !tUpdateTxCount {
		txCountOldUpdated = txCountOld
		return txCountOldUpdated
	} else {
		//start backward quering
		//var txsRespThin *TxsResp
		//get exact the totalCount tx, blockHeightOld should be equal to txsRespThin.Txs[0].Height
		//txsRespThin = queryOneTx(chainInfos[i].DaemonName, ourAddr, totalCount)
		var stepBackUsed int

		//inint stepback
		stepBackUsed = txStepBack

		//init with minimum of both as starting point
		txCountOldUpdated = utils.MinInt(txCountOld, totalCount)
		tFound = false

		for {

			//go back at max to first tx that can be retrieved
			if (txCountOldUpdated - stepBackUsed) >= 1 {
				txCountOldUpdated = txCountOldUpdated - stepBackUsed
			} else {
				txCountOldUpdated = 1
			}

			log.Printf("          trying %v of %v", txCountOldUpdated, totalCount)

			//query the height at this txCount
			height = queryHeight(daemonName, ourAddr, txCountOldUpdated)

			if height <= blockHeightOld {
				tFound = true
				break

			} else if txCountOldUpdated == 1 {
				break
			}
			//
			stepBackUsed = stepBackUsed * 2
		}

		if !tFound {
			log.Println("      [WARN] we could not find blockHeight < blockHeightOld -> there has been pruning more pruning than we had retrieved in last query")
			log.Println("             -> all available txs will bre retrived, but there are txs MISSING in the csv. You need to query an archive node to get them!")
		} else {
			log.Println("      [OK] txCount with blockHeight < blockHeightOld found")
		}

		return txCountOldUpdated
	}

} //checkHypothesisUpdateTxCount

func GetTxCountForAllRpcNodes(cfg *configData.Cfg, cfgAdr *configData.CfgAdr, chainInfos []nw.ChainInfo, chainName string) {
	var networkIdx int
	var addrs []string
	//var pubKeys []string
	//var blockHeightOld, blockHeight int
	var txCountOld int
	var totalCount int
	//var ourPubKey string
	var nodeAddr string

	log.Println("Querying network " + chainName + "for txCount =======================================================================")

	//get cfg's networks and relevant message types
	networks := cfg.GetNetworksFieldString("Name")

	//get index of given chainName
	cfgAdrIdx := indexInCfgAdr4chainName(cfgAdr, chainName)

	//get the idx in the cfg
	networkIdx = slices.Index(networks, chainName)
	if networkIdx == -1 {
		log.Println("Skipping unknonw network:" + chainName + " not present in config")
		return
	}

	//here we have a valid network; extract given addr and pubkeys as slice (for simple Contains check later on)
	addrs = cfgAdr.GetFieldString(cfgAdrIdx, "Addr")
	//pubKeys = cfgAdr.GetFieldString(i, "PubKey")

	for _, node := range chainInfos[networkIdx].Apis.Rpc {

		//nodeAddr = nw.EnsurePortInAddress(node.Address)

		nodeAddr = nw.CheckNode(chainInfos[networkIdx].DaemonName, node.Address, true)
		if nodeAddr == "" {
			log.Println("      [I] node skipped (not responsive): " + node.Address) //using node.Address here as nodeAddr is empty if not responsive
			continue
		}

		//we need to handle each address individually, as cosmos query does not provide || for event filter (only &&)
		//-> we can only get the txs per address individually
		for _, ourAddr := range addrs {
			//ourPubKey = pubKeys[j]

			//log.Println("   addr: " + ourAddr)
			//log.Println("   Checking totalCount hypothesis")

			//=== get most current retrieved txs' blockheight (last line) from csv file and tx count
			//blockHeightOld = taxcsv.GetLastBlockHeight(chainName + "_" + ourAddr + ".csv")
			txCountOld = taxcsv.GetLastTxCount(chainName + "_" + ourAddr + "_count.txt")

			//=== get only 1 tx to get header info about nr of total transactions
			totalCount = queryTotalCountFromNode(chainInfos[networkIdx].DaemonName, ourAddr, nodeAddr)
			//blockHeight = queryHeightFromNode(chainInfos[i].DaemonName, ourAddr, totalCount, node.Address)
			log.Println("      [I] node: txCountOld/totalCount: " + nodeAddr + ": " + strconv.Itoa(txCountOld) + "/" + strconv.Itoa(totalCount))

		} //for over networks addresses in cfgAdr

	} // for over the rpc nodes

	log.Println("[OK] Done querying nodes for txcount ==================================================================")

} //GetTxCountForAllRpcNodes

// get only 1 tx to get header info about nr of total transactions
func queryTotalCount(daemonName string, ourAddr string) int {
	var totalCount int
	var txsRespThin *TxsResp
	var err error

	txsRespThin = queryOneTx(daemonName, ourAddr, 1)

	totalCount, err = strconv.Atoi(txsRespThin.TotalCount)
	utils.ErrDefaultFatal(err) //on err log.Fatal with details

	return totalCount
}

// get only 1 tx to get header info about nr of total transactions from a specific node
func queryTotalCountFromNode(daemonName string, ourAddr string, node string) int {
	var totalCount int
	var txsRespThin *TxsResp
	var err error

	txsRespThin = queryOneTxFromNode(daemonName, ourAddr, node, 1)

	totalCount, err = strconv.Atoi(txsRespThin.TotalCount)
	utils.ErrDefaultFatal(err) //on err log.Fatal with details

	return totalCount
}

func queryHeight(daemonName string, ourAddr string, txCount int) int {
	var height int
	var txsRespThin *TxsResp
	var err error

	txsRespThin = queryOneTx(daemonName, ourAddr, txCount)

	height, err = strconv.Atoi(txsRespThin.Txs[0].Height)
	utils.ErrDefaultFatal(err)

	return height
}

func queryHeightFromNode(daemonName string, ourAddr string, txCount int, node string) int {
	var height int
	var txsRespThin *TxsResp
	var err error

	txsRespThin = queryOneTxFromNode(daemonName, ourAddr, node, txCount)

	height, err = strconv.Atoi(txsRespThin.Txs[0].Height)
	utils.ErrDefaultFatal(err)

	return height
}

// get only 1 tx and unmarhsal
func queryOneTx(daemonName string, ourAddr string, page int) *TxsResp {

	txsRespThin := &TxsResp{}
	out, err := exec.Command(daemonName, "query", "txs", "--events", "'message.sender="+ourAddr+"'", "--page", strconv.Itoa(page), "--limit", "1", "-out", "json").CombinedOutput()
	if err != nil {
		s := string(out)
		err = fmt.Errorf("%w; %v", err, s)
		utils.ErrDefaultFatal(err) //on err log.Fatal with detail
	}
	err = json.Unmarshal(out, &txsRespThin)
	utils.ErrDefaultFatal(err) //on err log.Fatal with details

	return txsRespThin
}

// get only 1 tx and unmarhsal
func queryOneTxFromNode(daemonName string, ourAddr string, node string, page int) *TxsResp {

	txsRespThin := &TxsResp{}
	out, err := exec.Command(daemonName, "--node", node, "query", "txs", "--events", "'message.sender="+ourAddr+"'", "--page", strconv.Itoa(page), "--limit", "1", "-out", "json").CombinedOutput()

	if err != nil {
		s := string(out)
		err = fmt.Errorf("%w; %v", err, s)
		utils.ErrDefaultFatal(err) //on err log.Fatal with detail
	}
	err = json.Unmarshal(out, &txsRespThin)
	utils.ErrDefaultFatal(err) //on err log.Fatal with details

	return txsRespThin
}

//This is similar to the standard Index function for slices, but applied
//to our slice cfgAdr.Addresses holding the chainName in a substruct
func indexInCfgAdr4chainName(cfgAdr *configData.CfgAdr, chainName string) int {
	for i, vs := range cfgAdr.Addresses {
		if chainName == vs.ChainName {
			return i
		}
	}
	return -1
}
