package notify

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/evan/football-picks/internal/models"
	"github.com/google/uuid"
	"github.com/resend/resend-go/v2"
)

func TestIntroPlain_StripsBold(t *testing.T) {
	got := introPlain("Hello **world**, this is **bold** text.")
	want := "Hello world, this is bold text."
	if got != want {
		t.Errorf("introPlain: got %q, want %q", got, want)
	}
}

func TestIntroPlain_NoMarkers(t *testing.T) {
	got := introPlain("Plain text only.")
	want := "Plain text only."
	if got != want {
		t.Errorf("introPlain: got %q, want %q", got, want)
	}
}

func TestIntroPlain_MultipleMarkers(t *testing.T) {
	got := introPlain("**A** and **B** and **C**")
	want := "A and B and C"
	if got != want {
		t.Errorf("introPlain: got %q, want %q", got, want)
	}
}

func TestIntroHTML_ConvertsBold(t *testing.T) {
	got := introHTML("Hello **world**!")
	want := "Hello <strong>world</strong>!"
	if got != want {
		t.Errorf("introHTML bold: got %q, want %q", got, want)
	}
}

func TestIntroHTML_EscapesHTMLEntities(t *testing.T) {
	got := introHTML("<script>alert(1)</script>")
	want := "&lt;script&gt;alert(1)&lt;/script&gt;"
	if got != want {
		t.Errorf("introHTML escape: got %q, want %q", got, want)
	}
}

func TestIntroHTML_ConvertsNewlines(t *testing.T) {
	got := introHTML("line1\nline2")
	want := "line1<br>line2"
	if got != want {
		t.Errorf("introHTML newline: got %q, want %q", got, want)
	}
}

func TestIntroHTML_AllTransforms(t *testing.T) {
	got := introHTML("Hey **player**!\nGood <luck>")
	want := "Hey <strong>player</strong>!<br>Good &lt;luck&gt;"
	if got != want {
		t.Errorf("introHTML combined: got %q, want %q", got, want)
	}
}

func TestNoopMailer_ReturnsNil(t *testing.T) {
	m := NoopMailer{}
	a := &models.Announcement{ID: uuid.New(), Intro: "test"}
	u := models.User{DisplayName: "Test", Email: "test@example.com"}
	err := m.SendAnnouncement(context.Background(), u, a, 1)
	if err != nil {
		t.Errorf("NoopMailer: got error %v, want nil", err)
	}
}

func TestNewResendMailer(t *testing.T) {
	m := NewResendMailer("test-key", "from@example.com", "https://app.example.com")
	if m == nil {
		t.Fatal("NewResendMailer returned nil")
	}
}

// mockTransport stubs Resend API responses in tests.
type mockTransport struct {
	statusCode int
	body       string
	err        error
}

func (m *mockTransport) RoundTrip(_ *http.Request) (*http.Response, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &http.Response{
		StatusCode: m.statusCode,
		Body:       io.NopCloser(strings.NewReader(m.body)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}, nil
}

func newTestResendMailer(transport http.RoundTripper) *ResendMailer {
	httpClient := &http.Client{Transport: transport}
	return &ResendMailer{
		client:    resend.NewCustomClient(httpClient, "test-key"),
		fromEmail: "from@example.com",
		fromName:  "Football Picks",
		appURL:    "https://example.com",
	}
}

func TestResendMailer_SendAnnouncement_Success(t *testing.T) {
	m := newTestResendMailer(&mockTransport{statusCode: 200, body: `{"id":"abc123"}`})
	a := &models.Announcement{ID: uuid.New(), Intro: "Hello **world**!"}
	err := m.SendAnnouncement(context.Background(), models.User{DisplayName: "Alice", Email: "alice@test.com"}, a, 7)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestResendMailer_SendAnnouncement_ErrorStatus(t *testing.T) {
	m := newTestResendMailer(&mockTransport{statusCode: 500, body: `{"message":"internal server error"}`})
	a := &models.Announcement{ID: uuid.New(), Intro: "test"}
	err := m.SendAnnouncement(context.Background(), models.User{DisplayName: "Bob", Email: "bob@test.com"}, a, 1)
	if err == nil {
		t.Error("expected error for 500 status, got nil")
	}
}

func TestResendMailer_SendAnnouncement_NetworkError(t *testing.T) {
	m := newTestResendMailer(&mockTransport{err: fmt.Errorf("connection refused")})
	a := &models.Announcement{ID: uuid.New(), Intro: "test"}
	err := m.SendAnnouncement(context.Background(), models.User{DisplayName: "Carol", Email: "carol@test.com"}, a, 3)
	if err == nil {
		t.Error("expected error for network failure, got nil")
	}
}
