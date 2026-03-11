package mcp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestHandleToolsList(t *testing.T) {
	server := New(".")
	reply := server.handle([]byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	if !strings.Contains(string(reply), `"apply_patch"`) {
		t.Fatalf("unexpected reply: %s", string(reply))
	}
	if !strings.Contains(string(reply), `"diff"`) {
		t.Fatalf("expected diff tool in reply: %s", string(reply))
	}
	if !strings.Contains(string(reply), `"generate_patch"`) {
		t.Fatalf("expected generate_patch tool in reply: %s", string(reply))
	}
}

func TestHandlePromptsAndResourcesList(t *testing.T) {
	server := New(".")

	promptsReply := server.handle([]byte(`{"jsonrpc":"2.0","id":1,"method":"prompts/list"}`))
	if !strings.Contains(string(promptsReply), `"prompts":[]`) {
		t.Fatalf("unexpected prompts/list reply: %s", string(promptsReply))
	}

	resourcesReply := server.handle([]byte(`{"jsonrpc":"2.0","id":2,"method":"resources/list"}`))
	if !strings.Contains(string(resourcesReply), `"resources":[]`) {
		t.Fatalf("unexpected resources/list reply: %s", string(resourcesReply))
	}

	templatesReply := server.handle([]byte(`{"jsonrpc":"2.0","id":3,"method":"resources/templates/list"}`))
	if !strings.Contains(string(templatesReply), `"resourceTemplates":[]`) {
		t.Fatalf("unexpected resources/templates/list reply: %s", string(templatesReply))
	}
}

func TestHandleUnknownNotificationProducesNoReply(t *testing.T) {
	server := New(".")
	reply := server.handle([]byte(`{"jsonrpc":"2.0","method":"notifications/custom"}`))
	if len(reply) != 0 {
		t.Fatalf("expected no reply for notification, got: %s", string(reply))
	}
}

func TestReadAndWriteMessage(t *testing.T) {
	payload := []byte(`{"jsonrpc":"2.0","id":1}`)
	var buf bytes.Buffer
	if err := writeMessage(&buf, payload); err != nil {
		t.Fatalf("writeMessage() error = %v", err)
	}
	got, err := readMessage(bufio.NewReader(bytes.NewReader(buf.Bytes())))
	if err != nil {
		t.Fatalf("readMessage() error = %v", err)
	}
	var msg map[string]any
	if err := json.Unmarshal(got, &msg); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if buf.String() != string(payload)+"\n" {
		t.Fatalf("unexpected wire format: %q", buf.String())
	}
}

func TestReadMessageAcceptsEOFWithoutTrailingNewline(t *testing.T) {
	got, err := readMessage(bufio.NewReader(strings.NewReader(`{"jsonrpc":"2.0","id":1}`)))
	if err != nil {
		t.Fatalf("readMessage() error = %v", err)
	}
	if string(got) != `{"jsonrpc":"2.0","id":1}` {
		t.Fatalf("unexpected message: %q", string(got))
	}
}

func TestHandleToolsCallGeneratePatch(t *testing.T) {
	server := New(".")
	reply := server.handle([]byte(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"generate_patch","arguments":{"path":"a.txt","old_content":"","new_content":"hello\n","mode":"auto"}}}`))
	if !strings.Contains(string(reply), `"isError":false`) {
		t.Fatalf("unexpected reply: %s", string(reply))
	}
	if !strings.Contains(string(reply), `*** Add File: a.txt`) {
		t.Fatalf("expected generated patch in reply: %s", string(reply))
	}
}
