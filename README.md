# CDMFileUpload

### Backend Setup
1. Clone the repository
2. Navigate to the project directory
3. Create a `.env` file in the root directory and add your Google credentials:
   ```
   GOOGLE_APPLICATION_CREDENTIALS='{"type": "service_account", ...}'
   PORT=8081
   ```
4. Run the backend:
   ```
   go run backend.go
   ```

### Frontend Setup
1. Navigate to the `file-upload-app` directory
2. Install dependencies:
   ```
   npm install
   ```
3. Start the development server:
   ```
   npm start
   ```

## Usage
1. Ensure both the backend (Go) and frontend (React) servers are running.
2. Open http://localhost:3000 in your browser to access the application.
3. Select a file using the file input.
4. Click the "Upload" button to upload the file to Google Drive.
