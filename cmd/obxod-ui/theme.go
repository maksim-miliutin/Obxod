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
		return color.NRGBA{R: 0x0f, G: 0x16, B: 0x14, A: 0xff}
	case theme.ColorNameForeground:
		return color.NRGBA{R: 0xec, G: 0xe3, B: 0xcf, A: 0xff}
	case theme.ColorNamePrimary:
		return color.NRGBA{R: 0x2e, G: 0x9e, B: 0x73, A: 0xff}
	case theme.ColorNameButton, theme.ColorNameInputBackground:
		return color.NRGBA{R: 0x1b, G: 0x24, B: 0x20, A: 0xff}
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
		return 6
	case theme.SizeNameInputRadius, theme.SizeNameSelectionRadius:
		return 4
	}

	return theme.DefaultTheme().Size(name)
}
