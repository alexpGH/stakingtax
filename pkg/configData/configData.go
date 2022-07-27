// configData.go
package configData

import (
	"reflect"
)

type TradePairs4TaxType struct {
	EndPoint string   `yaml:"endpoint"`
	Pairs    []string `yaml:"pairs"`
}

//config from yaml file
//note to unmarshall the data to the struct, the fields must be public(uppercase)
type Cfg struct {
	NetworksBasics struct {
		ChainRegistry  string `yaml:"chainRegistry"`
		ChainExtraPath string `yaml:"chainExtraPath"`
		ChainInfo      string `yaml:"chainInfo"`
	} `yaml:"networksBasics"`
	Networks []struct {
		Name           string             `yaml:"name"`
		Denom          string             `yaml:"denom"`
		Exponent       int                `yaml:"exponent"`
		FeeDenom       string             `yaml:"feedenom"`
		TradePairs4Tax TradePairs4TaxType `yaml:"tradePairs4Tax"`
		// TradePairs4Tax struct {
		// 	EndPoint string   `yaml:"endpoint"`
		// 	Pairs    []string `yaml:"pairs"`
		// } `yaml:"tradePairs4Tax"`
	} `yaml:"networks"`
	Query struct {
		PageLimit int `yaml:"pageLimit"`
	} `yaml:"query"`
	TaxRelevantMessageTypes []string `yaml:"taxRelevantMessageTypes"`
}

type CfgAdr struct {
	Addresses []struct {
		ChainName string `yaml:"chainName"`
		AddrList  []struct {
			Addr   string `yaml:"addr"`
			PubKey string `yaml:"pubKey"`
		} `yaml:"addrList"`
	} `yaml:"addresses"`
}

//allows to extract the Addr or Pubkeys as slice per chain
func (cfgAdr *CfgAdr) GetFieldString(addrIdx int, field string) []string {
	var data []string
	for _, addrList := range cfgAdr.Addresses[addrIdx].AddrList {
		r := reflect.ValueOf(addrList)
		f := reflect.Indirect(r).FieldByName(field)
		data = append(data, f.String())
	}
	return data
}

//allows to extract the Addr or Pubkeys as slice per chain
func (cfg *Cfg) GetNetworksFieldString(field string) []string {
	var data []string
	for _, network := range cfg.Networks {
		r := reflect.ValueOf(network)
		f := reflect.Indirect(r).FieldByName(field)
		data = append(data, f.String())
	}
	return data
}
