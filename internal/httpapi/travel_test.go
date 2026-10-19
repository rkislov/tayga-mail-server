package httpapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/tayga/tms/internal/calutil"
	"github.com/tayga/tms/internal/storage"
)

const testTicketText = "Flight:SU1234\nFrom:Moscow\nTo:Delhi\nDeparture:2026-10-25 23:30 UTC+3\nArrival:2026-10-26 07:00 +05:30\nPassenger:TEST PERSON\nBooking:TEST123\nSeat:12A"

func insertTicketMail(t *testing.T, f *fileFixture, body, kind, folder string) *storage.Message {
	t.Helper()
	ctx := context.Background()
	u, err := f.s.store.GetUserByID(ctx, f.users[0])
	if err != nil {
		t.Fatal(err)
	}
	mb, err := f.s.store.EnsureMailbox(ctx, u.ID, folder, f.s.ms.UserRoot(u.Email))
	if err != nil {
		t.Fatal(err)
	}
	raw := []byte("From: travel@example.com\r\nTo: owner@example.com\r\nSubject: Itinerary\r\nMIME-Version: 1.0\r\nContent-Type: " + kind + "; charset=utf-8\r\n\r\n" + body)
	path, size, err := f.s.ms.Deliver(u.Email, folder, raw)
	if err != nil {
		t.Fatal(err)
	}
	m, err := f.s.store.InsertMessage(ctx, &storage.Message{MailboxID: mb.ID, Size: size, FilePath: path})
	if err != nil {
		t.Fatal(err)
	}
	return m
}
func travelList(t *testing.T, f *fileFixture) []storage.TravelCandidate {
	t.Helper()
	var result struct{ Candidates []storage.TravelCandidate }
	if err := json.Unmarshal(f.call(0, "GET", "/api/v1/travel", "", 200).Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return result.Candidates
}
func TestTravelHTTPWorkerAndPrivateEvent(t *testing.T) {
	f := newFileFixture(t)
	ctx := context.Background()
	f.call(-1, "GET", "/api/v1/travel", "", 401)
	f.call(0, "PUT", "/api/v1/travel/preferences", `{"mode":"invalid"}`, 400)
	f.call(0, "PUT", "/api/v1/travel/preferences", `{"mode":"auto"}`, 200)
	msg := insertTicketMail(t, f, "<html><body><p>"+strings.ReplaceAll(testTicketText, "\n", "</p><p>")+"</p></body></html>", "text/html", "INBOX")
	if err := f.s.drainTravelJobs(ctx); err != nil {
		t.Fatal(err)
	}
	items := travelList(t, f)
	if len(items) != 1 || items[0].State != "saved" {
		t.Fatal(items)
	}
	item := items[0]
	o, err := f.s.store.GetCalendarObjectByID(ctx, item.EventID)
	if err != nil {
		t.Fatal(err)
	}
	cal, err := f.s.store.GetCalendarByID(ctx, f.users[0], o.CalendarID)
	if err != nil || cal.Name != "personal" {
		t.Fatal(cal, err)
	}
	parsed, err := calutil.ParseICS(o.Data)
	if err != nil || parsed.Start.UTC().Format("15:04") != "20:30" {
		t.Fatal(parsed, err)
	}
	if len(parsed.Attendees) != 0 || parsed.Method != "" {
		t.Fatal("unexpected scheduling data")
	}
	// Any user-facing operation is scoped to the mailbox owner.
	f.call(1, "GET", "/api/v1/travel/"+item.ID, "", 404)
	payload := `{"start":"2026-10-25T20:30:00Z","end":"2026-10-26T01:30:00Z"}`
	f.call(1, "POST", "/api/v1/travel/"+item.ID+"/event", payload, 404)
	f.call(1, "POST", "/api/v1/mail/messages/"+msg.ID+"/travel", "", 403)
	f.call(0, "POST", "/api/v1/mail/messages/"+msg.ID+"/travel", "", 200)
	insertTicketMail(t, f, testTicketText, "text/plain", "INBOX")
	if err = f.s.drainTravelJobs(ctx); err != nil {
		t.Fatal(err)
	}
	if len(travelList(t, f)) != 1 {
		t.Fatal("HTML/plain replay duplicated trip")
	}
	if count, err := f.s.store.CountOutbound(ctx); err != nil || count != 0 {
		t.Fatal("travel event sent mail", count, err)
	}
}
func TestTravelManualReviewAndModes(t *testing.T) {
	f := newFileFixture(t)
	ctx := context.Background()
	msg := insertTicketMail(t, f, testTicketText, "text/plain", "INBOX")
	if err := f.s.drainTravelJobs(ctx); err != nil {
		t.Fatal(err)
	}
	if len(travelList(t, f)) != 0 {
		t.Fatal("disabled auto-processing")
	}
	f.call(0, "POST", "/api/v1/mail/messages/"+msg.ID+"/travel", "", 200)
	items := travelList(t, f)
	if len(items) != 1 || items[0].State != "review" {
		t.Fatal(items)
	}
	path := "/api/v1/travel/" + items[0].ID + "/event"
	f.call(0, "POST", path, `{"start":"2026-10-25T20:30","end":"2026-10-26T01:30"}`, 400)
	f.call(0, "POST", path, `{"start":"2026-10-26T20:30:00Z","end":"2026-10-25T01:30:00Z"}`, 400)
	f.call(0, "POST", path, `{"start":"2026-10-25T20:30:00Z","end":"2026-10-26T01:30:00Z"}`, 200)
	f.call(0, "PUT", "/api/v1/travel/preferences", `{"mode":"auto"}`, 200)
	// A complete itinerary without explicit UTC offsets still requires review.
	body := strings.ReplaceAll(strings.ReplaceAll(testTicketText, " UTC+3", ""), " +05:30", "")
	body = strings.Replace(body, "SU1234", "SU1235", 1)
	insertTicketMail(t, f, body, "text/plain", "INBOX")
	if err := f.s.drainTravelJobs(ctx); err != nil {
		t.Fatal(err)
	}
	items = travelList(t, f)
	found := false
	for _, i := range items {
		if i.Ticket.Number == "SU1235" {
			found = true
			if i.State != "review" || i.EventID != "" {
				t.Fatal("guessed timezone", i)
			}
		}
	}
	if !found {
		t.Fatal("no review candidate")
	}
	// Queued mail obeys a subsequent disable, and spam is not processed.
	insertTicketMail(t, f, strings.Replace(testTicketText, "SU1234", "SU1236", 1), "text/plain", "INBOX")
	f.call(0, "PUT", "/api/v1/travel/preferences", `{"mode":"off"}`, 200)
	if err := f.s.drainTravelJobs(ctx); err != nil {
		t.Fatal(err)
	}
	if len(travelList(t, f)) != 2 {
		t.Fatal("disabled queued job processed")
	}
}

// A small valid text PDF, containing no personal/customer data.
func ticketTestPDF() []byte {
	lines := strings.Split(testTicketText, "\n")
	stream := "BT /F1 12 Tf 40 750 Td 16 TL\n"
	for _, line := range lines {
		stream += "(" + line + ") Tj T*\n"
	}
	stream += "ET\n"
	objects := []string{"<< /Type /Catalog /Pages 2 0 R >>", "<< /Type /Pages /Count 1 /Kids [3 0 R] >>", "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>", "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>", fmt.Sprintf("<< /Length %d >>\nstream\n%sendstream", len(stream), stream)}
	data := "%PDF-1.4\n"
	offsets := []int{0}
	for i, obj := range objects {
		offsets = append(offsets, len(data))
		data += fmt.Sprintf("%d 0 obj\n%s\nendobj\n", i+1, obj)
	}
	xref := len(data)
	data += "xref\n0 6\n0000000000 65535 f \n"
	for _, offset := range offsets[1:] {
		data += fmt.Sprintf("%010d 00000 n \n", offset)
	}
	data += fmt.Sprintf("trailer\n<< /Root 1 0 R /Size 6 >>\nstartxref\n%d\n%%%%EOF\n", xref)
	return []byte(data)
}
func TestTravelPDFExtraction(t *testing.T) {
	if _, err := exec.LookPath("pdftotext"); err != nil {
		t.Skip("pdftotext not installed")
	}
	raw, err := attachOutgoingFiles([]byte("From: travel@example.com\r\nMIME-Version: 1.0\r\nContent-Type: text/plain\r\n\r\nYour ticket is attached."), []outgoingAttachment{{Name: "ticket.pdf", Data: base64.StdEncoding.EncodeToString(ticketTestPDF())}})
	if err != nil {
		t.Fatal(err)
	}
	trips, warnings := extractTravelTickets(context.Background(), raw)
	if len(warnings) != 0 || len(trips) != 1 || !trips[0].Automatic() || trips[0].Source != "ticket.pdf" {
		t.Fatal(trips, warnings)
	}
}
func TestTravelPDFUnavailableAndOutputLimits(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if _, err := pdfTicketText(context.Background(), ticketTestPDF()); err == nil {
		t.Fatal("missing pdftotext hidden")
	}
	if _, err := pdfTicketText(context.Background(), make([]byte, (8<<20)+1)); err == nil {
		t.Fatal("large PDF accepted")
	}
	buf := &boundedPDFText{}
	if _, err := buf.Write(make([]byte, (512<<10)+1)); err == nil {
		t.Fatal("extraction unbounded")
	}
}

func TestTravelAviasalesWorkerRoundTrip(t *testing.T) {
	f := newFileFixture(t)
	ctx := context.Background()
	text, err := os.ReadFile("../travel/testdata/aviasales-layout.txt")
	if err != nil {
		t.Fatal(err)
	}
	f.call(0, "PUT", "/api/v1/travel/preferences", `{"mode":"auto"}`, 200)
	msg := insertTicketMail(t, f, string(text), "text/plain", "INBOX")
	if err = f.s.drainTravelJobs(ctx); err != nil {
		t.Fatal(err)
	}
	items := travelList(t, f)
	if len(items) != 4 {
		t.Fatalf("got %d trips", len(items))
	}
	events := map[string]bool{}
	for _, item := range items {
		if item.State != "saved" || item.EventID == "" || events[item.EventID] {
			t.Fatal("missing or conflated personal event")
		}
		events[item.EventID] = true
		object, err := f.s.store.GetCalendarObjectByID(ctx, item.EventID)
		if err != nil {
			t.Fatal(err)
		}
		ev, err := calutil.ParseICS(object.Data)
		if err != nil || ev.Start == nil || ev.End == nil {
			t.Fatal("invalid event")
		}
		start, end, err := item.Ticket.Times()
		if err != nil || !ev.Start.Equal(start) || !ev.End.Equal(end) {
			t.Fatal("local airport time shifted")
		}
		for _, secret := range []string{"0000000001", "0000000002", "01JAN90", "01JAN85"} {
			if strings.Contains(object.Data, secret) {
				t.Fatal("document data copied to calendar")
			}
		}
	}
	f.call(0, "POST", "/api/v1/mail/messages/"+msg.ID+"/travel", "", 200)
	if len(travelList(t, f)) != 4 {
		t.Fatal("repeat analysis duplicated trips")
	}
	if count, err := f.s.store.CountOutbound(ctx); err != nil || count != 0 {
		t.Fatal("travel sent an invitation")
	}
}

// Opt-in local integration check: private receipts stay outside the repository.
// The normal fixture above is anonymized; no customer PDF is committed to Git.
func TestTravelPrivateAviasalesPDF(t *testing.T) {
	path := os.Getenv("TAYGA_TEST_TRAVEL_PDF")
	if path == "" {
		t.Skip("private receipt not supplied")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal("cannot read local receipt")
	}
	raw, err := attachOutgoingFiles([]byte("From: travel@example.com\r\nMIME-Version: 1.0\r\nContent-Type: text/plain\r\n\r\nAttached itinerary."), []outgoingAttachment{{Name: "ticket.pdf", Data: base64.StdEncoding.EncodeToString(data)}})
	if err != nil {
		t.Fatal(err)
	}
	trips, warnings := extractTravelTickets(context.Background(), raw)
	if len(warnings) != 0 || len(trips) != 4 {
		t.Fatalf("got %d trips, %d warnings", len(trips), len(warnings))
	}
	for _, trip := range trips {
		if !trip.Automatic() || trip.Source != "ticket.pdf" {
			t.Fatal("incomplete extracted itinerary")
		}
	}
	t.Log("PDF/MIME extraction: four complete segments with verified airport times")
}
