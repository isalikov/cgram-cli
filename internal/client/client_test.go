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

// Integration test: requires a running cgram-server on 127.0.0.1:8080
func TestTwoClientsMessaging(t *testing.T) {
	ctx := context.Background()

	// Create two clients
	c1 := New("127.0.0.1:8080")
	c2 := New("127.0.0.1:8080")

	// Connect both
	if err := c1.Connect(ctx); err != nil {
		t.Fatalf("c1 connect: %v", err)
	}
	defer c1.Close()

	if err := c2.Connect(ctx); err != nil {
		t.Fatalf("c2 connect: %v", err)
	}
	defer c2.Close()

	t.Log("Both clients connected")

	user1 := fmt.Sprintf("test_%s", uuid.NewString()[:8])
	user2 := fmt.Sprintf("test_%s", uuid.NewString()[:8])

	// Register user1
	resp1, err := c1.Send(ctx, &pb.Frame{
		RequestId: uuid.NewString(),
		Payload: &pb.Frame_RegisterRequest{RegisterRequest: &pb.RegisterRequest{
			Username:          user1,
			PasswordVerifier:  []byte("pass1"),
			PublicIdentityKey: []byte("fake-key-32-bytes-padding-here!!"),
		}},
	})
	if err != nil {
		t.Fatalf("register user1: %v", err)
	}
	user1ID := resp1.Payload.(*pb.Frame_RegisterResponse).RegisterResponse.UserId
	t.Logf("Registered user1=%s id=%s", user1, user1ID)

	// Register user2
	resp2, err := c2.Send(ctx, &pb.Frame{
		RequestId: uuid.NewString(),
		Payload: &pb.Frame_RegisterRequest{RegisterRequest: &pb.RegisterRequest{
			Username:          user2,
			PasswordVerifier:  []byte("pass2"),
			PublicIdentityKey: []byte("fake-key-32-bytes-padding-here!!"),
		}},
	})
	if err != nil {
		t.Fatalf("register user2: %v", err)
	}
	user2ID := resp2.Payload.(*pb.Frame_RegisterResponse).RegisterResponse.UserId
	t.Logf("Registered user2=%s id=%s", user2, user2ID)

	// Login user1
	loginResp1, err := c1.Send(ctx, &pb.Frame{
		RequestId: uuid.NewString(),
		Payload: &pb.Frame_LoginRequest{LoginRequest: &pb.LoginRequest{
			Username:    user1,
			AuthMessage: []byte("pass1"),
		}},
	})
	if err != nil {
		t.Fatalf("login user1: %v", err)
	}
	token1 := loginResp1.Payload.(*pb.Frame_LoginResponse).LoginResponse.SessionToken
	t.Logf("user1 logged in, token=%s", token1[:16]+"...")

	// Login user2
	loginResp2, err := c2.Send(ctx, &pb.Frame{
		RequestId: uuid.NewString(),
		Payload: &pb.Frame_LoginRequest{LoginRequest: &pb.LoginRequest{
			Username:    user2,
			AuthMessage: []byte("pass2"),
		}},
	})
	if err != nil {
		t.Fatalf("login user2: %v", err)
	}
	token2 := loginResp2.Payload.(*pb.Frame_LoginResponse).LoginResponse.SessionToken
	t.Logf("user2 logged in, token=%s", token2[:16]+"...")

	// Build message payload
	msgPayload := &pb.MessagePayload{
		MessageId: uuid.NewString(),
		SenderId:  user1ID,
		SentAt:    time.Now().Unix(),
		Content:   &pb.MessagePayload_Text{Text: &pb.TextMessage{Body: "Hello from user1!"}},
	}
	payloadBytes, _ := proto.Marshal(msgPayload)

	// user1 sends envelope to user2
	t.Logf("Sending message from user1 to user2 (recipientId=%s)", user2ID)
	ackResp, err := c1.Send(ctx, &pb.Frame{
		RequestId: uuid.NewString(),
		Payload: &pb.Frame_Envelope{Envelope: &pb.Envelope{
			RecipientId: user2ID,
			Ciphertext:  payloadBytes, // plaintext for test
			Timestamp:   time.Now().Unix(),
		}},
	})
	if err != nil {
		t.Fatalf("send envelope: %v", err)
	}
	t.Logf("Got ack: %v", ackResp.Payload)

	// Wait for message on c2.Incoming
	t.Log("Waiting for message on c2.Incoming...")
	select {
	case frame := <-c2.Incoming:
		env, ok := frame.Payload.(*pb.Frame_Envelope)
		if !ok {
			t.Fatalf("expected envelope, got %T", frame.Payload)
		}
		t.Logf("c2 received envelope! recipientId=%s, ciphertext len=%d",
			env.Envelope.RecipientId, len(env.Envelope.Ciphertext))

		// Decode payload
		var received pb.MessagePayload
		if err := proto.Unmarshal(env.Envelope.Ciphertext, &received); err != nil {
			t.Fatalf("unmarshal payload: %v", err)
		}
		text := received.Content.(*pb.MessagePayload_Text).Text.Body
		t.Logf("Decoded message: senderId=%s body=%q", received.SenderId, text)

		if text != "Hello from user1!" {
			t.Fatalf("expected 'Hello from user1!', got %q", text)
		}
		t.Log("SUCCESS: message delivered and decoded correctly")

	case <-time.After(5 * time.Second):
		t.Fatal("TIMEOUT: no message received on c2.Incoming after 5s")
	}
}
