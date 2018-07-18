// Copyright © 2018 Joel Rebello <joel.rebello@booking.com>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"fmt"
	"github.com/bmc-toolbox/bmcbutler/asset"
	"github.com/bmc-toolbox/bmcbutler/butler"
	"github.com/bmc-toolbox/bmcbutler/inventory"
	"github.com/bmc-toolbox/bmcbutler/resource"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"os"
)

// setupCmd represents the setup command
var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Setup onetime configuration for BMCs.",
	Long: `Some BMC configuration options must be set just once,
and this config can cause the BMC and or its dependencies to power reset,
for example: disabling/enabling flex addresses on the Dell m1000e,
this requires all the blades in the chassis to be power cycled.`,
	Run: func(cmd *cobra.Command, args []string) {
		setup()
	},
}

func init() {
	rootCmd.AddCommand(setupCmd)

}

func setup() {

	// A channel to recieve inventory assets
	inventoryChan := make(chan []asset.Asset)

	inventorySource := viper.GetString("inventory.setup.source")
	butlersToSpawn := viper.GetInt("butlersToSpawn")

	if butlersToSpawn == 0 {
		butlersToSpawn = 5
	}

	switch inventorySource {
	case "needSetup":
		inventoryInstance := inventory.NeedSetup{Log: log, BatchSize: 10, Channel: inventoryChan}
		// Spawn a goroutine that returns a slice of assets over inventoryChan
		// the number of assets in the slice is determined by the batch size.
		if serial == "" {
			go inventoryInstance.AssetIter()
		} else {
			go inventoryInstance.AssetIterBySerial(serial, assetType)
		}
	default:
		fmt.Println("Unknown inventory source declared in cfg: ", inventorySource)
		os.Exit(1)
	}

	// Read in declared resources for one time setup
	resourceInstance := resource.Resource{Log: log}
	config := resourceInstance.ReadResourcesSetup()

	// Spawn butlers to work
	butlerChan := make(chan butler.ButlerMsg, 10)
	butlerInstance := butler.Butler{Log: log, SpawnCount: butlersToSpawn, Channel: butlerChan}
	if serial != "" || ignoreLocation {
		butlerInstance.IgnoreLocation = true
	}
	go butlerInstance.Spawn()

	//over inventory channel and pass asset lists recieved to bmcbutlers
	for assetList := range inventoryChan {
		for _, asset := range assetList {
			butlerMsg := butler.ButlerMsg{Asset: asset, Setup: config}
			butlerChan <- butlerMsg
		}
	}

	close(butlerChan)
	//wait until butlers are done.
	butlerInstance.Wait()
}
