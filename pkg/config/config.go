// config.go
package config

import (
	"alexp/stakingtax/pkg/configData"
	"alexp/stakingtax/pkg/utils"

	"os"

	"gopkg.in/yaml.v3"
)

//read config from yaml file
func GetConfigFromFile(configPathFile string, cfg *configData.Cfg) {

	//--- open config file
	file, err := os.Open(configPathFile)
	utils.ErrDefaultFatal(err) //on err log.Fatal with details
	defer file.Close()

	//--- populate
	// Init new YAML decode
	d := yaml.NewDecoder(file)

	// Start YAML decoding from file
	err = d.Decode(cfg)
	utils.ErrDefaultFatal(err) //on err log.Fatal with details

}

//read config from yaml file
func GetAddrFromFile(configPathFile string, cfgAdr *configData.CfgAdr) {

	//--- open config file
	file, err := os.Open(configPathFile)
	utils.ErrDefaultFatal(err) //on err log.Fatal with details
	defer file.Close()

	//--- populate
	// Init new YAML decode
	d := yaml.NewDecoder(file)

	// Start YAML decoding from file
	err = d.Decode(cfgAdr)
	utils.ErrDefaultFatal(err) //on err log.Fatal with details

}

//
// func validateConfigFile(pathFile string) error {
// 	s, err := os.Stat(pathFile)
// 	if err != nil {
// 		return err
// 	}
// 	if s.IsDir() {
// 		return fmt.Errorf("'%s' is a directory, not a normal file", pathFile)
// 	}
// 	return nil
// }
