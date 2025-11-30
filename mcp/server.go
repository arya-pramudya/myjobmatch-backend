package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/myjobmatch/backend/tools"
)

// Server represents an MCP (Model Context Protocol) server
// This allows the tools to be used by external AI agents
type Server struct {
	registry *tools.ToolRegistry
}

// NewServer creates a new MCP server
func NewServer(registry *tools.ToolRegistry) *Server {
	return &Server{
		registry: registry,
	}
}

// MCPRequest represents an incoming MCP tool call request
type MCPRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// MCPResponse represents an MCP response
type MCPResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *MCPError   `json:"error,omitempty"`
}

// MCPError represents an MCP error
type MCPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// ToolsListResult represents the result of tools/list
type ToolsListResult struct {
	Tools []ToolDefinition `json:"tools"`
}

// ToolDefinition represents a tool definition for MCP
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// ToolCallParams represents parameters for tools/call
type ToolCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// ToolCallResult represents the result of tools/call
type ToolCallResult struct {
	Content []ContentItem `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

// ContentItem represents a content item in MCP
type ContentItem struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// RegisterRoutes registers MCP endpoints on the given router group
func (s *Server) RegisterRoutes(router *gin.RouterGroup) {
	router.POST("/mcp", s.HandleMCP)
	router.POST("/mcp/tools/list", s.HandleToolsList)
	router.POST("/mcp/tools/call", s.HandleToolsCall)
}

// HandleMCP handles MCP JSON-RPC requests
func (s *Server) HandleMCP(c *gin.Context) {
	var req MCPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		s.sendError(c, nil, -32700, "Parse error", err.Error())
		return
	}

	switch req.Method {
	case "tools/list":
		s.handleToolsList(c, req)
	case "tools/call":
		s.handleToolsCall(c, req)
	default:
		s.sendError(c, req.ID, -32601, "Method not found", nil)
	}
}

// HandleToolsList handles GET /mcp/tools/list
func (s *Server) HandleToolsList(c *gin.Context) {
	tools := s.registry.List()

	definitions := make([]ToolDefinition, 0, len(tools))
	for _, tool := range tools {
		definitions = append(definitions, ToolDefinition{
			Name:        tool.Name(),
			Description: tool.Description(),
			InputSchema: tool.InputSchema(),
		})
	}

	c.JSON(http.StatusOK, ToolsListResult{
		Tools: definitions,
	})
}

// HandleToolsCall handles POST /mcp/tools/call
func (s *Server) HandleToolsCall(c *gin.Context) {
	var params ToolCallParams
	if err := c.ShouldBindJSON(&params); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	result, err := s.executeTool(c.Request.Context(), params.Name, params.Arguments)
	if err != nil {
		c.JSON(http.StatusOK, ToolCallResult{
			Content: []ContentItem{{Type: "text", Text: err.Error()}},
			IsError: true,
		})
		return
	}

	c.JSON(http.StatusOK, ToolCallResult{
		Content: []ContentItem{{Type: "text", Text: string(result)}},
	})
}

func (s *Server) handleToolsList(c *gin.Context, req MCPRequest) {
	tools := s.registry.List()

	definitions := make([]ToolDefinition, 0, len(tools))
	for _, tool := range tools {
		definitions = append(definitions, ToolDefinition{
			Name:        tool.Name(),
			Description: tool.Description(),
			InputSchema: tool.InputSchema(),
		})
	}

	s.sendResult(c, req.ID, ToolsListResult{
		Tools: definitions,
	})
}

func (s *Server) handleToolsCall(c *gin.Context, req MCPRequest) {
	var params ToolCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		s.sendError(c, req.ID, -32602, "Invalid params", err.Error())
		return
	}

	result, err := s.executeTool(c.Request.Context(), params.Name, params.Arguments)
	if err != nil {
		s.sendResult(c, req.ID, ToolCallResult{
			Content: []ContentItem{{Type: "text", Text: err.Error()}},
			IsError: true,
		})
		return
	}

	s.sendResult(c, req.ID, ToolCallResult{
		Content: []ContentItem{{Type: "text", Text: string(result)}},
	})
}

func (s *Server) executeTool(ctx context.Context, name string, args json.RawMessage) (json.RawMessage, error) {
	tool, ok := s.registry.Get(name)
	if !ok {
		return nil, fmt.Errorf("tool not found: %s", name)
	}

	log.Printf("[MCP] Executing tool: %s", name)
	result, err := tool.Execute(ctx, args)
	if err != nil {
		log.Printf("[MCP] Tool %s error: %v", name, err)
		return nil, err
	}

	log.Printf("[MCP] Tool %s completed", name)
	return result, nil
}

func (s *Server) sendResult(c *gin.Context, id interface{}, result interface{}) {
	c.JSON(http.StatusOK, MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	})
}

func (s *Server) sendError(c *gin.Context, id interface{}, code int, message string, data interface{}) {
	c.JSON(http.StatusOK, MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &MCPError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	})
}
