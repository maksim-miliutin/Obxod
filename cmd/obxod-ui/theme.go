package main

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

type obxodTheme struct{}

var _ fyne.Theme = obxodTheme{}

func (obxodTheme) Color(name fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return color.NRGBA{R: 0x0f, G: 0x13, B: 0x12, A: 0xff}
	case theme.ColorNameForeground:
		return color.NRGBA{R: 0xec, G: 0xe3, B: 0xcf, A: 0xff}
	case theme.ColorNamePrimary:
		return color.NRGBA{R: 0x2f, G: 0x7d, B: 0x64, A: 0xff}
	case theme.ColorNameButton, theme.ColorNameInputBackground:
		return color.NRGBA{R: 0x15, G: 0x1c, B: 0x18, A: 0xff}
	case theme.ColorNameHover:
		return color.NRGBA{R: 0x1d, G: 0x26, B: 0x21, A: 0xff}
	case theme.ColorNameSeparator:
		return color.NRGBA{R: 0x27, G: 0x30, B: 0x2b, A: 0xff}
	case theme.ColorNamePlaceHolder:
		return color.NRGBA{R: 0x9b, G: 0x97, B: 0x84, A: 0xff}
	}

	return theme.DefaultTheme().Color(name, theme.VariantDark)
}

func (obxodTheme) Font(style fyne.TextStyle) fyne.Resource {
	return theme.DefaultTheme().Font(style)
}

func (obxodTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(name)
}

func (obxodTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNamePadding:
		return 7
	case theme.SizeNameInputRadius, theme.SizeNameSelectionRadius:
		return 8
	}

	return theme.DefaultTheme().Size(name)
}
