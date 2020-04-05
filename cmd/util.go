package cmd

import (
  "os"

  "github.com/fatih/color"
)

func colorf(i int) func(...interface {}) string {
  attributes := []color.Attribute{
    color.FgRed,
    color.FgGreen,
    color.FgYellow,
    color.FgBlue,
    color.FgMagenta,
    color.FgCyan,
    color.FgWhite,
    color.BgRed,
    color.BgGreen,
    color.BgYellow,
    color.BgBlue,
    color.BgMagenta,
    color.BgCyan,
    color.BgWhite,
  }

  selected := attributes[i % len(attributes)]

  return color.New(selected).SprintFunc()
}

func fatal(err error) {
  color.Red(err.Error())
  os.Exit(1)
}

