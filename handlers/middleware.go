package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/saintmalik/delta/model"
)

type userContextKeyType struct{}

var userContextKey userContextKeyType

type authError struct {
	msg string
}

func (e *authError) Error() string {
	return e.msg
}

func IsAuthenticated(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, err := IsAuthenticatedUser(w, r)
		if err != nil {
			if err, ok := err.(*authError); ok {
				// Handle specific authentication error (e.g., not_authenticated)
				http.Redirect(w, r, fmt.Sprintf("/?error=%s", err.msg), http.StatusFound)
				return
			}
			fmt.Println(err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		ctx := context.WithValue(r.Context(), userContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
func IsAuthenticatedUser(w http.ResponseWriter, r *http.Request) (*model.User, error) {
    accessToken, err := r.Cookie("access_token")
    if err != nil {
        if errors.Is(err, http.ErrNoCookie) {
            return nil, errors.New("missing access token")
        }
        return nil, fmt.Errorf("failed to retrieve access token: %w", err)
    }

    if isEmptyString(accessToken.Value) {
        return nil, errors.New("empty access token")
    }

    ctx := context.Background()
    supabaseUser, err := supabaseClient.Auth.User(ctx, accessToken.Value)
    if err != nil {
        if isLikelyExpiredTokenError(err) {
            newAccessToken, err := refreshAccessToken(w, r)
            if err != nil {
                return nil, fmt.Errorf("failed to refresh access token: %w", err)
            }

            supabaseUser, err = supabaseClient.Auth.User(ctx, newAccessToken)
            if err != nil {
				http.Redirect(w, r, "/", http.StatusFound)
                return nil, fmt.Errorf("failed to retrieve user information: %w", err)
            }
        } else {
			http.Redirect(w, r, "/", http.StatusFound)
            return nil, fmt.Errorf("failed to retrieve user information: %w", err)
        }
    }

    modelUser := &model.User{
        ID:    supabaseUser.ID,
        Email: supabaseUser.Email,
    }
    return modelUser, nil
}
func isLikelyExpiredTokenError(err error) bool {
	return strings.Contains(err.Error(), "invalid token") || strings.Contains(err.Error(), "unauthorized")
}

func refreshAccessToken(w http.ResponseWriter, r *http.Request) (string, error) {
    refreshToken, err := r.Cookie("refresh_token")
    if err != nil {
        if errors.Is(err, http.ErrNoCookie) {
            return "", errors.New("missing refresh token")
        }
        return "", fmt.Errorf("failed to retrieve refresh token: %w", err)
    }

    resp, err := supabaseClient.Auth.RefreshUser(context.Background(), "", refreshToken.Value)
    if err != nil {
        return "", fmt.Errorf("failed to refresh access token: %w", err)
    }

    setCookie(w, "access_token", resp.AccessToken)
    return resp.AccessToken, nil
}
func isEmptyString(s string) bool {
	return len(strings.TrimSpace(s)) == 0
}
