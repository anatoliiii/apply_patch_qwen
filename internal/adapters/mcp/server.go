package mcp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"apply_patch_qwen/internal/adapters/discovery"
	"apply_patch_qwen/internal/toolcontract"
)

type Server struct {
	root string
}

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *responseError  `json:"error,omitempty"`
}

type responseError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func New(root string) *Server {
	return &Server{root: root}
}

func (s *Server) Serve(r io.Reader, w io.Writer) error {
	reader := bufio.NewReader(r)
	for {
		body, err := readMessage(reader)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		reply := s.handle(body)
		if len(reply) == 0 {
			continue
		}
		if err := writeMessage(w, reply); err != nil {
			return err
		}
	}
}

func (s *Server) handle(body []byte) []byte {
	var req request
	if err := json.Unmarshal(body, &req); err != nil {
		return mustMarshal(response{
			JSONRPC: "2.0",
			Error: &responseError{
				Code:    -32700,
				Message: err.Error(),
			},
		})
	}

	switch req.Method {
	case "initialize":
		return mustMarshal(response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"protocolVersion": "2025-03-26",
				"serverInfo": map[string]any{
					"name":    toolcontract.ServerName,
					"version": toolcontract.ServerVersion,
				},
				"capabilities": map[string]any{
					"tools": map[string]any{},
				},
			},
		})
	case "notifications/initialized":
		return nil
	case "notifications/cancelled":
		return nil
	case "ping":
		return mustMarshal(response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  map[string]any{},
		})
	case "prompts/list":
		return mustMarshal(response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"prompts": []map[string]any{},
			},
		})
	case "resources/list":
		return mustMarshal(response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"resources": []map[string]any{},
			},
		})
	case "resources/templates/list":
		return mustMarshal(response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"resourceTemplates": []map[string]any{},
			},
		})
	case "tools/list":
		entries := toolcontract.DiscoveryDocument()
		tools := make([]map[string]any, 0, len(entries))
		for _, entry := range entries {
			tools = append(tools, map[string]any{
				"name":        entry.Name,
				"description": entry.Description,
				"inputSchema": entry.ParametersJSONSchema,
			})
		}
		return mustMarshal(response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"tools": tools,
			},
		})
	case "tools/call":
		var params struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return mustMarshal(response{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error: &responseError{
					Code:    -32602,
					Message: err.Error(),
				},
			})
		}
		var out bytes.Buffer
		if err := discovery.Execute(s.root, params.Name, bytes.NewReader(params.Arguments), &out); err != nil {
			return mustMarshal(response{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: map[string]any{
					"content": []map[string]any{
						{
							"type": "text",
							"text": toolcontract.Failure("Patch rejected.", toolcontract.Diagnostic{
								Kind:    "call_error",
								Message: err.Error(),
							}).Summary,
						},
					},
					"isError": true,
				},
			})
		}
		var resp toolcontract.ApplyPatchResponse
		if err := json.Unmarshal(out.Bytes(), &resp); err != nil {
			return mustMarshal(response{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error: &responseError{
					Code:    -32603,
					Message: err.Error(),
				},
			})
		}
		text, _ := json.Marshal(resp)
		return mustMarshal(response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: map[string]any{
				"content": []map[string]any{
					{
						"type": "text",
						"text": string(text),
					},
				},
				"isError": !resp.OK,
			},
		})
	default:
		if strings.HasPrefix(req.Method, "notifications/") {
			return nil
		}
		return mustMarshal(response{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &responseError{
				Code:    -32601,
				Message: fmt.Sprintf("unsupported method %q", req.Method),
			},
		})
	}
}


func readMessage(r *bufio.Reader) ([]byte, error) {
	for {
		line, err := r.ReadBytes('\n')
		if err != nil {
			if err == io.EOF && len(line) > 0 {
				trimmed := bytes.TrimSpace(line)
				if len(trimmed) > 0 {
					return append([]byte(nil), trimmed...), nil
				}
			}
			return nil, err
		}
		trimmed := bytes.TrimSpace(line)
		if len(trimmed) == 0 {
			continue
		}
		return append([]byte(nil), trimmed...), nil
	}
}

func writeMessage(w io.Writer, payload []byte) error {
	if _, err := w.Write(payload); err != nil {
		return err
	}
	_, err := w.Write([]byte("\n"))
	return err
}

func mustMarshal(v any) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return data
}
