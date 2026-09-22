package main

import (
	"bytes"
	"os"

	"github.com/spf13/cobra"
)

// completionCmd 表示 nazhi completion 命令。
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
		// N-12：先缓冲完整补全脚本，成功后再一次性写 stdout——此前直接
		// GenXxxCompletion(os.Stdout)，若中途失败 Execute 返回 error，main.go
		// 再把 JSON 错误 envelope 追加到 stdout，与已写出的脚本碎片混流，
		// 脚本解析器拿到「合法脚本 + 尾部 JSON」组合出错。
		buf := new(bytes.Buffer)
		var genErr error
		switch args[0] {
		case "bash":
			genErr = cmd.Root().GenBashCompletion(buf)
		case "zsh":
			genErr = cmd.Root().GenZshCompletion(buf)
		case "fish":
			genErr = cmd.Root().GenFishCompletion(buf, true)
		case "powershell":
			genErr = cmd.Root().GenPowerShellCompletionWithDesc(buf)
		default:
			return cmd.Help()
		}
		if genErr != nil {
			return genErr
		}
		_, err := os.Stdout.Write(buf.Bytes())
		return err
	},
}
