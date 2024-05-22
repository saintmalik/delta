package handlers

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	// supabase "github.com/lengzuo/supa"
	// "github.com/lengzuo/supa/dto"
	// "github.com/lengzuo/supa/dto"

	supa "github.com/nedpals/supabase-go"
)

var (
	supabaseUrl    = os.Getenv("SUPABASE_URL")
	supabaseKey    = os.Getenv("SUPABASE_KEY")
	baseUrl = os.Getenv("DOMAIN")
	supabaseClient = supa.CreateClient(supabaseUrl, supabaseKey)
)
func init() {
	// fmt.Println(os.Getenv("SUPABASE_URL"), os.Getenv("SUPABASE_KEY"), os.Getenv("GITHUB_TOKEN"))
}

func setCookie(w http.ResponseWriter, name string, value string) {
	cookie := &http.Cookie{
		Name:    name,
		Value:   value,
		Expires: time.Now().Add(24 * time.Hour),
		Secure:  true,
		Path:    "/",
	}

	http.SetCookie(w, cookie)
}

func deleteCookie(w http.ResponseWriter, name string) {
	cookie := &http.Cookie{
		Name:    name,
		Value:   "",
		Expires: time.Unix(0, 0),
		Secure:  true,
		Path:    "/",
	}

	http.SetCookie(w, cookie)
}
func HandleSignup(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	options := supa.ProviderSignInOptions{
		Provider:   "github",
		FlowType:   supa.PKCE,
		RedirectTo: baseUrl + "callback",
	}

	response, err := supabaseClient.Auth.SignInWithProvider(options)
	if err != nil {
		log.Printf("Error signing up with GitHub: %v", err)
		http.Error(w, "Error signing up with GitHub", http.StatusInternalServerError)
		return
	}
	setCookie(w, "code_verifier", response.CodeVerifier)

	fmt.Println(response.URL)

	http.Redirect(w, r, response.URL, http.StatusFound)
}

func HandleGitHubCallback(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Get the code from the query string
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "No code provided", http.StatusBadRequest)
		return
	}

	codeVerifier, err := r.Cookie("code_verifier")
	if err != nil {
		http.Error(w, "Code verifier cookie not found", http.StatusInternalServerError)
		return
	}

	ops := supa.ExchangeCodeOpts{
		CodeVerifier: codeVerifier.Value,
		AuthCode:     code,
	}

	resp, err := supabaseClient.Auth.ExchangeCode(context.Background(), ops)
	if err != nil {
		log.Printf("Error exchanging code for token: %v", err)
		http.Error(w, "Error exchanging code for token", http.StatusInternalServerError)
		return
	}
	deleteCookie(w, "code_verifier")

	setCookie(w, "access_token", resp.AccessToken)
	setCookie(w, "refresh_token", resp.RefreshToken)
	setCookie(w, "user_id", resp.User.ID)
	fmt.Println(resp)
	http.Redirect(w, r, "/dash", http.StatusFound)
}

func WelcomeHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Welcome")
}

func HandleUserLogout(w http.ResponseWriter, r *http.Request) {
	token, err := r.Cookie("access_token")
	if err != nil {
		return
	}
	err = supabaseClient.Auth.SignOut(context.Background(), token.Value)
	if err != nil {
		return
	}
	deleteCookie(w, "access_token")

	http.Redirect(w, r, "/", http.StatusFound)
}
