package pdftospeech

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"MODULE_NAME/jsou-tts/internal/pdf-to-text/pdfprocessor"
	"MODULE_NAME/jsou-tts/internal/storage"
	"MODULE_NAME/jsou-tts/internal/tts"
	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	v2 "github.com/cloudevents/sdk-go/v2"
)

// StorageObjectData is the payload of a GCS event.
type StorageObjectData struct {
	Bucket      string `json:"bucket"`
	Name        string `json:"name"`
	ContentType string `json:"contentType"`
}

// internal/storage has its own client now, so no global Storage Client is needed.

func init() {
	// Register the Cloud Function entry point directly to the handler that expects StorageObjectData.
	functions.CloudEvent("ProcessPDFToSpeechTest", func(ctx context.Context, e v2.Event) error {
		var eventData StorageObjectData
		if err := e.DataAs(&eventData); err != nil {
			return fmt.Errorf("failed to parse event data: %w", err)
		}
		return processPDFToSpeechHandler(ctx, eventData)
	})
}

// processPDFToSpeechHandler is the Cloud Function's event handler.
// It's triggered by Cloud Storage object finalization events, with the payload
// directly unmarshaled into the StorageObjectData struct by the functions-framework.
func processPDFToSpeechHandler(ctx context.Context, e StorageObjectData) error {
	log.Printf("Received event for file: %s in bucket: %s with content type: %s", e.Name, e.Bucket, e.ContentType)

	// Ensure the file is a PDF and from the correct input prefix
	if !strings.HasSuffix(strings.ToLower(e.Name), ".pdf") {
		log.Printf("Skipping non-PDF file: %s. Content type: %s", e.Name, e.ContentType)
		return nil // Not an error, just skipping
	}
	if !strings.HasPrefix(e.Name, "pdf-input/") {
		log.Printf("Skipping PDF file not in 'pdf-input/' folder: %s", e.Name)
		return nil
	}

	// Define folder prefixes
	const inputFolderPrefix = "pdf-input/"
	const outputFolderPrefix = "mp3-output/"

	// Extract the base file name (e.g., "document.pdf" from "pdf-input/document.pdf").
	baseFileName := filepath.Base(e.Name)
	// Construct the full output object name with the output folder prefix and .mp3 extension.
	outputAudioObjectName := outputFolderPrefix + strings.TrimSuffix(baseFileName, filepath.Ext(baseFileName)) + ".mp3"
	outputGCSURI := fmt.Sprintf("gs://%s/%s", e.Bucket, outputAudioObjectName)

	// Get Project Number and Location from environment variables.
	projectNumber := os.Getenv("PROJECT_NUMBER")
	location := os.Getenv("GCP_LOCATION")

	if projectNumber == "" || location == "" {
		return fmt.Errorf("environment variables PROJECT_NUMBER and GCP_LOCATION must be set in the Cloud Function configuration")
	}

	// Get TTS Voice Name from environment variable.
	ttsVoiceName := os.Getenv("TTS_VOICE_NAME")
	if ttsVoiceName == "" {
		log.Printf("TTS_VOICE_NAME environment variable not set. Using default 'en-US-Wavenet-D'.")
		ttsVoiceName = "en-US-Wavenet-D" // A common, generally available Wavenet voice
	}

	log.Printf("Processing PDF: %s in bucket: %s", e.Name, e.Bucket)
	log.Printf("Target output: %s", outputGCSURI)
	log.Printf("Using Project Number: %s, Location: %s, Voice: %s", projectNumber, location, ttsVoiceName)

	// 1. Download the PDF file from the input bucket to a temporary path.
	// The call to storage.DownloadFileToTemp is correct here.
	tempPDFPath, cleanupTempFile, err := storage.DownloadFileToTemp(ctx, e.Bucket, e.Name)
	if err != nil {
		return fmt.Errorf("failed to download PDF %s: %w", e.Name, err)
	}
	defer cleanupTempFile() // Ensure temp file is cleaned up after processing

	// 2. Extract text from the temporary PDF file.
	extractedText, err := pdfprocessor.ExtractTextFromPDFFilePath(tempPDFPath)
	if err != nil {
		return fmt.Errorf("failed to extract text from PDF %s: %w", e.Name, err)
	}

	if strings.TrimSpace(extractedText) == "" {
		log.Printf("No text extracted from PDF: %s. Skipping TTS.", e.Name)
		return nil
	}
	log.Printf("Text extracted from PDF. Length: %d characters.", len(extractedText))

	// 3. Synthesize long audio using the TTS API, directly to GCS.
	err = tts.SynthesizeLongAudio(ctx, extractedText, projectNumber, location, outputGCSURI, ttsVoiceName)
	if err != nil {
		return fmt.Errorf("failed to synthesize speech for %s: %w", e.Name, err)
	}

	log.Printf("Successfully processed %s. Output: %s", e.Name, outputGCSURI)
	return nil
}
