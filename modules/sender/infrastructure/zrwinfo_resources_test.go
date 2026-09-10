package infrastructure

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/modules/sender/domain"
)

func resourceAccount() *model.ProviderAccount {
	return &model.ProviderAccount{Config: `{"accesskey":"test-key","secret":"test-secret"}`}
}

func TestZrwinfoResourceProtocols(t *testing.T) {
	for _, kind := range []domain.ResourceKind{domain.ResourceTemplates, domain.ResourceSignatures} {
		for _, action := range []domain.ResourceAction{domain.ResourceCreate, domain.ResourceUpdate, domain.ResourceDelete, domain.ResourceQuery} {
			t.Run(string(kind)+"/"+string(action), func(t *testing.T) {
				noun, idField, contentField := "Sign", "signId", "sign"
				if kind == domain.ResourceTemplates {
					noun, idField, contentField = "Template", "templateId", "template"
				}
				path := "/open/api/add" + noun
				success := "ret"
				requestID := ""
				switch action {
				case domain.ResourceUpdate:
					path = "/open/api/modify" + noun
					requestID = "id"
				case domain.ResourceDelete:
					path = "/open/query/del" + noun
					success = "code"
					requestID = idField
				case domain.ResourceQuery:
					path = "/query/signlist"
					if kind == domain.ResourceTemplates {
						path = "/query/templatelist"
					}
					success = "code"
				}
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method != "POST" || r.URL.Path != path {
						t.Errorf("request: %s %s", r.Method, r.URL.Path)
					}
					if err := r.ParseForm(); err != nil {
						t.Error(err)
					}
					if r.Form.Get("accesskey") != "test-key" || r.Form.Get("secret") != "test-secret" {
						t.Error("authentication missing")
					}
					if requestID != "" && r.Form.Get(requestID) != "123" {
						t.Errorf("missing %s: %v", requestID, r.Form)
					}
					if action == domain.ResourceCreate || action == domain.ResourceUpdate {
						if r.Form.Get(contentField) != "测试{1}" {
							t.Error("wrong content field")
						}
						if kind == domain.ResourceTemplates && action == domain.ResourceUpdate && r.Form.Has("categoryId") {
							t.Error("edit sent unsupported category")
						}
					}
					if action == domain.ResourceDelete {
						fmt.Fprint(w, `{"code":0}`)
						return
					}
					data := fmt.Sprintf(`{"%s":123,"%s":"测试{1}","applyStatus":"2","applyReply":"已通过"}`, idField, contentField)
					if action == domain.ResourceQuery {
						data = "[" + data + "]"
					}
					fmt.Fprintf(w, `{"%s":"0","data":%s}`, success, data)
				}))
				defer server.Close()
				definition := newZrwinfoResourceDefinitions(server.Client(), server.URL)[kind]
				if err := definition.Validate(kind); err != nil {
					t.Fatal(err)
				}
				input := domain.ResourceInput{ID: "123", Name: "名称", Content: "测试{1}", Category: "2", Description: "用途"}
				if action == domain.ResourceQuery {
					input.ID = ""
				}
				rows, err := definition.Execute(context.Background(), resourceAccount(), action, input)
				if err != nil || len(rows) != 1 || rows[0].ID != "123" {
					t.Fatalf("result: %+v %v", rows, err)
				}
			})
		}
	}
}

func TestZrwinfoQueryDetailAndResponseValidation(t *testing.T) {
	t.Run("detail", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "GET" || r.URL.Path != "/query/getTemplate" || r.URL.Query().Get("templateId") != "9007199254740993" {
				t.Errorf("request: %s %s", r.Method, r.URL)
			}
			fmt.Fprint(w, `{"code":0,"data":{"templateId":9007199254740993,"template":"测试{1}","applyStatus":2}}`)
		}))
		defer server.Close()
		rows, err := newZrwinfoResourceDefinitions(server.Client(), server.URL)[domain.ResourceTemplates].Execute(context.Background(), resourceAccount(), domain.ResourceQuery, domain.ResourceInput{ID: "9007199254740993"})
		if err != nil || rows[0].ID != "9007199254740993" {
			t.Fatalf("lost numeric precision: %v %v", rows, err)
		}
	})
	for _, reply := range []string{`{}`, `{"code":0}`, `{"code":0,"data":null}`, `{"code":0,"data":[{"signId":1,"id":2,"sign":"品牌"}]}`, `{"code":0,"data":[{"signId":1}]}`, `{"code":9006,"msg":"test-key test-secret"}`} {
		t.Run(reply, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, reply) }))
			defer server.Close()
			_, err := newZrwinfoResourceDefinitions(server.Client(), server.URL)[domain.ResourceSignatures].Execute(context.Background(), resourceAccount(), domain.ResourceQuery, domain.ResourceInput{})
			if err == nil {
				t.Fatal("invalid response accepted")
			}
			if strings.Contains(err.Error(), "test-key") || strings.Contains(err.Error(), "test-secret") {
				t.Fatal("credential leaked")
			}
		})
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{"code":"0","data":[]}`) }))
	defer server.Close()
	rows, err := newZrwinfoResourceDefinitions(server.Client(), server.URL)[domain.ResourceSignatures].Execute(context.Background(), resourceAccount(), domain.ResourceQuery, domain.ResourceInput{})
	if err != nil || len(rows) != 0 {
		t.Fatalf("empty list: %v %v", rows, err)
	}
}

type resourceFailTransport struct{ calls int }

func (r *resourceFailTransport) RoundTrip(*http.Request) (*http.Response, error) {
	r.calls++
	return nil, errors.New("connection closed")
}
func TestZrwinfoMutationDoesNotRetryUncertainWrites(t *testing.T) {
	transport := &resourceFailTransport{}
	definition := newZrwinfoResourceDefinitions(&http.Client{Transport: transport}, "https://provider.example")[domain.ResourceSignatures]
	_, err := definition.Execute(context.Background(), resourceAccount(), domain.ResourceCreate, domain.ResourceInput{Content: "品牌"})
	var remote *domain.RemoteResourceError
	if !errors.As(err, &remote) || !remote.Uncertain || transport.calls != 1 {
		t.Fatalf("uncertain mutation: %v, calls=%d", err, transport.calls)
	}
}
