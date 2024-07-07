package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

const (
	maxFileSizeBytes = 50 * 1024 * 1024 // 50 MB
	uploadPath       = "./tmp"
	tokenFile        = "token.json"
)

var (
	config    *oauth2.Config
	authState = &struct {
		authorized bool
		mu         sync.Mutex
	}{authorized: false}
)

func init() {
	b, err := os.ReadFile("credentials.json")
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}

	config, err = google.ConfigFromJSON(b, drive.DriveFileScope)
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}

	// Set the correct redirect URL
	config.RedirectURL = "http://localhost:8081/oauth2callback"

	// Print the redirect URL for debugging
	fmt.Printf("Redirect URL set to: %s\n", config.RedirectURL)

	// Check if token file exists
	if _, err := os.Stat(tokenFile); err == nil {
		authState.authorized = true
	}
}

func main() {
	http.HandleFunc("/", serveHTML)
	http.HandleFunc("/get-auth-url", getAuthURL)
	http.HandleFunc("/check-auth-status", checkAuthStatus)
	http.HandleFunc("/oauth2callback", handleOAuth2Callback)
	http.HandleFunc("/upload", uploadHandler)

	fmt.Println("Server is running on http://localhost:8081")
	log.Fatal(http.ListenAndServe("0.0.0.0:8081", nil))
}

func serveHTML(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "index.html")
}

func getAuthURL(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Generated Auth URL: %s\n", authURL) // Print the generated URL for debugging
	json.NewEncoder(w).Encode(map[string]string{"url": authURL})
}

func checkAuthStatus(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	authState.mu.Lock()
	defer authState.mu.Unlock()
	json.NewEncoder(w).Encode(map[string]bool{"authorized": authState.authorized})
}

func handleOAuth2Callback(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("Received callback request: %s\n", r.URL.String()) // Print the received callback URL for debugging
	state := r.URL.Query().Get("state")
	if state != "state-token" {
		http.Error(w, "Invalid state parameter", http.StatusBadRequest)
		return
	}

	code := r.URL.Query().Get("code")
	tok, err := config.Exchange(context.Background(), code)
	if err != nil {
		http.Error(w, "Unable to retrieve token from web", http.StatusInternalServerError)
		return
	}

	saveToken(tokenFile, tok)
	authState.mu.Lock()
	authState.authorized = true
	authState.mu.Unlock()

	fmt.Fprintf(w, "Authorization successful! You can close this window and return to the application.")
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	authState.mu.Lock()
	authorized := authState.authorized
	authState.mu.Unlock()

	if !authorized {
		http.Error(w, "Not authorized. Please authorize the application first.", http.StatusUnauthorized)
		return
	}

	r.ParseMultipartForm(10 << 20)
	file, handler, err := r.FormFile("uploadfile")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	err = os.MkdirAll(uploadPath, os.ModePerm)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	tempFile, err := os.CreateTemp(uploadPath, "upload-*.tmp")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer tempFile.Close()
	defer os.Remove(tempFile.Name()) // Clean up the temp file after we're done

	fileBytes, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if len(fileBytes) > maxFileSizeBytes {
		http.Error(w, "File too large", http.StatusBadRequest)
		return
	}

	tempFile.Write(fileBytes)

	err = uploadFileToDrive(tempFile.Name())
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to upload file to Google Drive: %v", err), http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, "Successfully Uploaded File: %s", handler.Filename)
}

func uploadFileToDrive(filePath string) error {
	ctx := context.Background()
	client, err := getClient(config)
	if err != nil {
		return fmt.Errorf("unable to get client: %v", err)
	}

	srv, err := drive.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return fmt.Errorf("unable to retrieve Drive client: %v", err)
	}

	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("unable to open file: %v", err)
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("unable to get file info: %v", err)
	}

	fileMetadata := &drive.File{
		Name: filepath.Base(filePath),
	}
	res, err := srv.Files.Create(fileMetadata).Media(file).Do()
	if err != nil {
		return fmt.Errorf("unable to create file: %v", err)
	}

	fmt.Printf("File '%s' uploaded successfully. File ID: %s\n", fileInfo.Name(), res.Id)
	return nil
}

func getClient(config *oauth2.Config) (*http.Client, error) {
	tok, err := tokenFromFile(tokenFile)
	if err != nil {
		return nil, fmt.Errorf("token not found or invalid: %v", err)
	}
	return config.Client(context.Background(), tok), nil
}

func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

func saveToken(path string, token *oauth2.Token) {
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Unable to cache oauth token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}

func enableCors(w *http.ResponseWriter) {
	(*w).Header().Set("Access-Control-Allow-Origin", "*")
	(*w).Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
	(*w).Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
}
