package operation

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestFilterAllowedTools(t *testing.T) {
	allowed := map[string]bool{"keep_me": true}
	tools := []mcp.Tool{{Name: "keep_me"}, {Name: "drop_me"}}
	got := filterAllowedTools(allowed)(context.Background(), tools)
	if len(got) != 1 || got[0].Name != "keep_me" {
		t.Fatalf("filterAllowedTools = %v, want only keep_me", got)
	}
}

func TestRejectDisallowedTools(t *testing.T) {
	allowed := map[string]bool{"keep_me": true}
	next := func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultText("ran"), nil
	}
	mw := rejectDisallowedTools(allowed)(next)

	req := mcp.CallToolRequest{}
	req.Params.Name = "drop_me"
	res, err := mw(context.Background(), req)
	if err != nil {
		t.Fatalf("middleware returned transport error: %v", err)
	}
	if !res.IsError {
		t.Fatal("disallowed tool call should return a tool error result")
	}

	req.Params.Name = "keep_me"
	res, err = mw(context.Background(), req)
	if err != nil || res.IsError {
		t.Fatalf("allowed tool should pass through, got res=%v err=%v", res, err)
	}
}
