package cmd

import "github.com/spf13/cobra"

var RootCmd = &cobra.Command{
  Use: "gosh",
  Short: "gosh is a tool to use ssh with AWS ssm easily",
}

func Execute() {
  if err := RootCmd.Execute(); err != nil {
    fmt.Println(err)
    os.Exit(1)
  }
}
