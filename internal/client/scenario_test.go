package client

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	pb "github.com/isalikov/cgram-proto/gen/proto"
	"google.golang.org/protobuf/proto"
)

// Exact user scenario: a1 sends to a2, wait 20 seconds to check connection stays alive.
func TestScenarioA1A2(t *testing.T) {
	ctx := context.Background()

	c1 := New("127.0.0.1:8080")
	c2 := New("127.0.0.1:8080")

	if err := c1.Connect(ctx); err != nil {
		t.Fatalf("c1 connect: %v", err)
	}
	defer c1.Close()
	if err := c2.Connect(ctx); err != nil {
		t.Fatalf("c2 connect: %v", err)
	}
	defer c2.Close()

	t.Log("Both connected")

	// Use unique names to avoid conflicts with existing users
	a1 := fmt.Sprintf("a1_%s", uuid.NewString()[:6])
	a2 := fmt.Sprintf("a2_%s", uuid.NewString()[:6])

	// Register a1
	r1, err := c1.Send(ctx, &pb.Frame{
		RequestId: uuid.NewString(),
		Payload: &pb.Frame_RegisterRequest{RegisterRequest: &pb.RegisterRequest{
			Username:          a1,
			PasswordVerifier:  []byte("123123"),
			PublicIdentityKey: []byte("fake-key-32-bytes-padding-here!!"),
		}},
	})
	if err != nil {
		t.Fatalf("register a1: %v", err)
	}
	a1ID := r1.Payload.(*pb.Frame_RegisterResponse).RegisterResponse.UserId
	t.Logf("a1 registered: username=%s id=%s", a1, a1ID)

	// Register a2
	r2, err := c2.Send(ctx, &pb.Frame{
		RequestId: uuid.NewString(),
		Payload: &pb.Frame_RegisterRequest{RegisterRequest: &pb.RegisterRequest{
			Username:          a2,
			PasswordVerifier:  []byte("123123"),
			PublicIdentityKey: []byte("fake-key-32-bytes-padding-here!!"),
		}},
	})
	if err != nil {
		t.Fatalf("register a2: %v", err)
	}
	a2ID := r2.Payload.(*pb.Frame_RegisterResponse).RegisterResponse.UserId
	t.Logf("a2 registered: username=%s id=%s", a2, a2ID)

	// Login a1
	l1, err := c1.Send(ctx, &pb.Frame{
		RequestId: uuid.NewString(),
		Payload:   &pb.Frame_LoginRequest{LoginRequest: &pb.LoginRequest{Username: a1, AuthMessage: []byte("123123")}},
	})
	if err != nil {
		t.Fatalf("login a1: %v", err)
	}
	t.Logf("a1 logged in, token=%s...", l1.Payload.(*pb.Frame_LoginResponse).LoginResponse.SessionToken[:12])

	// Login a2
	l2, err := c2.Send(ctx, &pb.Frame{
		RequestId: uuid.NewString(),
		Payload:   &pb.Frame_LoginRequest{LoginRequest: &pb.LoginRequest{Username: a2, AuthMessage: []byte("123123")}},
	})
	if err != nil {
		t.Fatalf("login a2: %v", err)
	}
	t.Logf("a2 logged in, token=%s...", l2.Payload.(*pb.Frame_LoginResponse).LoginResponse.SessionToken[:12])

	// Check connections are still alive
	t.Logf("c1.Connected=%v c2.Connected=%v", c1.Connected(), c2.Connected())

	// Wait 5 seconds to simulate real usage gap
	t.Log("Waiting 5 seconds to simulate user adding contacts...")
	time.Sleep(5 * time.Second)

	t.Logf("After 5s: c1.Connected=%v c2.Connected=%v", c1.Connected(), c2.Connected())
	if !c1.Connected() {
		t.Fatal("c1 disconnected after 5 seconds! Server timeout issue.")
	}
	if !c2.Connected() {
		t.Fatal("c2 disconnected after 5 seconds! Server timeout issue.")
	}

	// Wait 15 more seconds to test the old 15s ReadTimeout
	t.Log("Waiting 15 more seconds to test timeout fix...")
	time.Sleep(15 * time.Second)

	t.Logf("After 20s total: c1.Connected=%v c2.Connected=%v", c1.Connected(), c2.Connected())
	if !c1.Connected() {
		t.Fatal("c1 disconnected after 20 seconds! Server ReadTimeout still active.")
	}
	if !c2.Connected() {
		t.Fatal("c2 disconnected after 20 seconds! Server ReadTimeout still active.")
	}

	// Now send message a1 -> a2
	payload := &pb.MessagePayload{
		MessageId: uuid.NewString(),
		SenderId:  a1ID,
		SentAt:    time.Now().Unix(),
		Content:   &pb.MessagePayload_Text{Text: &pb.TextMessage{Body: "Hello from a1!"}},
	}
	payloadBytes, _ := proto.Marshal(payload)

	t.Logf("a1 sending to a2 (recipientId=%s)", a2ID)
	ack, err := c1.Send(ctx, &pb.Frame{
		RequestId: uuid.NewString(),
		Payload: &pb.Frame_Envelope{Envelope: &pb.Envelope{
			RecipientId: a2ID,
			Ciphertext:  payloadBytes,
			Timestamp:   time.Now().Unix(),
		}},
	})
	if err != nil {
		t.Fatalf("send failed: %v", err)
	}
	t.Logf("ack received: %v", ack.Payload)

	// Check a2 receives
	t.Log("Waiting for message on a2...")
	select {
	case frame := <-c2.Incoming:
		env := frame.Payload.(*pb.Frame_Envelope).Envelope
		var recv pb.MessagePayload
		proto.Unmarshal(env.Ciphertext, &recv)
		t.Logf("a2 received: sender=%s body=%q", recv.SenderId, recv.Content.(*pb.MessagePayload_Text).Text.Body)
		t.Log("SUCCESS: a1 -> a2 works!")
	case <-time.After(5 * time.Second):
		t.Fatal("TIMEOUT: a2 never received the message")
	}

	// Now test a2 -> a1
	payload2 := &pb.MessagePayload{
		MessageId: uuid.NewString(),
		SenderId:  a2ID,
		SentAt:    time.Now().Unix(),
		Content:   &pb.MessagePayload_Text{Text: &pb.TextMessage{Body: "Reply from a2!"}},
	}
	payloadBytes2, _ := proto.Marshal(payload2)

	t.Logf("a2 sending to a1 (recipientId=%s)", a1ID)
	ack2, err := c2.Send(ctx, &pb.Frame{
		RequestId: uuid.NewString(),
		Payload: &pb.Frame_Envelope{Envelope: &pb.Envelope{
			RecipientId: a1ID,
			Ciphertext:  payloadBytes2,
			Timestamp:   time.Now().Unix(),
		}},
	})
	if err != nil {
		t.Fatalf("send failed: %v", err)
	}
	t.Logf("ack received: %v", ack2.Payload)

	select {
	case frame := <-c1.Incoming:
		env := frame.Payload.(*pb.Frame_Envelope).Envelope
		var recv pb.MessagePayload
		proto.Unmarshal(env.Ciphertext, &recv)
		t.Logf("a1 received: sender=%s body=%q", recv.SenderId, recv.Content.(*pb.MessagePayload_Text).Text.Body)
		t.Log("SUCCESS: a2 -> a1 works!")
	case <-time.After(5 * time.Second):
		t.Fatal("TIMEOUT: a1 never received the message")
	}
}
