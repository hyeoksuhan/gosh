package cmd

import (
  "github.com/spf13/cobra"
)

var RootCmd = &cobra.Command{
  Use: "gosh",
  Short: "gosh is a tool to use SSM easily",
}

func Execute() {
  RootCmd.Execute()
}
