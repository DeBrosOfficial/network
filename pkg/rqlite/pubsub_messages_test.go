package rqlite

import (
	"encoding/json"
	"testing"
	"time"
)

func TestMarshalUnmarshalMetadataMessage_DatabaseCreateRequest(t *testing.T) {
	payload := DatabaseCreateRequest{
		DatabaseName:      "testdb",
		RequesterNodeID:   "node123",
		ReplicationFactor: 3,
	}

	data, err := MarshalMetadataMessage(MsgDatabaseCreateRequest, "node123", payload)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var decoded DatabaseCreateRequest
	msg, err := UnmarshalMetadataMessage(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if msg.Type != MsgDatabaseCreateRequest {
		t.Errorf("Expected type %s, got %s", MsgDatabaseCreateRequest, msg.Type)
	}

	if decoded.DatabaseName != payload.DatabaseName {
		t.Errorf("Expected database name %s, got %s", payload.DatabaseName, decoded.DatabaseName)
	}

	if decoded.ReplicationFactor != payload.ReplicationFactor {
		t.Errorf("Expected replication factor %d, got %d", payload.ReplicationFactor, decoded.ReplicationFactor)
	}
}

func TestMarshalUnmarshalMetadataMessage_DatabaseCreateResponse(t *testing.T) {
	payload := DatabaseCreateResponse{
		DatabaseName: "testdb",
		NodeID:       "node456",
		AvailablePorts: PortPair{
			HTTPPort: 5001,
			RaftPort: 7001,
		},
	}

	data, err := MarshalMetadataMessage(MsgDatabaseCreateResponse, "node456", payload)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var decoded DatabaseCreateResponse
	msg, err := UnmarshalMetadataMessage(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if msg.Type != MsgDatabaseCreateResponse {
		t.Errorf("Expected type %s, got %s", MsgDatabaseCreateResponse, msg.Type)
	}

	if decoded.AvailablePorts.HTTPPort != 5001 {
		t.Errorf("Expected HTTP port 5001, got %d", decoded.AvailablePorts.HTTPPort)
	}
}

func TestMarshalUnmarshalMetadataMessage_DatabaseCreateConfirm(t *testing.T) {
	payload := DatabaseCreateConfirm{
		DatabaseName: "testdb",
		SelectedNodes: []NodeAssignment{
			{NodeID: "node1", HTTPPort: 5001, RaftPort: 7001, Role: "leader"},
			{NodeID: "node2", HTTPPort: 5002, RaftPort: 7002, Role: "follower"},
			{NodeID: "node3", HTTPPort: 5003, RaftPort: 7003, Role: "follower"},
		},
		CoordinatorNodeID: "node1",
	}

	data, err := MarshalMetadataMessage(MsgDatabaseCreateConfirm, "node1", payload)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var decoded DatabaseCreateConfirm
	msg, err := UnmarshalMetadataMessage(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if len(decoded.SelectedNodes) != 3 {
		t.Errorf("Expected 3 nodes, got %d", len(decoded.SelectedNodes))
	}

	if decoded.SelectedNodes[0].Role != "leader" {
		t.Errorf("Expected first node to be leader, got %s", decoded.SelectedNodes[0].Role)
	}
}

func TestMarshalUnmarshalMetadataMessage_DatabaseStatusUpdate(t *testing.T) {
	payload := DatabaseStatusUpdate{
		DatabaseName: "testdb",
		NodeID:       "node123",
		Status:       StatusActive,
		HTTPPort:     5001,
	}

	data, err := MarshalMetadataMessage(MsgDatabaseStatusUpdate, "node123", payload)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var decoded DatabaseStatusUpdate
	msg, err := UnmarshalMetadataMessage(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if decoded.Status != StatusActive {
		t.Errorf("Expected status active, got %s", decoded.Status)
	}
}

func TestMarshalUnmarshalMetadataMessage_NodeCapacityAnnouncement(t *testing.T) {
	payload := NodeCapacityAnnouncement{
		NodeID:           "node123",
		MaxDatabases:     100,
		CurrentDatabases: 5,
		PortRangeHTTP:    PortRange{Start: 5001, End: 5999},
		PortRangeRaft:    PortRange{Start: 7001, End: 7999},
	}

	data, err := MarshalMetadataMessage(MsgNodeCapacityAnnouncement, "node123", payload)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var decoded NodeCapacityAnnouncement
	msg, err := UnmarshalMetadataMessage(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if decoded.MaxDatabases != 100 {
		t.Errorf("Expected max databases 100, got %d", decoded.MaxDatabases)
	}

	if decoded.CurrentDatabases != 5 {
		t.Errorf("Expected current databases 5, got %d", decoded.CurrentDatabases)
	}
}

func TestMarshalUnmarshalMetadataMessage_DatabaseIdleNotification(t *testing.T) {
	now := time.Now()
	payload := DatabaseIdleNotification{
		DatabaseName: "testdb",
		NodeID:       "node123",
		LastActivity: now,
	}

	data, err := MarshalMetadataMessage(MsgDatabaseIdleNotification, "node123", payload)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var decoded DatabaseIdleNotification
	msg, err := UnmarshalMetadataMessage(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	// Time comparison with some tolerance
	if decoded.LastActivity.Unix() != now.Unix() {
		t.Errorf("Expected last activity %v, got %v", now, decoded.LastActivity)
	}
}

func TestMarshalUnmarshalMetadataMessage_NodeReplacementNeeded(t *testing.T) {
	payload := NodeReplacementNeeded{
		DatabaseName:      "testdb",
		FailedNodeID:      "node3",
		CurrentNodes:      []string{"node1", "node2"},
		ReplicationFactor: 3,
	}

	data, err := MarshalMetadataMessage(MsgNodeReplacementNeeded, "node1", payload)
	if err != nil {
		t.Fatalf("Failed to marshal: %v", err)
	}

	var decoded NodeReplacementNeeded
	msg, err := UnmarshalMetadataMessage(data, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if decoded.FailedNodeID != "node3" {
		t.Errorf("Expected failed node node3, got %s", decoded.FailedNodeID)
	}

	if len(decoded.CurrentNodes) != 2 {
		t.Errorf("Expected 2 current nodes, got %d", len(decoded.CurrentNodes))
	}
}

func TestMetadataMessage_UnmarshalPayload(t *testing.T) {
	payload := DatabaseCreateRequest{
		DatabaseName:      "testdb",
		RequesterNodeID:   "node123",
		ReplicationFactor: 3,
	}

	payloadBytes, _ := json.Marshal(payload)
	msg := &MetadataMessage{
		Type:      MsgDatabaseCreateRequest,
		Timestamp: time.Now(),
		NodeID:    "node123",
		Payload:   payloadBytes,
	}

	var decoded DatabaseCreateRequest
	err := msg.UnmarshalPayload(&decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal payload: %v", err)
	}

	if decoded.DatabaseName != "testdb" {
		t.Errorf("Expected database name testdb, got %s", decoded.DatabaseName)
	}
}

func TestMetadataMessage_EmptyPayload(t *testing.T) {
	msg := &MetadataMessage{
		Type:      MsgNodeHealthPing,
		Timestamp: time.Now(),
		NodeID:    "node123",
		Payload:   nil,
	}

	var decoded struct{}
	err := msg.UnmarshalPayload(&decoded)
	if err != nil {
		t.Fatalf("Expected no error for empty payload, got %v", err)
	}
}
