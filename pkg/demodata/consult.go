package demodata

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"connectrpc.com/connect"
	v1 "github.com/lao-tseu-is-alive/go-cloud-k8s-poc-2026/gen/goeland/v1"
)

// Consultations of the stories (GLD-089): a person reads before acting, so
// each story opens the pages its people would open, at story times, and the
// access log, « Consultations » and « Mon tableau de bord » show a lived-in
// history. demo-seed records every one of these reads (no 15-minute window).

// openCase is user opening the page of a case.
func (r *run) openCase(user, caseID string) {
	call(r, user, "open case", r.c.cases.GetCase, &v1.GetCaseRequest{Id: caseID})
	r.count["consultations"]++
}

// openThing is user opening the page of a thing.
func (r *run) openThing(user, thingID string) {
	call(r, user, "open thing", r.c.things.GetThing, &v1.GetThingRequest{Id: thingID})
	r.count["consultations"]++
}

// openDocument is user opening the page of each document, whose preview
// (GLD-087) downloads its content.
func (r *run) openDocument(user string, documentIDs ...string) {
	for _, id := range documentIDs {
		call(r, user, "open document", r.c.documents.GetDocument, &v1.GetDocumentRequest{Id: id})
		if r.err == nil {
			if err := r.c.download(as(r.ctx, user), id); err != nil {
				r.err = err
			}
		}
		r.count["consultations"] += 2
	}
}

// refused sends req as user and requires the server to refuse it for lack of
// rights: the story shows a refusal, which the access log records.
func refused[Req, Res any](r *run, user, what string, fn func(context.Context, *connect.Request[Req]) (*connect.Response[Res], error), req *Req) {
	if r.err != nil {
		return
	}
	_, err := fn(as(r.ctx, user), connect.NewRequest(req))
	if connect.CodeOf(err) != connect.CodePermissionDenied {
		r.err = fmt.Errorf("%s (as %s) must be refused for lack of rights, got: %w", what, user, errors.Join(err, errNotRefused))
		return
	}
	r.count["refusals"]++
}

// errNotRefused marks a story refusal the server allowed.
var errNotRefused = errors.New("not refused")

// download reads the current content of a document as the user ctx acts as,
// through the governed download, and discards it.
func (c *Clients) download(ctx context.Context, documentID string) error {
	token, err := c.session.tokenFor(ctx)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/documents/"+url.PathEscape(documentID)+"/content", nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("download %s: %w", documentID, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		return fmt.Errorf("download %s: %w", documentID, err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: HTTP %d", documentID, resp.StatusCode)
	}
	return nil
}
