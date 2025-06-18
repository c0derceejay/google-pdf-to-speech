# PROJECT: PDF-TO-SPEECH CONVERTER

## PDF to Speech Go Application
This Go application continuously monitors a specified Google Cloud Storage (GCS) bucket for new PDF files in an `pdf-input/` "folder" (prefix). When a new PDF is detected, it extracts the text, uses Google Cloud's Text-to-Speech Long Audio Synthesis API to convert the text into an audio file, and then saves the resulting audio (in LINEAR16 format) to an `mp3-output/` "folder" within the same GCS bucket.

The application is designed with modularity in mind, separating concerns into distinct Go packages.

### Project Structure
The project follows a standard Go module structure with `internal` packages for reusable, application-specific logic.

```
pdf-to-speech-app/
├── go.mod                     # Go module definition and dependencies
├── main.go                    # The main application orchestrator
└── internal/
    ├── pdf-to-text/           # Package for PDF text extraction
    │   └── pdfprocessor/
    │       └── pdf_to_text.go # Core PDF text extraction logic
    ├── storage/               # Package for Google Cloud Storage interactions
    │   └── storage.go         # GCS download, upload, and listing functions
    └── tts/                   # Package for Google Cloud Text-to-Speech interactions
        └── tts.go             # TTS Long Audio Synthesis API calls and polling
```
### How It Works: Module Breakdown
`main.go`

This is the application's entry point and its orchestrator.

- Initialization: Reads necessary configuration (GCS bucket name, Google Cloud Project Number, API location, and TTS voice name) from environment variables.

- Polling Loop: Enters an infinite loop that periodically (every `PollingInterval`, currently 10 seconds)

    - Lists objects within the `pdf-input/` prefix of the specified GCS bucket using the `internal/storage` package.
    - Filters for new PDF files that haven't been processed yet.

- Workflow Coordination: For each new PDF found:

    - Calls `internal/storage` to download the PDF to a temporary local file.
    - Calls `internal/pdf-to-text/pdfprocessor` to extract text from the downloaded PDF.
    - Calls `internal/tts` to initiate the Long Audio Synthesis, providing the extracted text and target GCS output path.
    - Marks the PDF as processed to prevent redundant processing.

- Error Handling: Logs errors at each stage, allowing for monitoring and debugging.

`internal/pdf-to-text/pdfprocessor/pdf_to_text.go`

This package is responsible for the core logic of extracting text content from PDF files.

- Dependency: It utilizes the `github.com/dslipak/pdf` library for PDF parsing.

- `ExtractTextFromFilePath` Function: This is the primary function exposed by this package. It takes a local file path to a PDF, opens it, iterates through its pages, and concatenates all found text into a single string.

    - Note: This library is best suited for text-based PDFs. For PDFs that are scanned images, an Optical Character Recognition (OCR) service (like Google Cloud Vision API) would be required, which is not currently integrated into this module.

`internal/storage/storage.go`

This package encapsulates all interactions with Google Cloud Storage.

- Global Client: Initializes a single `cloud.google.com/go/storage` client for efficiency and reuse across operations.

- `DownloadFileToTemp` Function: Downloads a specified object from a GCS bucket to a temporary file on the local filesystem. It returns the path to the temporary file and a cleanup function to ensure the temporary file is removed after use.

- `UploadFile` Function: Uploads content (as a byte slice) to a specified object path within a GCS bucket.

- `ListObjectsWithPrefix` Function: Lists objects within a GCS bucket that match a given prefix, which is used by main.go to find PDFs in pdf-input/.

`internal/tts/tts.go`

This package handles all communication with the Google Cloud Text-to-Speech API, specifically for Long Audio Synthesis.

- Global Client: Initializes a single `cloud.google.com/go/texttospeech/apiv1.TextToSpeechLongAudioSynthesizeClient`.

- `SynthesizeLongAudio` Function:

    - Constructs a `SynthesizeLongAudioRequest` using the extracted text, project number, location, desired output GCS URI, and the specified voice name.

    - Important: Configures `AudioEncoding: texttospeechpb.AudioEncoding_LINEAR16` with a `SampleRateHertz: 16000`, as the Long Audio Synthesis API currently only supports LINEAR16 output directly to GCS.

    - Initiates an asynchronous long-running operation with the TTS API.

    - Polling: Implements a polling mechanism that repeatedly checks the status of the long-running operation until it completes (either successfully or with an error). This ensures the application waits for the audio synthesis to finish before moving on.

    - Logs the progress and final status of the synthesis operation.

### Deployment & Running
The application is designed to be run as a standalone Go executable, typically on a Google Compute Engine (GCE) VM instance.

#### Prerequisites:
- A Google Cloud Project with Cloud Storage API and Cloud Text-to-Speech API enabled.

- A single GCS bucket (e.g., `pdf-audio-bucket`) to hold both input PDFs (`pdf-input/`) and output audio (`mp3-output/`).

- A Service Account (e.g., `pdf-to-speech-app-sa`) with the following IAM roles:

    - `roles/storage.objectViewer` on `gs://pdf-audio-bucket`

    - `roles/storage.objectCreator` on `gs://pdf-audio-bucket`

    - `roles/cloudtexttospeech.user` on `YOUR_PROJECT_ID`

    - `roles/serviceusage.serviceUsageConsumer` on `YOUR_PROJECT_ID`

- Go (version 1.21 or higher) installed locally or on your deployment environment.

- Google Cloud CLI installed and authenticated (for local testing credentials).

### Local Setup & Execution:
1. Clone/Create Project Structure:
```
git clone your_repo_url # If hosted
# Or manually create:
mkdir pdf-to-speech-app
cd pdf-to-speech-app
go mod init jsou/tts # Your module name
mkdir -p internal/pdf-to-text/pdfprocessor internal/storage internal/tts
```

2. Place Code: Copy the respective code into `main.go`, `internal/pdf-to-text/pdfprocessor/pdf_to_text.go`, `internal/storage/storage.go`, and `internal/tts/tts.go`.

3. Update `go.mod`: Ensure your `go.mod` matches the provided "Updated go.mod for Refactored Application" content.

4. Download Dependencies:
```
go mod tidy
```

5. Local Authentication:
```
gcloud auth application-default login
```

6. Set Environment Variables:
```
export BASE_GCS_BUCKET="BUCKET_NAME"
export PROJECT_NUMBER="YOUR_ACTUAL_PROJECT_NUMBER" # Find in GCP Console
export GCP_LOCATION="YOUR_REGION"   # Or your chosen region (e.g., global)
export TTS_VOICE_NAME="en-US-Wavenet-D" # Or another voice from TTS docs
```
7. Run Application:
```
go run .
```

### Usage
1. Drop PDF: Upload a PDF file to `gs://pdf-audio-bucket/pdf-input/` using the GCS Console or `gsutil`.

2. Monitor Output: The application will process the PDF, and the resulting LINEAR16 audio file will appear in `gs://pdf-audio-bucket/mp3-output/` with the same base filename.
