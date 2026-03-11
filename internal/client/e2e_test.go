package client

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	pb "github.com/isalikov/cgram-proto/gen/proto"
	"github.com/isalikov/cgram-cli/internal/store"
	"google.golang.org/protobuf/proto"
)

// Full E2E test simulating the TUI flow: register, login, add contact, send, receive.
func TestE2EFullFlow(t *testing.T) {
	ctx := context.Background()

	tmpDir, _ := os.MkdirTemp("", "cgram-test-*")
	defer os.RemoveAll(tmpDir)

	// --- Setup two clients ---
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

	user1 := fmt.Sprintf("e2e_%s", uuid.NewString()[:8])
	user2 := fmt.Sprintf("e2e_%s", uuid.NewString()[:8])

	// --- Register + login user1 ---
	resp1, _ := c1.Send(ctx, &pb.Frame{
		RequestId: uuid.NewString(),
		Payload: &pb.Frame_RegisterRequest{RegisterRequest: &pb.RegisterRequest{
			Username:          user1,
			PasswordVerifier:  []byte("pass1"),
			PublicIdentityKey: []byte("fake-key-32-bytes-padding-here!!"),
		}},
	})
	user1ID := resp1.Payload.(*pb.Frame_RegisterResponse).RegisterResponse.UserId
	t.Logf("user1: username=%s userID=%s", user1, user1ID)

	login1, _ := c1.Send(ctx, &pb.Frame{
		RequestId: uuid.NewString(),
		Payload: &pb.Frame_LoginRequest{LoginRequest: &pb.LoginRequest{
			Username: user1, AuthMessage: []byte("pass1"),
		}},
	})
	token1 := login1.Payload.(*pb.Frame_LoginResponse).LoginResponse.SessionToken

	// --- Register + login user2 ---
	resp2, _ := c2.Send(ctx, &pb.Frame{
		RequestId: uuid.NewString(),
		Payload: &pb.Frame_RegisterRequest{RegisterRequest: &pb.RegisterRequest{
			Username:          user2,
			PasswordVerifier:  []byte("pass2"),
			PublicIdentityKey: []byte("fake-key-32-bytes-padding-here!!"),
		}},
	})
	user2ID := resp2.Payload.(*pb.Frame_RegisterResponse).RegisterResponse.UserId
	t.Logf("user2: username=%s userID=%s", user2, user2ID)

	login2, _ := c2.Send(ctx, &pb.Frame{
		RequestId: uuid.NewString(),
		Payload: &pb.Frame_LoginRequest{LoginRequest: &pb.LoginRequest{
			Username: user2, AuthMessage: []byte("pass2"),
		}},
	})
	token2 := login2.Payload.(*pb.Frame_LoginResponse).LoginResponse.SessionToken

	_ = token1
	_ = token2

	// --- Open per-user stores ---
	st1, err := store.New(filepath.Join(tmpDir, user1+".db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st1.Close()

	st2, err := store.New(filepath.Join(tmpDir, user2+".db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st2.Close()

	// Save identities
	st1.SaveIdentity(&store.Identity{UserID: user1ID, Username: user1})
	st2.SaveIdentity(&store.Identity{UserID: user2ID, Username: user2})

	// --- user1 adds user2 as contact (by server userID) ---
	t.Logf("user1 adds contact: userID=%s", user2ID)
	st1.AddContact(user2ID, user2)

	// --- user2 adds user1 as contact (by server userID) ---
	t.Logf("user2 adds contact: userID=%s", user1ID)
	st2.AddContact(user1ID, user1)

	// --- user1 sends message to user2 ---
	msgID := uuid.NewString()
	now := time.Now()

	payload := &pb.MessagePayload{
		MessageId: msgID,
		SenderId:  user1ID,
		SentAt:    now.Unix(),
		Content:   &pb.MessagePayload_Text{Text: &pb.TextMessage{Body: "Hello user2!"}},
	}
	payloadBytes, _ := proto.Marshal(payload)

	t.Logf("user1 sends to user2 (recipientId=%s)", user2ID)
	ack, err := c1.Send(ctx, &pb.Frame{
		RequestId: uuid.NewString(),
		Payload: &pb.Frame_Envelope{Envelope: &pb.Envelope{
			RecipientId: user2ID,
			Ciphertext:  payloadBytes,
			Timestamp:   now.Unix(),
		}},
	})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	t.Logf("ack: %v", ack.Payload)

	// Save sent message locally (like TUI does)
	st1.SaveMessage(&store.Message{
		ID: msgID, ContactID: user2ID, Content: "Hello user2!",
		IsMine: true, Timestamp: now, Read: true,
	})

	// --- user2 receives ---
	t.Log("Waiting for message on c2...")
	select {
	case frame := <-c2.Incoming:
		env := frame.Payload.(*pb.Frame_Envelope).Envelope
		t.Logf("user2 got envelope: recipientId=%s ciphertext_len=%d", env.RecipientId, len(env.Ciphertext))

		// Simulate handleIncomingEnvelope logic
		var received pb.MessagePayload
		if err := proto.Unmarshal(env.Ciphertext, &received); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}

		senderID := received.SenderId
		t.Logf("senderID=%s body=%q", senderID, received.Content.(*pb.MessagePayload_Text).Text.Body)

		// Check that senderID matches a known contact
		contacts, _ := st2.ListContacts()
		found := false
		for _, c := range contacts {
			if c.UserID == senderID {
				found = true
				t.Logf("Sender matches contact: %s (%s)", c.Name, c.UserID)
			}
		}
		if !found {
			t.Errorf("senderID %q NOT FOUND in contacts: %v", senderID, contacts)
		}

		// Save message
		st2.SaveMessage(&store.Message{
			ID: received.MessageId, ContactID: senderID,
			Content: received.Content.(*pb.MessagePayload_Text).Text.Body,
			IsMine: false, Timestamp: time.Unix(received.SentAt, 0),
		})

		// Verify messages in store
		msgs, _ := st2.GetMessages(senderID, 100)
		t.Logf("user2 has %d messages from %s", len(msgs), senderID)
		if len(msgs) != 1 {
			t.Fatalf("expected 1 message, got %d", len(msgs))
		}
		if msgs[0].Content != "Hello user2!" {
			t.Fatalf("wrong content: %q", msgs[0].Content)
		}

		t.Log("SUCCESS")

	case <-time.After(5 * time.Second):
		t.Fatal("TIMEOUT")
	}
}
