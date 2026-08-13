// Package fake 提供确定性模型夹具，供集成/验收测试使用；不访问网络、不读凭证。
package fake

import (
	"context"
	"encoding/json"

	"github.com/klaude/klaude/internal/model"
)

// Provider 按 Calls 下标依次吐出预设 Streams，耗尽后返回 fixture_exhausted。
type Provider struct {
	Streams [][]model.Event
	Calls   int
}

// NewCodingTurn 模拟「读文件 → 打补丁 → 收尾文本」的三轮编码回合。
func NewCodingTurn() *Provider {
	args, _ := json.Marshal(map[string]string{"path": "README.md"})
	patchArgs, _ := json.Marshal(map[string]string{"path": "README.md", "oldText": "# Klaude", "newText": "# Klaude\n\nA local-first coding agent."})
	return &Provider{Streams: [][]model.Event{
		{{Type: model.TextDelta, Text: "I will inspect the workspace first."}, {Type: model.ToolCallStart, ID: "read-1", Name: "read_file"}, {Type: model.ToolCallEnd, ID: "read-1", Arguments: args}, {Type: model.UsageUpdate, InputTokens: intPtr(42), OutputTokens: intPtr(18)}, {Type: model.ModelCompleted}},
		{{Type: model.ToolCallStart, ID: "patch-1", Name: "apply_patch"}, {Type: model.ToolCallEnd, ID: "patch-1", Arguments: patchArgs}, {Type: model.UsageUpdate, InputTokens: intPtr(60), OutputTokens: intPtr(22)}, {Type: model.ModelCompleted}},
		{{Type: model.TextDelta, Text: "The approved patch is ready for review."}, {Type: model.ModelCompleted}},
	}}
}

func (p *Provider) Stream(ctx context.Context, _ model.Request) (<-chan model.Event, error) {
	if p.Calls >= len(p.Streams) {
		return nil, &model.Error{Code: "fixture_exhausted", Message: "fake provider stream exhausted"}
	}
	events := p.Streams[p.Calls]
	p.Calls++
	out := make(chan model.Event, len(events))
	go func() {
		defer close(out)
		for _, event := range events {
			select {
			case out <- event:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out, nil
}

func intPtr(value int) *int { return &value }
