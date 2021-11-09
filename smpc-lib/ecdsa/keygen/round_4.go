/*
 *  Copyright (C) 2020-2021  AnySwap Ltd. All rights reserved.
 *  Copyright (C) 2020-2021  haijun.cai@anyswap.exchange
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

package keygen

import (
	"errors"
	"fmt"
	//"time"
	"sync"
	"github.com/anyswap/Anyswap-MPCNode/crypto/secp256k1"
	"github.com/anyswap/Anyswap-MPCNode/smpc-lib/crypto/ec2"
	"github.com/anyswap/Anyswap-MPCNode/smpc-lib/smpc"
	"math/big"
	"github.com/anyswap/Anyswap-MPCNode/internal/common"
	//"crypto/rand"
	//"github.com/anyswap/Anyswap-MPCNode/internal/common/math/random"
)

var (
	mutex    sync.Mutex
	HoeffdingBound *big.Int // m = k*32*ln2
)

// Start verify vss and commitment data,calc pubkey and SKi,create Ntilde
func (round *round4) Start() error {
	if round.started {
		return errors.New("round already started")
	}
	round.number = 4
	round.started = true
	round.resetOK()

	curIndex, err := round.GetDNodeIDIndex(round.dnodeid)
	if err != nil {
		return err
	}

	ids, err := round.GetIDs()
	if err != nil {
		return errors.New("round.Start get ids fail")
	}

	for k := range ids {
		msg2, ok := round.temp.kgRound2Messages[k].(*KGRound2Message)
		if !ok {
			return errors.New("round.Start get round2 msg fail")
		}

		ushare := &ec2.ShareStruct2{ID: msg2.ID, Share: msg2.Share}
		msg3, ok := round.temp.kgRound3Messages[k].(*KGRound3Message)
		if !ok {
			return errors.New("round.Start get round3 msg fail")
		}

		ps := &ec2.PolyGStruct2{PolyG: msg3.U1PolyGG}
		if !ushare.Verify2(ps) {
			fmt.Printf("========= round4 verify share fail, k = %v ==========\n", k)
			return errors.New("verify share data fail")
		}

		//verify commitment
		msg1, ok := round.temp.kgRound1Messages[k].(*KGRound1Message)
		if !ok {
			return errors.New("round.Start get round1 msg fail")
		}

		deCommit := &ec2.Commitment{C: msg1.ComC, D: msg3.ComU1GD}
		if !deCommit.Verify() {
			fmt.Printf("========= round4 verify commitment fail, k = %v ==========\n", k)
			return errors.New("verify commitment fail")
		}

		//verify bip32 commitment
		deCommitBip32 := &ec2.Commitment{C: msg1.ComCBip32, D: msg3.ComC1GD}
		if !deCommitBip32.Verify() {
			fmt.Printf("========= round4 verify commitment for bip32 fail, k = %v ==========\n", k)
			return errors.New("verify commitment fail")
		}

		_, c1G := deCommitBip32.DeCommit()
		msg21, ok := round.temp.kgRound2Messages1[k].(*KGRound2Message1)
		if !ok {
			return errors.New("round.Start get round2.1 msg fail")
		}

		cGVerifyx, cGVerifyy := secp256k1.S256().ScalarBaseMult(msg21.C1.Bytes())
		if c1G[0].Cmp(cGVerifyx) == 0 && c1G[1].Cmp(cGVerifyy) == 0 {
			//.....
		} else {
			fmt.Printf("========= round4 verify threshold for bip32 fail, k = %v ==========\n", k)
			return errors.New("verify threshold bip32 fail")
		}
	}

	var pkx *big.Int
	var pky *big.Int
	var c *big.Int
	var skU1 *big.Int

	for k := range ids {
		msg3, _ := round.temp.kgRound3Messages[k].(*KGRound3Message)
		msg1, _ := round.temp.kgRound1Messages[k].(*KGRound1Message)
		msg2, _ := round.temp.kgRound2Messages[k].(*KGRound2Message)
		ushare := &ec2.ShareStruct2{ID: msg2.ID, Share: msg2.Share}

		deCommit := &ec2.Commitment{C: msg1.ComC, D: msg3.ComU1GD}
		_, u1G := deCommit.DeCommit()
		pkx = u1G[0]
		pky = u1G[1]

		msg21, _ := round.temp.kgRound2Messages1[k].(*KGRound2Message1)

		c = msg21.C1
		skU1 = ushare.Share
		break
	}

	for k := range ids {
		if k == 0 {
			continue
		}

		msg2, _ := round.temp.kgRound2Messages[k].(*KGRound2Message)
		ushare := &ec2.ShareStruct2{ID: msg2.ID, Share: msg2.Share}
		msg3, _ := round.temp.kgRound3Messages[k].(*KGRound3Message)
		msg1, _ := round.temp.kgRound1Messages[k].(*KGRound1Message)

		deCommit := &ec2.Commitment{C: msg1.ComC, D: msg3.ComU1GD}

		_, u1G := deCommit.DeCommit()
		pkx, pky = secp256k1.S256().Add(pkx, pky, u1G[0], u1G[1])

		msg21, _ := round.temp.kgRound2Messages1[k].(*KGRound2Message1)

		c = new(big.Int).Add(c, msg21.C1)
		skU1 = new(big.Int).Add(skU1, ushare.Share)
	}

	c = new(big.Int).Mod(c, secp256k1.S256().N)
	skU1 = new(big.Int).Mod(skU1, secp256k1.S256().N)

	round.Save.SkU1 = skU1
	round.Save.Pkx = pkx
	round.Save.Pky = pky
	round.Save.C = c

	common.Info("===================keygen round4,gen ntilde start================")
	// zk of paillier key
	NtildeLength := 2048
	u1NtildeH1H2, alpha, beta, p, q,p1,p2 := ec2.GenerateNtildeH1H2(NtildeLength)
	if u1NtildeH1H2 == nil {
		return errors.New("gen ntilde h1 h2 fail")
	}

	round.temp.p1 = p1
	round.temp.p2 = p2

	////////////test
	/*fmt.Printf("===============keygen,round 4,ntilde len = %v,ntilde = %v=================\n",u1NtildeH1H2.Ntilde.BitLen(),u1NtildeH1H2.Ntilde)

	for {
	    roh := GetRandomPositiveRelativelyPrimeInt(u1NtildeH1H2.Ntilde)
	    var x *big.Int
	    if p1.Cmp(p2) >= 0 {
		x,_,_,_ = GetTheQuadraticResidueInt(roh,u1NtildeH1H2.Ntilde,p1,p2)
	    } else {
		x,_,_,_ = GetTheQuadraticResidueInt(roh,u1NtildeH1H2.Ntilde,p2,p1)
	    }

	    if x != nil {
		x2 := new(big.Int).Mul(x,x)
		x2 = new(big.Int).Mod(x2,u1NtildeH1H2.Ntilde)
		if x2.Cmp(roh) == 0 {
		    common.Info("===================keygen round4,gen ntilde end================","x",x,"roh",roh)
		    break
		}
	    }
	    
	    time.Sleep(time.Duration(1000))
	}*/
	
	/////////////////test
	
	ntildeProof1 := ec2.NewNtildeProof(u1NtildeH1H2.H1, u1NtildeH1H2.H2, alpha, p, q, u1NtildeH1H2.Ntilde)
	ntildeProof2 := ec2.NewNtildeProof(u1NtildeH1H2.H2, u1NtildeH1H2.H1, beta, p, q, u1NtildeH1H2.Ntilde)

	kg := &KGRound4Message{
		KGRoundMessage: new(KGRoundMessage),
		U1NtildeH1H2:   u1NtildeH1H2,
		NtildeProof1:   ntildeProof1,
		NtildeProof2:   ntildeProof2,
	}
	kg.SetFromID(round.dnodeid)
	kg.SetFromIndex(curIndex)

	round.Save.U1NtildeH1H2[curIndex] = u1NtildeH1H2
	round.temp.kgRound4Messages[curIndex] = kg
	round.out <- kg

	//fmt.Printf("========= round4 start success ==========\n")
	return nil
}

// CanAccept is it legal to receive this message 
func (round *round4) CanAccept(msg smpc.Message) bool {
	if _, ok := msg.(*KGRound4Message); ok {
		return msg.IsBroadcast()
	}
	return false
}

// Update  is the message received and ready for the next round? 
func (round *round4) Update() (bool, error) {
	for j, msg := range round.temp.kgRound4Messages {
		if round.ok[j] {
			continue
		}
		if msg == nil || !round.CanAccept(msg) {
			return false, nil
		}
		round.ok[j] = true
	}
	return true, nil
}

// NextRound enter next round
func (round *round4) NextRound() smpc.Round {
	round.started = false
	return &round5{round}
}
