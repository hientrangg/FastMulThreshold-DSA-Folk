package keygen

import (
	"errors"
	"fmt"
	"github.com/anyswap/Anyswap-MPCNode/smpc-lib/smpc"
)

// Start broacast DPk
func (round *round3) Start() error {
	if round.started {
		return errors.New("ed,round already started")
	}
	round.number = 3
	round.started = true
	round.resetOK()

	cur_index, err := round.GetDNodeIDIndex(round.dnodeid)
	if err != nil {
		return err
	}

	kg := &KGRound3Message{
		KGRoundMessage: new(KGRoundMessage),
		DPk:            round.temp.DPk,
	}
	kg.SetFromID(round.dnodeid)
	kg.SetFromIndex(cur_index)
	round.temp.kgRound3Messages[cur_index] = kg
	round.out <- kg

	fmt.Printf("========= round3 start success, DPk = %v, cur_index = %v ==========\n", round.temp.DPk, cur_index)
	return nil
}

// CanAccept is it legal to receive this message 
func (round *round3) CanAccept(msg smpc.Message) bool {
	if _, ok := msg.(*KGRound3Message); ok {
		return msg.IsBroadcast()
	}
	return false
}

// Update  is the message received and ready for the next round? 
func (round *round3) Update() (bool, error) {
	for j, msg := range round.temp.kgRound3Messages {
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
func (round *round3) NextRound() smpc.Round {
	round.started = false
	return &round4{round}
}
