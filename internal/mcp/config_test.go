package mcp

import "testing"

func TestNormalizeDefinitionRequiresTransportSpecificFields(t *testing.T) {
	if _, err := NormalizeDefinition(Definition{Name: "Remote", Transport: TransportStreamableHTTP}); err == nil {
		t.Fatal("expected streamable HTTP URL validation error")
	}
	if _, err := NormalizeDefinition(Definition{Name: "Local", Transport: TransportStdio}); err == nil {
		t.Fatal("expected stdio command validation error")
	}
}

func TestNormalizeDefinitionTrimsAndDefaultsID(t *testing.T) {
	got, err := NormalizeDefinition(Definition{Name: "  Local Server ", Transport: TransportStdio, Command: "  node  ", Args: []string{" server.js "}})
	if err != nil {
		t.Fatal(err)
	}
	if got.ID == "" || got.Name != "Local Server" || got.Command != "node" || got.Args[0] != "server.js" {
		t.Fatalf("normalized definition = %+v", got)
	}
}
