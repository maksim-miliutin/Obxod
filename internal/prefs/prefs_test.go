package prefs

import "testing"

func TestDecodeOfNothingIsUsable(t *testing.T) {
	p := Decode(nil)

	if p.Uses == nil {
		t.Error("Decode(nil) left the uses map nil")
	}

	if p.Method != "" {
		t.Errorf("Decode(nil) invented a method: %q", p.Method)
	}
}

func TestDecodeFillsTheUsesMap(t *testing.T) {
	if Decode([]byte(`{"lang":"en"}`)).Uses == nil {
		t.Error("a stored prefs without uses decoded to a nil map")
	}
}

func TestFakeQUICOnWithNoSavedChoice(t *testing.T) {
	if !Decode(nil).FakeQUIC() {
		t.Error("the quic fake reads as off when nothing was saved")
	}
}

func TestFakeQUICOffOnceTurnedOff(t *testing.T) {
	if Decode([]byte(`{"quic_off":true}`)).FakeQUIC() {
		t.Error("the quic fake stayed on after it was turned off")
	}
}

func TestEncodeRoundTripsMethodAndQUIC(t *testing.T) {
	want := Prefs{Method: "ya_decoy", QUICOff: true, Lang: "ru"}
	got := Decode(want.Encode())

	if got.Method != want.Method {
		t.Errorf("method: got %q, want %q", got.Method, want.Method)
	}

	if got.FakeQUIC() != want.FakeQUIC() {
		t.Errorf("quic: got %v, want %v", got.FakeQUIC(), want.FakeQUIC())
	}
}
