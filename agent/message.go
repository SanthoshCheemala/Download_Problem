package agent

// Message is a generic peer-to-peer vote/proposal. Round+Topic key the receive
// window; Topic is 0 for protocols with one channel per round (Algorithm 2).
type Message struct {
	Sender int    `json:"sender"`
	Round  int    `json:"round"`
	Topic  int    `json:"topic"`
	Value  []byte `json:"value"`
}

// SealRound closes the receive window; messages for this round or earlier
// arriving after this point are silently dropped.
func (a *Agent) SealRound(round int) {
	for {
		current := a.sealedRound.Load()
		if int32(round) <= current {
			return
		}
		if a.sealedRound.CompareAndSwap(current, int32(round)) {
			return
		}
	}
}

// ForgetRound frees a round's stored messages once it has been decided. The
// store would otherwise hold every round for the whole run, so memory grows
// with n; freeing keeps it bounded to roughly one round.
func (a *Agent) ForgetRound(round int) {
	a.messageMu.Lock()
	delete(a.messages, round)
	a.messageMu.Unlock()
}

// StoreMessage records a received message unless its round is already sealed.
func (a *Agent) StoreMessage(msg Message) {
	if int32(msg.Round) <= a.sealedRound.Load() {
		return // round already expired; drop late message
	}

	a.messageMu.Lock()
	defer a.messageMu.Unlock()

	copied := Message{
		Sender: msg.Sender,
		Round:  msg.Round,
		Topic:  msg.Topic,
		Value:  append([]byte(nil), msg.Value...),
	}

	byTopic, ok := a.messages[msg.Round]
	if !ok {
		byTopic = make(map[int][]Message)
		a.messages[msg.Round] = byTopic
	}
	byTopic[msg.Topic] = append(byTopic[msg.Topic], copied)
}

// Messages returns a copy of the messages received for (round, topic).
func (a *Agent) Messages(round, topic int) []Message {
	a.messageMu.RLock()
	defer a.messageMu.RUnlock()

	byTopic, ok := a.messages[round]
	if !ok {
		return nil
	}
	msgs := byTopic[topic]
	copied := make([]Message, len(msgs))
	for i, m := range msgs {
		copied[i] = Message{
			Sender: m.Sender,
			Round:  m.Round,
			Topic:  m.Topic,
			Value:  append([]byte(nil), m.Value...),
		}
	}
	return copied
}
