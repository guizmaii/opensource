package jetcloud

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"

	"go.jetpack.io/envsec/internal/auth"
	"go.jetpack.io/envsec/internal/envvar"
	typeid "go.jetpack.io/typeid/typed"
	"golang.org/x/oauth2"
)

type projectPrefix struct{}

func (projectPrefix) Type() string { return "proj" }

type projectID struct{ typeid.TypeID[projectPrefix] }

var nilProjectID = projectID{typeid.Nil[projectPrefix]()}

type project struct {
	ID projectID `json:"id"`
}

type client struct {
	apiHost string
}

func newClient() *client {
	return &client{
		apiHost: envvar.Get(
			"ENVSEC_API_HOST",
			"https://envsec-server-web-mike-jetpack-io.cloud.jetpack.dev/",
		),
	}
}

func (c *client) endpoint(path string) string {
	endpointURL, err := url.JoinPath(c.apiHost, path)
	if err != nil {
		panic(err)
	}
	return endpointURL
}

func (c *client) newProjectID(user *auth.User, repo, subdir string) (projectID, error) {
	fmt.Fprintf(os.Stderr, "Creating new project for repo=%s subdir=%s\n", repo, subdir)

	p, err := post[project](c, user, map[string]string{
		// TODO: org_id should be a claim in the ID token, not passed as post data
		"org_id":   user.OrgID(),
		"repo_url": repo,
		"subdir":   subdir,
	})
	if err != nil {
		return nilProjectID, err
	}

	return p.ID, nil
}

func post[T any](c *client, user *auth.User, data any) (*T, error) {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}

	src := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: user.IDToken.Raw},
	)
	httpClient := oauth2.NewClient(context.Background(), src)

	req, err := http.NewRequest(
		http.MethodPost,
		c.endpoint("projects"),
		bytes.NewBuffer(dataBytes),
	)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	log.Println(string(body))

	var result T
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	return &result, nil
}