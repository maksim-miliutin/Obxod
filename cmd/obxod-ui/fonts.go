package main

import (
	_ "embed"

	"gioui.org/font"
	"gioui.org/font/opentype"
)

//go:embed Manrope-Regular.ttf
var manropeRegular []byte

//go:embed Manrope-Bold.ttf
var manropeBold []byte

func manropeCollection() []font.FontFace {
	reg, _ := opentype.Parse(manropeRegular)
	bold, _ := opentype.Parse(manropeBold)

	return []font.FontFace{
		{Font: font.Font{}, Face: reg},
		{Font: font.Font{Weight: font.Bold}, Face: bold},
	}
}
