package operation

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// filterAllowedTools returns a ToolFilterFunc hiding tools not in the
// allowlist from tools/list.
func filterAllowedTools(allowed map[string]bool) server.ToolFilterFunc {
	return func(ctx context.Context, tools []mcp.Tool) []mcp.Tool {
		filtered := make([]mcp.Tool, 0, len(tools))
		for _, tool := range tools {
			if allowed[tool.Name] {
				filtered = append(filtered, tool)
			}
		}
		return filtered
	}
}

// rejectDisallowedTools returns middleware that refuses tools/call for
// tools outside the allowlist (defense in depth alongside the list filter).
func rejectDisallowedTools(allowed map[string]bool) server.ToolHandlerMiddleware {
	return func(next server.ToolHandlerFunc) server.ToolHandlerFunc {
		return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			if !allowed[req.Params.Name] {
				return mcp.NewToolResultError(
					fmt.Sprintf("tool %q is not enabled on this server (see TOOLS_ENABLED)", req.Params.Name),
				), nil
			}
			return next(ctx, req)
		}
	}
}
