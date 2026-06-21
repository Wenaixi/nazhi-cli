package main

import (
	"fmt"
	"time"

	"github.com/Wenaixi/nazhi-cli/pkg/client"
	"github.com/spf13/cobra"
)

// whoamiCmd 表示 nazhi whoami 命令
//
//	nazhi whoami --token <token> [--base-url <url>] [--timeout <秒>]
//
// 流程：4 步 Session 激活（HAR 对齐：/ → getMenu×2 → getMyInfo）→ 输出用户信息
var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "获取当前登录用户完整信息",
	Long: `获取用户的完整个人资料，包括姓名、性别、学号、学校、年级、班级、座号、生日等。

内部执行 HAR 对齐的 4 步 Session 激活（GET / → getMenu×2 → getMyInfo），
确保数据完整准确。`,
	Example: `  nazhi whoami --token eyJhbGciOiJIUzI1NiJ9.xxx
  nazhi whoami --token eyJhbGciOiJIUzI1NiJ9.xxx --base-url http://139.159.205.146:8280`,
	Run: func(cmd *cobra.Command, args []string) {
		token, _ := cmd.Flags().GetString("token")
		baseURL, _ := cmd.Flags().GetString("base-url")
		timeoutSec, _ := cmd.Flags().GetInt("timeout")

		// 环境变量 fallback
		if token == "" {
			token = envString("NAZHI_TOKEN", "")
		}
		if baseURL == "" {
			baseURL = envString("NAZHI_BASE_URL", "")
		}
		if timeoutSec == 15 {
			timeoutSec = envInt("NAZHI_TIMEOUT", 15)
		}

		if token == "" {
			printError(fmt.Errorf("--token 为必填（也可通过 NAZHI_TOKEN 环境变量设置）"))
			return
		}

		opts := []client.Option{client.WithTimeout(time.Duration(timeoutSec) * time.Second)}
		if baseURL != "" {
			opts = append(opts, client.WithBaseURL(baseURL))
		}
		c := client.New(opts...)

		printVerbose("正在激活 Session 并获取用户信息（4 步 HAR 对齐）...")
		info, err := c.ActivateSession(cmd.Context(), token)
		if err != nil {
			printError(fmt.Errorf("Session 激活失败: %w", err))
			return
		}
		if info == nil {
			printError(fmt.Errorf("未找到用户信息"))
			return
		}

		// 友好输出：生日使用 YMD 格式而非数组
		printJSON(map[string]any{
			"id":            info.ID,
			"name":          info.Name,
			"studentNumber": info.StudentNumber,
			"studyNumber":   info.StudyNumber,
			"idCard":        info.IDCard,
			"gender":        info.Gender,
			"genderName":    info.GenderName,
			"birthday":      info.Birthday.YMD(),
			"schoolId":      info.SchoolID,
			"schoolName":    info.SchoolName,
			"gradeName":     info.GradeName,
			"className":     info.ClassName,
			"seat":          info.Seat,
		})
	},
}

func init() {
	whoamiCmd.Flags().String("token", "", "X-Auth-Token（必填）")
	whoamiCmd.Flags().String("base-url", "", "业务 API 根地址（默认 http://139.159.205.146:8280）")
	whoamiCmd.Flags().Int("timeout", 15, "HTTP 超时（秒）")
}
