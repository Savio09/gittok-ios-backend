package githubclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMetadataPresenceAndPublicVisibility(t *testing.T) {
	for _, tc := range []struct {
		body            string
		public          bool
		hasStars        bool
		nullDescription bool
	}{
		{`{"private":false,"stargazers_count":0,"description":null}`, true, true, true},
		{`{"description":""}`, false, false, false},
		{`{"private":true,"stargazers_count":12}`, false, true, false},
		{`{"private":null}`, false, false, false},
		{`{"private":false,"visibility":"internal"}`, false, false, false},
	} {
		t.Run(tc.body, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(tc.body)) }))
			defer srv.Close()
			r, err := New("", WithBaseURL(srv.URL)).GetRepo(context.Background(), "o", "r")
			if err != nil {
				t.Fatal(err)
			}
			if r.Public != tc.public {
				t.Fatalf("public=%v", r.Public)
			}
			_, has := r.Metadata["stargazers_count"]
			if has != tc.hasStars {
				t.Fatal("lost field presence")
			}
			if r.Metadata["description"].Null != tc.nullDescription {
				t.Fatal("lost explicit null")
			}
			if tc.hasStars && r.Stars == 0 && r.Metadata["stargazers_count"].Value != "0" {
				t.Fatal("lost reported zero")
			}
		})
	}
}
