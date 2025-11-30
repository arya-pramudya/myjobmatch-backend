package tools

import (
	"context"
	"encoding/json"
)

// Tool represents an MCP tool interface
type Tool interface {
	// Name returns the tool name
	Name() string

	// Description returns the tool description for the agent
	Description() string

	// InputSchema returns the JSON schema for the tool input
	InputSchema() map[string]interface{}

	// Execute runs the tool with the given input
	Execute(ctx context.Context, input json.RawMessage) (json.RawMessage, error)
}

// ToolRegistry holds all available tools
type ToolRegistry struct {
	tools map[string]Tool
}

// NewToolRegistry creates a new tool registry
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]Tool),
	}
}

// Register adds a tool to the registry
func (r *ToolRegistry) Register(tool Tool) {
	r.tools[tool.Name()] = tool
}

// Get retrieves a tool by name
func (r *ToolRegistry) Get(name string) (Tool, bool) {
	tool, ok := r.tools[name]
	return tool, ok
}

// List returns all registered tools
func (r *ToolRegistry) List() []Tool {
	tools := make([]Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, tool)
	}
	return tools
}

// GetToolDefinitions returns tool definitions in ADK format
func (r *ToolRegistry) GetToolDefinitions() []map[string]interface{} {
	definitions := make([]map[string]interface{}, 0, len(r.tools))
	for _, tool := range r.tools {
		def := map[string]interface{}{
			"name":        tool.Name(),
			"description": tool.Description(),
			"parameters":  tool.InputSchema(),
		}
		definitions = append(definitions, def)
	}
	return definitions
}

// ToolResult represents the result of a tool execution
type ToolResult struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data,omitempty"`
	Error   string          `json:"error,omitempty"`
}

// NewSuccessResult creates a successful tool result
func NewSuccessResult(data interface{}) (json.RawMessage, error) {
	result := ToolResult{Success: true}
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	result.Data = dataBytes
	return json.Marshal(result)
}

// NewErrorResult creates an error tool result
func NewErrorResult(errMsg string) (json.RawMessage, error) {
	result := ToolResult{
		Success: false,
		Error:   errMsg,
	}
	return json.Marshal(result)
}
