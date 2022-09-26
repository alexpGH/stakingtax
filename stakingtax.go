// stakingtax.go
package main

import (
	"alexp/stakingtax/pkg/config"
	"alexp/stakingtax/pkg/configData"
	nw "alexp/stakingtax/pkg/network"
	"alexp/stakingtax/pkg/txs"
	"flag"
	"log"
)

//config flags from command line
type Cfl struct {
	configPathFile string
	addrPathFile   string
	tHelp          bool
	tCheckOnly     bool
	tQueryRpcNodes bool
}

// logger configured to emit app name, line number, timestamps etc.
//var mylog = log.New(os.Stderr, "app: ", log.LstdFlags|log.Lshortfile)

//==============================================================================
//=== main
//==============================================================================
func main() {
	//argsWithoutProg := os.Args[1:]
	//log.Println(argsWithoutProg)
	//log.SetFlags(log.LstdFlags | log.Lshortfile)

	//get empty config structs
	cfg := &configData.Cfg{}
	cfgAdr := &configData.CfgAdr{}
	cfl := &Cfl{}

	//=== set up & parse all the command line flags stuff
	initFlags(cfl) //sets up flags containers
	flag.Parse()   //parse the command line for flags

	//print help
	if cfl.tHelp {
		flag.PrintDefaults()
		return
	}

	//=== read config file
	config.GetConfigFromFile(cfl.configPathFile, cfg)
	config.GetAddrFromFile(cfl.addrPathFile, cfgAdr)

	//=== check for all networks: version, rpc endpoints etc.
	chainInfos := nw.CheckNetworks(cfg)

	if cfl.tCheckOnly {
		log.Println("Check networks only done.")
		return
	}

	if cfl.tQueryRpcNodes {
		for _, v := range chainInfos {
			txs.GetTxCountForAllRpcNodes(cfg, cfgAdr, chainInfos, v.ChainName)
		}
		log.Println("Querying RPC nodes done.")
		return
	}

	//=== now get and process all txs
	txs.GetProcessTxsForNetworks(cfg, cfgAdr, chainInfos)

}

//set up flags containers
func initFlags(cfl *Cfl) {
	//fmt.Println("Init")
	flag.BoolVar(&cfl.tHelp, "help", false, "print help")
	flag.StringVar(&cfl.configPathFile, "configPathFile", "./config.yaml", "name (and path) of config file")
	flag.StringVar(&cfl.addrPathFile, "addrPathFile", "./addr.yaml", "name (and path) of file holding the addresses to scan for tax relevant txs")
	flag.BoolVar(&cfl.tCheckOnly, "checkOnly", false, "only check/update the networks configuration")
	flag.BoolVar(&cfl.tQueryRpcNodes, "queryRpcNodes", false, "only query the RPC nodes for their number of relevant txs")

}
