package accounts

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type GitHubUser struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

func (c Config) ExchangeGitHubCode(ctx context.Context, code, redirectURI string) (string, error) {
	form := url.Values{
		"code":          {code},
		"client_id":     {c.GitHubClientID},
		"client_secret": {c.GitHubClientSecret},
		"redirect_uri":  {redirectURI},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://github.com/login/oauth/access_token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("github token endpoint returned %d", resp.StatusCode)
	}

	var body struct {
		AccessToken      string `json:"access_token"`
		Error            string `json:"error"`
		ErrorDescription string `json:"error_description"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", err
	}
	if body.Error != "" {
		return "", fmt.Errorf("github oauth error: %s - %s", body.Error, body.ErrorDescription)
	}
	if body.AccessToken == "" {
		return "", fmt.Errorf("github oauth: no access token in response")
	}
	return body.AccessToken, nil
}

// GitHubUserInfo fetches the profile, then falls back to the emails
// endpoint if the profile's email is private (the common case - GitHub
// only returns a public profile email if the user opted in to showing one).
func (c Config) GitHubUserInfo(ctx context.Context, accessToken string) (*GitHubUser, error) {
	user, err := getGitHubJSON[GitHubUser](ctx, accessToken, "https://api.github.com/user")
	if err != nil {
		return nil, err
	}

	if user.Email == "" {
		type githubEmail struct {
			Email    string `json:"email"`
			Primary  bool   `json:"primary"`
			Verified bool   `json:"verified"`
		}
		emails, err := getGitHubJSON[[]githubEmail](ctx, accessToken, "https://api.github.com/user/emails")
		if err == nil {
			for _, e := range *emails {
				if e.Primary && e.Verified {
					user.Email = e.Email
					break
				}
			}
			if user.Email == "" {
				for _, e := range *emails {
					if e.Verified {
						user.Email = e.Email
						break
					}
				}
			}
		}
	}

	return user, nil
}

func getGitHubJSON[T any](ctx context.Context, accessToken, url string) (*T, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("User-Agent", "k8-platform")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github request to %s returned %d", url, resp.StatusCode)
	}

	var out T
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}
