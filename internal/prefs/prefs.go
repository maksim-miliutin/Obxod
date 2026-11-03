package prefs

import "encoding/json"

type Prefs struct {
	Favourites []string       `json:"favourites"`
	Uses       map[string]int `json:"uses"`
	Lang       string         `json:"lang"`
	Method     string         `json:"method"`
	QUICOff    bool           `json:"quic_off"`
}

func Decode(data []byte) Prefs {
	p := Prefs{}
	_ = json.Unmarshal(data, &p)

	if p.Uses == nil {
		p.Uses = map[string]int{}
	}

	return p
}

func (p Prefs) Encode() []byte {
	data, err := json.Marshal(p)
	if err != nil {
		return nil
	}

	return data
}

func (p Prefs) FakeQUIC() bool {
	// The flag is the off switch, so prefs with no saved choice keep the fake on.
	return !p.QUICOff
}
