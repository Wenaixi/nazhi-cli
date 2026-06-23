package main

import (
	"os"

	"github.com/spf13/cobra"
)

// completionCmd 表示 nazhi completion 命令
// cobra 原生支持 shell 自动补全，只需注册即可
var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "生成 shell 自动补全脚本",
	Long: `生成指定 shell 的自动补全脚本。

执行后按提示 source 到 shell 配置即可启用自动补全：

  # Bash
  source <(nazhi completion bash)

  # Zsh（先加载补全系统）
  echo "autoload -U compinit; compinit" >> ~/.zshrc
  echo "source <(nazhi completion zsh)" >> ~/.zshrc

  # fish
  nazhi completion fish | source

  # PowerShell
  nazhi completion powershell | Out-String | Invoke-Expression`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			return cmd.Root().GenBashCompletion(os.Stdout)
		case "zsh":
			return cmd.Root().GenZshCompletion(os.Stdout)
		case "fish":
			return cmd.Root().GenFishCompletion(os.Stdout, true)
		case "powershell":
			return cmd.Root().GenPowerShellCompletionWithDesc(os.Stdout)
		default:
			return cmd.Help()
		}
	},
}
