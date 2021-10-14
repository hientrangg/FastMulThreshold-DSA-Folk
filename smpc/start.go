/*
 *  Copyright (C) 2018-2019  Fusion Foundation Ltd. All rights reserved.
 *  Copyright (C) 2018-2019  haijun.cai@anyswap.exchange
 *
 *  This library is free software; you can redistribute it and/or
 *  modify it under the Apache License, Version 2.0.
 *
 *  This library is distributed in the hope that it will be useful,
 *  but WITHOUT ANY WARRANTY; without even the implied warranty of
 *  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.
 *
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package smpc

import (
	"github.com/anyswap/Anyswap-MPCNode/internal/common"
	p2psmpc "github.com/anyswap/Anyswap-MPCNode/p2p/layer2"
	smpclibec2 "github.com/anyswap/Anyswap-MPCNode/smpc-lib/crypto/ec2"
	"github.com/fsn-dev/cryptoCoins/coins"
	cryptocoinsconfig "github.com/fsn-dev/cryptoCoins/coins/config"
	"github.com/fsn-dev/cryptoCoins/coins/eos"
	"os"
)

var (
	cur_enode    string
	init_times   = 0
	recalc_times = 1
	KeyFile      string
)

func init() {
	p2psmpc.RegisterRecvCallback(Call2)
	p2psmpc.SdkProtocol_registerBroadcastInGroupCallback(Call)
	p2psmpc.RegisterCallback(Call)

	RegP2pGetGroupCallBack(p2psmpc.SdkProtocol_getGroup)
	RegP2pSendToGroupAllNodesCallBack(p2psmpc.SdkProtocol_SendToGroupAllNodes)
	RegP2pGetSelfEnodeCallBack(p2psmpc.GetSelfID)
	RegP2pBroadcastInGroupOthersCallBack(p2psmpc.SdkProtocol_broadcastInGroupOthers)
	RegP2pSendMsgToPeerCallBack(p2psmpc.SendMsgToPeer)
	RegP2pParseNodeCallBack(p2psmpc.ParseNodeID)
	RegSmpcGetEosAccountCallBack(eos.GetEosAccount)
	InitChan()
}

//------------------------------------------------------------------------

type LunchParams struct {
	WaitMsg      uint64
	TryTimes     uint64
	PreSignNum   uint64
	WaitAgree    uint64
	Bip32Pre     uint64
	Sync_PreSign string
}

// Start init gsmpc
// 1. Initialization: local database (including general database, private key database, bip32 c value database,bip32 pre-sign data database, pre-sign data database, public key group information database, database for saving data related to generate pubkey command, database for saving data related to signature command, database for saving data related to resare command, pubkey), P2P callback function, Crypto coins configuration, startup parameters (including the number of pre generated packets, the timeout waiting for P2P information, the number of automatic retries after failed address application or signature, the timeout agreed by the nodes, whether to synchronize pre generated packets between nodes, etc.), and the enodeid of the local node.
// 2. Load the pubkeys generated by history and execute it only once.
// 3. Generate 4 large prime numbers
// 4. Execute automatic pre generation of data packets.
// 5. Listen for the arrival of the sign command.
// 6. Delete the data related to generating pubkey command, the signature command and the restore command from the corresponding sub database, and correspondingly change the status of the command data to timeout in the general database.
func Start(params *LunchParams) {

	cryptocoinsconfig.Init()
	coins.Init()

	cur_enode = p2psmpc.GetSelfID()
	accloaded := AccountLoaded()

	go smpclibec2.GenRandomSafePrime()

	common.Info("======================smpc.Start======================", "accounts loaded", accloaded, "cache", cache, "handles", handles, "cur enode", cur_enode)
	err := StartSmpcLocalDb()
	if err != nil {
		info := "======================smpc.Start," + err.Error() + ",so terminate smpc node startup"
		common.Error(info)
		os.Exit(1)
		return
	}

	common.Info("======================smpc.Start,open all db success======================", "cur_enode", cur_enode)

	PrePubDataCount = int(params.PreSignNum)
	WaitMsgTimeGG20 = int(params.WaitMsg)
	recalc_times = int(params.TryTimes)
	waitallgg20 = WaitMsgTimeGG20 * recalc_times
	WaitAgree = int(params.WaitAgree)
	PreBip32DataCount = int(params.Bip32Pre)
	if params.Sync_PreSign == "true" {
		syncpresign = true
	} else {
		syncpresign = false
	}

	AutoPreGenSignData()

	go HandleRpcSign()

	// do this must after openning accounts db success,but get accloaded must before it
	if !accloaded {
		go CopyAllAccountsFromDb()
	}

	CleanUpAllReqAddrInfo()
	CleanUpAllSignInfo()
	CleanUpAllReshareInfo()

	common.Info("================================smpc.Start,init finish.========================", "cur_enode", cur_enode, "waitmsg", WaitMsgTimeGG20, "trytimes", recalc_times, "presignnum", PrePubDataCount, "bip32pre", PreBip32DataCount)
}
