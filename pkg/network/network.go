// network.go
package network

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"time"

	"golang.org/x/exp/slices"

	"alexp/stakingtax/pkg/configData"
	"alexp/stakingtax/pkg/utils"
)

type ChainInfoSub struct {
	RecommendedVersion string   `json:"recommended_version"`
	CompatibleVersions []string `json:"compatible_versions"`
}

// chainInfo subpart of the json
type ChainInfo struct {
	ChainName  string `json:"chain_name"`
	ChainId    string `json:"chain_id"`
	DaemonName string `json:"daemon_name"`
	Codebase   struct {
		RecommendedVersion string         `json:"recommended_version"`
		CompatibleVersions []string       `json:"compatible_versions"`
		Versions           []ChainInfoSub `json:"versions"` //since 2023 there is also versions and not only codebase; however still use codebase (versions seem only having extra entries during upgrade)
	} `json:"codebase"`
	Apis struct {
		Rpc []struct {
			Address string `json:"address"`
		} `json:"rpc"`
	} `json:"apis"`
	TProcess bool `json:"ourExtraParameter"`
}

// checks network configurations and possibly updates the config
func CheckNetworks(cfg *configData.Cfg) []ChainInfo {

	var chainInfos []ChainInfo
	var chainName string

	log.Println("Checking networks ===============================================================================")

	for i, chainDetails := range cfg.Networks {
		chainName = chainDetails.Name
		log.Println("Checking: " + chainName + " --------------------------------------------------------")
		//fetch chain_info from git hub
		chainInfos = append(chainInfos, fetchChainInfo(cfg, chainName))

		ensureValidDaemonVersion(chainInfos[i])
		ensureCorrectConfigChainId(chainInfos[i])
		ensureCorrectConfigNode(&chainInfos[i], chainDetails.KeepConfigNode)
	}
	log.Println("[OK] checking networks ==========================================================================")
	log.Println("")

	return chainInfos
} // CheckNetworks

// fetches chain_info.json from github chain-registry
func fetchChainInfo(cfg *configData.Cfg, chainName string) ChainInfo {

	webAddr := cfg.NetworksBasics.ChainRegistry + cfg.NetworksBasics.ChainExtraPath + chainName + cfg.NetworksBasics.ChainInfo
	res, err := http.Get(webAddr)
	utils.ErrDefaultFatal(err) //on err log.Fatal with details
	defer res.Body.Close()

	if res.StatusCode != 200 {
		err = fmt.Errorf("%w;Fetching %s's chain info failed with statusCode: %d and status %s", err, chainName, res.StatusCode, res.Status)
		utils.ErrDefaultFatal(err) //on err log.Fatal with details
	}

	chainI := ChainInfo{}
	err = json.NewDecoder(res.Body).Decode(&chainI)
	utils.ErrDefaultFatal(err) //on err log.Fatal with details

	//sanity check: do we have the correct chain?
	if chainI.ChainName != chainName {
		log.Fatal("Chain name from config - " + chainName + "- and from github's jsons - " + chainI.ChainName + " - do not match!")
	}

	//set our internal parameter - might be set to false in case node is not responsive etc.
	chainI.TProcess = true
	return chainI

} //fetchChainInfo

// check to have a compatible daemon version
func ensureValidDaemonVersion(chainI ChainInfo) {

	//--- switch for using Codebase.Versions instead of Codebase
	tUseCodebase := true // default is using codebase; version list has more than the trivial (same as Codebase) entry only during upgrade phases.

	//--- check daemon version
	out, err := exec.Command(chainI.DaemonName, "version").CombinedOutput()
	utils.ErrDefaultFatal(err) //on err log.Fatal with detail
	//convert to string, remove newline characters
	yourV := utils.StringCleaned(out)

	//
	if tUseCodebase {
		// --- use Codebase
		if yourV != chainI.Codebase.RecommendedVersion {
			log.Printf("Your daemon version is %v, while recommended in codebase is %v\n", yourV, chainI.Codebase.RecommendedVersion)

			if slices.Contains(chainI.Codebase.CompatibleVersions, yourV) {
				log.Println("[OK] Found your daemon version in codebase's compatible versions -> going on")
			} else {
				myS := fmt.Sprintf("%v", chainI.Codebase.CompatibleVersions)
				log.Fatal("Your daemon version is also not in the list of compatible versions: " + myS + ". " + utils.FatalDetails())
			}
		} else {
			log.Println("[OK] Your daemon version: " + yourV + " is up to date (compared to codebase)!")
		}

	} else {
		//--- use Codebase.Versions sublist
		if yourV != chainI.Codebase.Versions[0].RecommendedVersion {
			log.Printf("Your daemon version is %v, while recommended is %v\n", yourV, chainI.Codebase.Versions[0].RecommendedVersion)

			if slices.Contains(chainI.Codebase.Versions[0].CompatibleVersions, yourV) {
				log.Println("[OK] Found your daemon version in compatible versions -> going on")
			} else {
				myS := fmt.Sprintf("%v", chainI.Codebase.Versions[0].CompatibleVersions)
				log.Fatal("Your daemon version is also not in the list of compatible versions: " + myS + ". " + utils.FatalDetails())
			}
		} else {
			log.Println("[OK] Your daemon version: " + yourV + " is up to date!")
		}

	}

} //ensureValidDaemonVersion

// ensures the correct chain_id and a responsive node in the config
func ensureCorrectConfigChainId(chainI ChainInfo) {
	log.Println("Checking chain-Id in config")

	//--- check and update chain-id
	out, err := exec.Command(chainI.DaemonName, "config", "chain-id").CombinedOutput()
	utils.ErrDefaultFatal(err) //on err log.Fatal with details
	//convert to string, remove newline characters
	chIdCurr := utils.StringCleaned(out)

	if chIdCurr != chainI.ChainId {
		log.Println("	Your chain-id: " + chIdCurr + " does not match the current chain (from chainregistry): " + chainI.ChainId + " => replacing it in the config")

		//--- try to update config
		_, err = exec.Command(chainI.DaemonName, "config", "chain-id", chainI.ChainId).CombinedOutput()
		utils.ErrDefaultFatal(err) //on err log.Fatal with details

		//--- check to ensure it was successfull
		out, err = exec.Command(chainI.DaemonName, "config", "chain-id").CombinedOutput()
		utils.ErrDefaultFatal(err) //on err log.Fatal with details
		chIdCurr = utils.StringCleaned(out)
		if chIdCurr != chainI.ChainId {
			log.Fatal("Failed to update config " + utils.FatalDetails())
		} else {
			log.Println("	...done.")
		}

	} else {
		log.Println("	[OK] Your chain-Id setting was correct: " + chIdCurr)
	}

} //ensureCorrectConfigChainId

// helper function: checks status of a gien node - is it responsive?
func CheckNode(daemonName string, currAddr string, tcheckAddChannel bool) string {

	finalAddr := currAddr
	if tcheckAddChannel {
		finalAddr = EnsurePortInAddress(currAddr)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel() // The cancel should be deferred so resources are cleaned up
	cmd := exec.CommandContext(ctx, daemonName, "status", "--node", finalAddr)
	_, err := cmd.Output()

	if err != nil {
		return ""
	} else {
		return finalAddr
	}
} //checkNode

func EnsurePortInAddress(currAddr string) string {
	finalAddr := currAddr
	//check if there is a :channel present, if not use default :443 (assumes a 3 digit channel)
	//fmt.Println(currAddr[len(currAddr)-4 : len(currAddr)-3])
	if currAddr[len(currAddr)-4:len(currAddr)-3] != ":" {
		//check for trailng / which is invalid
		//log.Print(currAddr[len(currAddr)-1:])
		if currAddr[len(currAddr)-1:] == "/" {
			finalAddr = currAddr[0 : len(currAddr)-1]
		}
		finalAddr += ":443"
	}
	return finalAddr
}

// ensures the node config to be correct (responsive node)
func ensureCorrectConfigNode(chainI *ChainInfo, keepConfigNode bool) {
	log.Println("Checking to have a responsive node")

	out, err := exec.Command(chainI.DaemonName, "config", "node").CombinedOutput()
	utils.ErrDefaultFatal(err) //on err log.Fatal with details
	//convert to string, remove newline characters
	nodeCurr := utils.StringCleaned(out)

	if nodeCurr != "" {
		log.Println("	Checking responsiveness of your node in config: " + nodeCurr)
		addrUsed := CheckNode(chainI.DaemonName, nodeCurr, false) //false: do not check channel, use it as it is
		if addrUsed != "" {
			log.Println("	[OK] -> node responded")
			return
		} else {
			if keepConfigNode {
				chainI.TProcess = false //mark as not to be processed
				log.Println("	-> node did not respond, but keepConfigNode in config.yaml=true -> excluding this network from further processing in this run")
				return
			} else {
				log.Println("	-> node did not respond, keepConfigNode in config.yaml=false -> searching a responsive one")
				chainI.TProcess = false //for now
			}
		}
	}

	//here we are left with the task to find a responsive node from the chain registry list
	tFound := false
	var addrUsed string
	for _, v := range chainI.Apis.Rpc {
		addrUsed = CheckNode(chainI.DaemonName, v.Address, true) //true: check channel
		if addrUsed != "" {
			log.Println("	[OK] -> " + addrUsed + " responded, adding it to config")
			tFound = true
			break
		}
	}

	if tFound {
		chainI.TProcess = true //reactivate as we found a responsive node
	} else {
		log.Println("	No responsive node found, giving up.")
		return
	}

	//--- try to update config
	_, err = exec.Command(chainI.DaemonName, "config", "node", addrUsed).CombinedOutput()
	utils.ErrDefaultFatal(err) //on err log.Fatal with details

	//--- check to ensure it was successfull
	out, err = exec.Command(chainI.DaemonName, "config", "node").CombinedOutput()
	utils.ErrDefaultFatal(err) //on err log.Fatal with details
	nodeFinal := utils.StringCleaned(out)
	if nodeFinal != addrUsed {
		log.Fatal("	Failed to update config for node setting" + utils.FatalDetails())
	} else {
		log.Println("	[OK] ...done.")
	}
} //ensureCorrectConfigNode

//This is similar to the standard Index function for slices, but applied
//to our slice holding the address in a substruct Address from json unmarshalling
//func indexAddressField(chainI ChainInfo, addr string) int {
//	for i, vs := range chainI.Apis.Rpc {
//		if addr == vs.Address {
//			return i
//		}
//	}
//	return -1
//}
