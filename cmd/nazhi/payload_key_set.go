package main

// payload_key_set.go：写操作命令 payload 允许键的单一真相源。
//
// 存在的理由：未知键拒绝需要一份小写允许集（大小写不敏感比较，避免
// {"Telephone":...} 被误判未知），而 --help 需要按用户书写形态展示键名
// （驼峰原样，否则会教用户照抄 typeid 这类与出站 JSON 不符的全小写形态）。
// 此前这两份信息是分开的：允许集在各自文件里，--help 里则根本没有——
// 未知键文案承诺「允许键见 nazhi <cmd> --help」，而帮助文本里没有任何键名。
//
// 本文件让两者由同一份定义派生：新增或删除允许键只需改一处，
// 允许集与 --help 清单不可能脱节。TestWriteOpMode_HelpKeysCoverAllModes
// 锁定「展示键折叠小写后与允许集完全相等」这条不变式。
import (
	"sort"
	"strings"
)

// payloadKeySet 是一组 payload 顶层允许键。
//
// 键名按用户书写形态（与出站 json 键逐字一致）声明，内部派生两份视图：
// allowed 供 unknownUpdatePayloadKeys 做大小写不敏感比较，
// display 供 --help 稳定展示。
type payloadKeySet struct {
	// keys 是用户书写形态的键名，通常是驼峰。
	keys []string
	// allowed 是派生的小写集合，字段名表明它不是可写字段。
	derivedAllowed map[string]struct{}
}

// newPayloadKeySet 由用户书写形态的键名列表构造允许键集合。
// 键名去重后按小写建索引：同一键的大小写变体只登记一次。
func newPayloadKeySet(keys ...string) payloadKeySet {
	seen := make(map[string]struct{}, len(keys))
	uniq := make([]string, 0, len(keys))
	for _, k := range keys {
		lower := strings.ToLower(k)
		if _, dup := seen[lower]; dup {
			continue
		}
		seen[lower] = struct{}{}
		uniq = append(uniq, k)
	}
	return payloadKeySet{keys: uniq, derivedAllowed: seen}
}

// allowed 返回小写允许集。
func (s payloadKeySet) allowed() map[string]struct{} { return s.derivedAllowed }

// display 返回排序后的用户书写形态键名，供 --help 稳定输出。
func (s payloadKeySet) display() []string {
	out := make([]string, len(s.keys))
	copy(out, s.keys)
	sort.Strings(out)
	return out
}

// merged 合并若干组键为一份允许集（用于在既有允许集上叠加补充键，
// 避免调用方重抄一遍完整列表）。
func merged(sets ...payloadKeySet) payloadKeySet {
	var all []string
	for _, s := range sets {
		all = append(all, s.keys...)
	}
	return newPayloadKeySet(all...)
}

// helpSection 生成写入 --help 的允许键清单段落。键名逐行排列并排序，
// 使 --help 输出稳定可比对——它是脚本与 AI 代理读取契约的第一入口。
// allowNoneNote 为真时额外说明「不写即空串」，用于本就没有必填键的命令。
func helpSection(keys []string, allowNoneNote bool) string {
	if len(keys) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("payload 允许键（顶层，未列出的键会被拒绝）：")
	for _, k := range keys {
		b.WriteString("\n  ")
		b.WriteString(k)
	}
	if allowNoneNote {
		b.WriteString("\n（以上键均可省略，省略时按空值提交）")
	}
	return b.String()
}
