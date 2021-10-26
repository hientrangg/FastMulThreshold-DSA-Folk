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

package signing

import (
	"errors"
	"fmt"
	"github.com/anyswap/Anyswap-MPCNode/crypto/secp256k1"
	"github.com/anyswap/Anyswap-MPCNode/smpc-lib/ecdsa/keygen"
	"github.com/anyswap/Anyswap-MPCNode/smpc-lib/smpc"
	"math/big"
)

func newRound9(temp *localTempData, save *keygen.LocalDNodeSaveData, idsign smpc.SortableIDSSlice, out chan<- smpc.Message, end chan<- PrePubData, kgid string, threshold int, paillierkeylength int, predata *PrePubData, txhash *big.Int, finalizeend chan<- *big.Int) smpc.Round {
	return &round9{
		&base{temp, save, idsign, out, end, make([]bool, threshold), false, 0, kgid, threshold, paillierkeylength, predata, txhash, finalizeend}}
}

// Start broacast current node s to other nodes
func (round *round9) Start() error {
	if round.started {
		fmt.Printf("============= round9.start fail =======\n")
		return errors.New("round already started")
	}
	round.number = 9
	round.started = true
	round.resetOK()

	curIndex, err := round.GetDNodeIDIndex(round.kgid)
	if err != nil {
		return err
	}

	mk1 := new(big.Int).Mul(round.txhash, round.predata.K1)
	rSigma1 := new(big.Int).Mul(round.predata.R, round.predata.Sigma1)
	us1 := new(big.Int).Add(mk1, rSigma1)
	us1 = new(big.Int).Mod(us1, secp256k1.S256().N)

	srm := &SignRound8Message{
		SignRoundMessage: new(SignRoundMessage),
		Us1:              us1,
	}
	srm.SetFromID(round.kgid)
	srm.SetFromIndex(curIndex)

	round.temp.signRound8Messages[curIndex] = srm
	round.out <- srm

	//fmt.Printf("============= round8.start success, current node id = %v =======\n", round.kgid)
	return nil
}

// CanAccept is it legal to receive this message 
func (round *round9) CanAccept(msg smpc.Message) bool {
	if _, ok := msg.(*SignRound8Message); ok {
		return msg.IsBroadcast()
	}
	return false
}

// Update  is the message received and ready for the next round? 
func (round *round9) Update() (bool, error) {
	for j, msg := range round.temp.signRound8Messages {
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
func (round *round9) NextRound() smpc.Round {
	round.started = false
	return &round10{round}
}
